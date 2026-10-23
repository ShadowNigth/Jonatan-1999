package dbus

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BusName es el nombre del autobús de sesión en las llamadas al propio bus.
const (
	BusName     = "org.freedesktop.DBus"
	BusPath     = "/org/freedesktop/DBus"
	BusIface    = "org.freedesktop.DBus"
	FlagNoAuto  = 0x4 // no encolar si el nombre está ocupado
	FlagReplace = 0x2
)

// Conn es una conexión al autobús de D-Bus.
type Conn struct {
	nc net.Conn
	br *bufio.Reader

	writeMu sync.Mutex

	mu      sync.Mutex
	serial  uint32
	pending map[uint32]chan *Message
	closed  bool

	handlerMu sync.RWMutex
	onSignal  func(*Message)
	onCall    func(*Message)

	// Unique es el nombre único asignado por el bus (":1.42").
	Unique string
	// GUID es el identificador del bus, recibido en el handshake.
	GUID string
}

// DialSession se conecta al autobús de sesión y hace el handshake.
func DialSession() (*Conn, error) {
	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if addr == "" {
		return nil, fmt.Errorf("DBUS_SESSION_BUS_ADDRESS no está definido (¿hay una sesión D-Bus?)")
	}
	return Dial(addr)
}

// SessionAvailable indica si parece haber un autobús de sesión accesible.
func SessionAvailable() bool {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return false
	}
	c, err := DialSession()
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// Dial se conecta a la dirección de autobús indicada y hace el handshake SASL.
func Dial(address string) (*Conn, error) {
	network, target, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	nc, err := net.DialTimeout(network, target, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con D-Bus (%s): %w", target, err)
	}
	c := &Conn{
		nc:      nc,
		br:      bufio.NewReaderSize(nc, 64*1024),
		pending: map[uint32]chan *Message{},
	}
	if err := c.handshake(); err != nil {
		nc.Close()
		return nil, err
	}
	go c.readLoop()
	return c, nil
}

// WrapConn envuelve un socket ya conectado y autenticado (por ejemplo, el lado
// servidor de un socketpair en pruebas, o una conexión heredada por
// activación de socket) y empieza a leer mensajes.
func WrapConn(nc net.Conn) *Conn {
	c := &Conn{
		nc:      nc,
		br:      bufio.NewReaderSize(nc, 64*1024),
		pending: map[uint32]chan *Message{},
	}
	go c.readLoop()
	return c
}

// parseAddress interpreta "unix:path=...,guid=..." y sus variantes.
func parseAddress(address string) (network, target string, err error) {
	for _, part := range strings.Split(address, ";") {
		if part == "" {
			continue
		}
		kind := part
		params := ""
		if i := strings.IndexByte(part, ':'); i >= 0 {
			kind, params = part[:i], part[i+1:]
		}
		vals := map[string]string{}
		for _, kv := range strings.Split(params, ",") {
			if i := strings.IndexByte(kv, '='); i > 0 {
				vals[kv[:i]] = kv[i+1:]
			}
		}
		switch kind {
		case "unix":
			if p := vals["path"]; p != "" {
				return "unix", p, nil
			}
			if a := vals["abstract"]; a != "" {
				return "unix", "@" + a, nil
			}
		case "tcp":
			if h := vals["host"]; h != "" {
				return "tcp", net.JoinHostPort(h, vals["port"]), nil
			}
		}
	}
	return "", "", fmt.Errorf("dirección de D-Bus no soportada: %q", address)
}

