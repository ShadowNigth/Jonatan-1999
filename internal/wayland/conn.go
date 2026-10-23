package wayland

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Conn es una conexión al compositor Wayland.
type Conn struct {
	mu      sync.Mutex
	nc      *net.UnixConn
	nextID  uint32
	objects map[uint32]*object
	// globals anunciados por el registro.
	globals []Global
	// cola de eventos pendientes de procesar.
	pending [][]byte
	// readBuf acumula bytes a la espera de mensajes completos.
	readBuf []byte
	// pendingFds son descriptores recibidos pendientes de asignar.
	pendingFds []int
	// registryID es el objeto wl_registry.
	registryID uint32
	closed     bool
}

// object representa un objeto del protocolo con su escucha de eventos.
type object struct {
	id      uint32
	iface   string
	handler func(opcode uint16, d *decoder)
}

// Global es un objeto global anunciado por el compositor.
type Global struct {
	Name      uint32
	Interface string
	Version   uint32
}

// NewConn abre el socket del compositor indicado por WAYLAND_DISPLAY.
func NewConn() (*Conn, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return nil, fmt.Errorf("XDG_RUNTIME_DIR no está definido")
	}
	name := os.Getenv("WAYLAND_DISPLAY")
	if name == "" {
		name = "wayland-0"
	}
	path := name
	if !filepath.IsAbs(name) {
		path = filepath.Join(runtimeDir, name)
	}
	nc, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("conectando al compositor Wayland (%s): %w", path, err)
	}
	c := &Conn{
		nc:      nc,
		nextID:  2, // el 1 está reservado para wl_display
		objects: map[uint32]*object{},
		readBuf: make([]byte, 0, 64*1024),
	}
	c.objects[1] = &object{id: 1, iface: "wl_display"}
	return c, nil
}

// allocID reserva un identificador nuevo para un objeto del cliente.
func (c *Conn) allocID(iface string, handler func(uint16, *decoder)) uint32 {
	id := c.nextID
	c.nextID++
	c.objects[id] = &object{id: id, iface: iface, handler: handler}
	return id
}

// nextIDFor pide al compositor un identificador para un constructor de objeto
// creado por el servidor (no se usa en este cliente).
func (c *Conn) setHandler(id uint32, iface string, handler func(uint16, *decoder)) {
	if o, ok := c.objects[id]; ok {
		o.iface = iface
		o.handler = handler
		return
	}
	c.objects[id] = &object{id: id, iface: iface, handler: handler}
}

// Send escribe un mensaje sin descriptor.
func (c *Conn) Send(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.nc.Write(msg)
	return err
}

// SendWithFD escribe un mensaje enviando un descriptor por SCM_RIGHTS.
func (c *Conn) SendWithFD(msg []byte, fd int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	buf := msg
	oob := syscall.UnixRights(fd)
	for {
		n, oobn, err := c.nc.WriteMsgUnix(buf, oob, nil)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return err
		}
		if n < len(buf) {
			buf = buf[n:]
			continue
		}
		_ = oobn
		return nil
	}
}

// Dispatch lee y procesa los mensajes disponibles, esperando como máximo timeout.
//
// Devuelve el número de mensajes procesados.
func (c *Conn) Dispatch(timeout time.Duration) (int, error) {
	if timeout > 0 {
		_ = c.nc.SetReadDeadline(time.Now().Add(timeout))
	} else {
		_ = c.nc.SetReadDeadline(time.Time{})
	}
	// Primero se procesa lo que quedó pendiente de la lectura anterior.
	processed := c.drain()

	buf := make([]byte, 64*1024)
	oob := make([]byte, syscall.CmsgSpace(4*8))
	n, oobn, _, _, err := c.nc.ReadMsgUnix(buf, oob)
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return processed, nil
		}
		if err == syscall.EINTR {
			return processed, nil
		}
		return processed, err
	}
	if n == 0 {
		return processed, fmt.Errorf("el compositor cerró la conexión")
	}
	fds := parseFDs(oob[:oobn])
	c.readBuf = append(c.readBuf, buf[:n]...)
	c.pendingFds = append(c.pendingFds, fds...)
	processed += c.drain()
	return processed, nil
}

// drain procesa todos los mensajes completos del búfer de lectura.
func (c *Conn) drain() int {
	processed := 0
	for len(c.readBuf) >= 8 {
		size := int(uint16(c.readBuf[6]) | uint16(c.readBuf[7])<<8)
		if size < 8 || size > len(c.readBuf) {
			break
		}
		msgData := c.readBuf[:size]
		fds := c.takeFDs(msgData)
		msg, err := decodeMessage(msgData, fds)
		if err == nil {
			c.handle(msg)
			processed++
		}
		c.readBuf = c.readBuf[size:]
	}
	if len(c.readBuf) == 0 {
		c.readBuf = c.readBuf[:0]
	}
	return processed
}