// handshake hace la autenticación EXTERNAL (uid) y pasa a modo binario.
func (c *Conn) handshake() error {
	if _, err := c.nc.Write([]byte{0}); err != nil {
		return err
	}
	uid := strconv.Itoa(os.Getuid())
	auth := "AUTH EXTERNAL " + hex.EncodeToString([]byte(uid)) + "\r\n"
	if _, err := c.nc.Write([]byte(auth)); err != nil {
		return err
	}
	line, err := c.readLine()
	if err != nil {
		return err
	}
	switch {
	case strings.HasPrefix(line, "OK "):
		c.GUID = strings.TrimSpace(strings.TrimPrefix(line, "OK "))
	case strings.HasPrefix(line, "REJECTED"):
		// Algunos buses permiten ANONYMOUS si EXTERNAL está desactivado.
		if _, err := c.nc.Write([]byte("AUTH ANONYMOUS\r\n")); err != nil {
			return err
		}
		line, err = c.readLine()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, "OK ") {
			return fmt.Errorf("D-Bus rechazó la autenticación: %s", line)
		}
		c.GUID = strings.TrimSpace(strings.TrimPrefix(line, "OK "))
	default:
		return fmt.Errorf("respuesta de autenticación inesperada: %s", line)
	}
	if _, err := c.nc.Write([]byte("BEGIN\r\n")); err != nil {
		return err
	}
	return nil
}

func (c *Conn) readLine() (string, error) {
	_ = c.nc.SetReadDeadline(time.Now().Add(10 * time.Second))
	line, err := c.br.ReadString('\n')
	_ = c.nc.SetReadDeadline(time.Time{})
	if err != nil {
		return "", fmt.Errorf("error leyendo del autobús D-Bus: %w", err)
	}
	return line, nil
}

// SetSignalHandler registra la función que recibe las señales.
func (c *Conn) SetSignalHandler(fn func(*Message)) {
	c.handlerMu.Lock()
	c.onSignal = fn
	c.handlerMu.Unlock()
}

// SetMethodHandler registra la función que atiende las llamadas a método.
// Sin él, cualquier llamada recibe un error UnknownMethod.
func (c *Conn) SetMethodHandler(fn func(*Message)) {
	c.handlerMu.Lock()
	c.onCall = fn
	c.handlerMu.Unlock()
}

// readLoop lee mensajes del socket y los reparte.
func (c *Conn) readLoop() {
	header := make([]byte, 16)
	for {
		if _, err := readFull(c.br, header); err != nil {
			c.failAll(err)
			return
		}
		bodyLen := int(binary.LittleEndian.Uint32(header[4:]))
		fieldsLen := int(binary.LittleEndian.Uint32(header[12:]))
		// Cabecera (rellenada a 8), cuerpo y relleno final a 8.
		bodyStart := align8(16 + fieldsLen)
		total := align8(bodyStart + bodyLen)
		if total < 16 || total > 128<<20 {
			c.failAll(fmt.Errorf("mensaje D-Bus de tamaño imposible: %d", total))
			return
		}
		msg := make([]byte, total)
		copy(msg, header)
		if _, err := readFull(c.br, msg[16:]); err != nil {
			c.failAll(err)
			return
		}
		decoded, err := DecodeMessage(msg)
		if err != nil {
			continue
		}
		c.dispatch(decoded)
	}
}

func align8(n int) int { return (n + 7) &^ 7 }

func readFull(br *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := br.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (c *Conn) dispatch(m *Message) {
	switch m.Type {
	case MethodReturn, ErrorReply:
		c.mu.Lock()
		ch := c.pending[m.ReplySerial]
		delete(c.pending, m.ReplySerial)
		c.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	case Signal:
		c.handlerMu.RLock()
		fn := c.onSignal
		c.handlerMu.RUnlock()
		if fn != nil {
			fn(m)
		}
	case MethodCall:
		c.handlerMu.RLock()
		fn := c.onCall
		c.handlerMu.RUnlock()
		if fn == nil {
			_ = c.ReplyError(m, "org.freedesktop.DBus.Error.UnknownMethod", "método desconocido")
			return
		}
		go fn(m)
	}
}

func (c *Conn) failAll(err error) {
	c.mu.Lock()
	c.closed = true
	for serial, ch := range c.pending {
		delete(c.pending, serial)
		ch <- &Message{Type: ErrorReply, ErrorName: "org.freedesktop.DBus.Error.Disconnected", Body: []any{err.Error()}}
	}
	c.mu.Unlock()
}

// nextSerial devuelve un serial nuevo.
func (c *Conn) nextSerial() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serial++
	return c.serial
}

// send serializa y escribe un mensaje con un serial nuevo.
func (c *Conn) send(m *Message) (uint32, error) {
	return c.sendSerial(m, c.nextSerial())
}

// sendSerial escribe el mensaje con el serial ya reservado.
func (c *Conn) sendSerial(m *Message, serial uint32) (uint32, error) {
	m.Serial = serial
	data, err := m.Encode(serial)
	if err != nil {
		return 0, err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.nc.Write(data); err != nil {
		return 0, fmt.Errorf("error escribiendo en D-Bus: %w", err)
	}
	return serial, nil
}

// Call envía una llamada a método y espera la respuesta.
func (c *Conn) Call(ctx context.Context, dest, path, iface, member, sig string, args ...any) (*Message, error) {
	m := NewCall(dest, path, iface, member, sig, args...)
	// La espera se registra ANTES de enviar: en un autobús local la respuesta
	// puede llegar antes de que termine la escritura.
	ch := make(chan *Message, 1)
	serial := c.nextSerial()
	c.mu.Lock()
	c.pending[serial] = ch
	c.mu.Unlock()
	if _, err := c.sendSerial(m, serial); err != nil {
		c.mu.Lock()
		delete(c.pending, serial)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case reply := <-ch:
		if reply.Type == ErrorReply {
			text := ""
			if len(reply.Body) > 0 {
				text, _ = reply.Body[0].(string)
			}
			return reply, fmt.Errorf("%s: %s", reply.ErrorName, text)
		}
		return reply, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, serial)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Emit envía una señal sin esperar respuesta.
func (c *Conn) Emit(path, iface, member, sig string, args ...any) error {
	_, err := c.send(NewSignal(path, iface, member, sig, args...))
	return err
}

// EmitTo manda una señal con destino concreto (unicast). Si dest está vacío se
// comporta como Emit, difundiéndola por el bus.
func (c *Conn) EmitTo(dest, path, iface, member, sig string, args ...any) error {
	m := NewSignal(path, iface, member, sig, args...)
	m.Destination = dest
	_, err := c.send(m)
	return err
}

// Reply responde a una llamada con un valor de retorno.
func (c *Conn) Reply(call *Message, sig string, args ...any) error {
	m := &Message{
		Type: MethodReturn, Destination: call.Sender, ReplySerial: call.Serial,
		Signature: sig, Body: args,
	}
	_, err := c.send(m)
	return err
}

// ReplyError responde a una llamada con un error.
func (c *Conn) ReplyError(call *Message, name, text string) error {
	m := &Message{
		Type: ErrorReply, Destination: call.Sender, ReplySerial: call.Serial,
		ErrorName: name, Signature: "s", Body: []any{text},
	}
	_, err := c.send(m)
	return err
}

// Hello registra la conexión y averigua el nombre único.
func (c *Conn) Hello() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, err := c.Call(ctx, BusName, BusPath, BusIface, "Hello", "")
	if err != nil {
		return err
	}
	if len(reply.Body) > 0 {
		c.Unique, _ = reply.Body[0].(string)
	}
	return nil
}

// RequestName pide un nombre bien conocido; devuelve el código del bus
// (1 = propietario, 2 = en cola, 3 = ya existe, 4 = ya era propietario).
func (c *Conn) RequestName(name string, flags uint32) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, err := c.Call(ctx, BusName, BusPath, BusIface, "RequestName", "su", name, flags)
	if err != nil {
		return 0, err
	}
	if len(reply.Body) > 0 {
		if n, ok := toUint(reply.Body[0]); ok {
			return uint32(n), nil
		}
	}
	return 0, nil
}

// ReleaseName libera un nombre bien conocido.
func (c *Conn) ReleaseName(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := c.Call(ctx, BusName, BusPath, BusIface, "ReleaseName", "s", name)
	return err
}

// Close cierra la conexión.
func (c *Conn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	return c.nc.Close()
}