// takeFDs reparte los descriptores pendientes entre los mensajes que aún no se
// han procesado. Como no hay forma de saber cuántos fds lleva cada mensaje sin
// conocer el esquema, se asignan todos al primer mensaje que los pida.
func (c *Conn) takeFDs(msgData []byte) []int {
	if len(c.pendingFds) == 0 {
		return nil
	}
	fds := c.pendingFds
	c.pendingFds = nil
	return fds
}

func parseFDs(oob []byte) []int {
	var out []int
	msgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return nil
	}
	for _, m := range msgs {
		got, err := syscall.ParseUnixRights(&m)
		if err != nil {
			continue
		}
		out = append(out, got...)
	}
	return out
}

func (c *Conn) handle(msg message) {
	o, ok := c.objects[msg.id]
	if !ok {
		// Objeto desconocido: se descartan sus eventos, pero se cierran los fds.
		for {
			if msg.args.fd() < 0 {
				break
			}
		}
		return
	}
	if o.handler != nil {
		o.handler(msg.opcode, &msg.args)
	}
	// Descriptores no consumidos por el manejador: se cierran para no filtrarlos.
	for {
		fd := msg.args.fd()
		if fd < 0 {
			break
		}
		_ = syscall.Close(fd)
	}
}

// Close cierra la conexión y todos los objetos registrados.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.nc.Close()
}

// ─── API de alto nivel ─────────────────────────────────────────────────────

// Roundtrip envía wl_display.sync y espera a que se procesen todos los eventos.
func (c *Conn) Roundtrip() error {
	id := c.allocID("wl_callback", nil)
	done := make(chan struct{}, 1)
	c.setHandler(id, "wl_callback", func(op uint16, d *decoder) {
		if op == callbackDoneEvent {
			d.uint32() // serial del callback
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	msg := newEncoder(1, displaySync).newID(id).bytes()
	if err := c.Send(msg); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		select {
		case <-done:
			delete(c.objects, id)
			return nil
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("tiempo de espera agotado en wl_display.sync")
		}
		if _, err := c.Dispatch(100 * time.Millisecond); err != nil {
			return err
		}
	}
}

// GetRegistry solicita el registro de globals y espera a recibirlos.
func (c *Conn) GetRegistry() ([]Global, error) {
	id := c.allocID("wl_registry", func(op uint16, d *decoder) {
		if op != registryGlobal {
			return
		}
		name := d.uint32()
		iface := d.string()
		version := d.uint32()
		c.globals = append(c.globals, Global{Name: name, Interface: iface, Version: version})
	})
	c.registryID = id
	msg := newEncoder(1, displayGetRegist).newID(id).bytes()
	if err := c.Send(msg); err != nil {
		return nil, err
	}
	if err := c.Roundtrip(); err != nil {
		return c.globals, err
	}
	return c.globals, nil
}

// Bind crea un objeto local a partir de un global.
func (c *Conn) Bind(g Global, version uint32, handler func(uint16, *decoder)) uint32 {
	id := c.allocID(g.Interface, handler)
	regID := c.registryID
	msg := newEncoder(regID, registryBind).
		uint32(g.Name).string(g.Interface).uint32(version).newID(id).bytes()
	_ = c.Send(msg)
	return id
}

// Error del compositor, recibido en wl_display.
type Error struct {
	ObjectID uint32
	Code     uint32
	Message  string
}

func (e Error) Error() string {
	return fmt.Sprintf("error del compositor Wayland (objeto %d, código %d): %s", e.ObjectID, e.Code, e.Message)
}

// RegistryID devuelve el id del objeto registro.
func (c *Conn) RegistryID() uint32 { return c.registryID }

// SetRegistryID fija el registro para poder hacer bind.
func (c *Conn) SetRegistryID(id uint32) {
	c.registryID = id
	c.setHandler(id, "wl_registry", func(op uint16, d *decoder) {
		if op != registryGlobal {
			return
		}
		name := d.uint32()
		iface := d.string()
		version := d.uint32()
		c.globals = append(c.globals, Global{Name: name, Interface: iface, Version: version})
	})
}

// FindGlobal busca un global por interfaz.
func (c *Conn) FindGlobal(iface string) (Global, bool) {
	for _, g := range c.globals {
		if g.Interface == iface {
			return g, true
		}
	}
	return Global{}, false
}

// EnvDisplay devuelve el nombre del display configurado.
func EnvDisplay() string {
	name := os.Getenv("WAYLAND_DISPLAY")
	if name == "" {
		return "wayland-0"
	}
	return name
}

// Available indica si parece haber un compositor Wayland accesible.
func Available() bool {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("SWAYSOCK") == "" {
		return false
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return false
	}
	path := EnvDisplay()
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	_, err := os.Stat(path)
	return err == nil
}
