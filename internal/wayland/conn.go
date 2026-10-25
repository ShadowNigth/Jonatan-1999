package wayland

import (
	"errors"
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
	// registryID es el objeto wl_registry. shmID conserva un único enlace a
	// wl_shm durante la conexión; los paneles pueden reconstruir sus pools sin
	// crear un objeto global nuevo por cada redimensionado.
	registryID uint32
	shmID      uint32
	closed     bool
	fatalErr   error
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
	c.objects[1] = &object{id: 1, iface: "wl_display", handler: func(op uint16, d *decoder) {
		switch op {
		case displayErrorEvent:
			c.fatalErr = Error{ObjectID: d.uint32(), Code: d.uint32(), Message: d.string()}
		case displayDeleteID:
			d.uint32() // los ids no se reutilizan, pero hay que consumir el evento
		}
	}}
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
// Un timeout cero o negativo sondea el socket sin bloquear.
//
// Devuelve el número de mensajes procesados.
func (c *Conn) Dispatch(timeout time.Duration) (int, error) {
	// Primero se procesa lo que quedó pendiente de la lectura anterior.
	processed := c.drain()
	if c.fatalErr != nil {
		return processed, c.fatalErr
	}

	buf := make([]byte, 64*1024)
	oob := make([]byte, syscall.CmsgSpace(4*8))
	var n, oobn int
	var err error
	if timeout <= 0 {
		// Un deadline cero en net.Conn significa «sin límite», no lectura
		// inmediata. Los loops de Wayland usan Dispatch(0) para no bloquear
		// el render/IPC mientras sondean eventos pendientes, así que usamos
		// recvmsg(MSG_DONTWAIT) dentro del control seguro del descriptor.
		n, oobn, err = c.readAvailable(buf, oob)
	} else {
		_ = c.nc.SetReadDeadline(time.Now().Add(timeout))
		n, oobn, _, _, err = c.nc.ReadMsgUnix(buf, oob)
	}
	// Un Read puede devolver bytes y también un error (por ejemplo, EOF tras
	// recibir el último evento); procesa siempre los bytes antes de propagarlo.
	if n > 0 {
		fds := parseFDs(oob[:oobn])
		c.readBuf = append(c.readBuf, buf[:n]...)
		c.pendingFds = append(c.pendingFds, fds...)
		processed += c.drain()
	}
	if c.fatalErr != nil {
		return processed, c.fatalErr
	}
	if err != nil {
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			return processed, nil
		}
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return processed, nil
		}
		if errors.Is(err, syscall.EINTR) {
			return processed, nil
		}
		return processed, err
	}
	if n == 0 {
		return processed, fmt.Errorf("el compositor cerró la conexión")
	}
	return processed, nil
}

// readAvailable lee un lote sin esperar a que llegue un evento. SyscallConn
// mantiene el descriptor protegido mientras recvmsg usa MSG_DONTWAIT.
func (c *Conn) readAvailable(buf, oob []byte) (int, int, error) {
	raw, err := c.nc.SyscallConn()
	if err != nil {
		return 0, 0, err
	}
	var n, oobn int
	var recvErr error
	if err := raw.Control(func(fd uintptr) {
		n, oobn, _, _, recvErr = syscall.Recvmsg(int(fd), buf, oob, syscall.MSG_DONTWAIT|syscall.MSG_CMSG_CLOEXEC)
	}); err != nil {
		return 0, 0, err
	}
	return n, oobn, recvErr
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

// Bind enlaza un global anunciado por wl_registry y devuelve el id del objeto
// cliente creado. El nombre del global no es un id de objeto y no debe usarse
// para enviar solicitudes de interfaz.
func (c *Conn) Bind(g Global, version uint32, handler func(uint16, *decoder)) (uint32, error) {
	if version == 0 || version > g.Version {
		return 0, fmt.Errorf("versión inválida de %s: %d (máxima anunciada: %d)", g.Interface, version, g.Version)
	}
	if c.registryID == 0 {
		return 0, fmt.Errorf("no se puede enlazar %s antes de obtener wl_registry", g.Interface)
	}
	id := c.allocID(g.Interface, handler)
	msg := newEncoder(c.registryID, registryBind).
		uint32(g.Name).string(g.Interface).uint32(version).newID(id).bytes()
	if err := c.Send(msg); err != nil {
		delete(c.objects, id)
		return 0, fmt.Errorf("enlazando %s: %w", g.Interface, err)
	}
	return id, nil
}

func versionAtMost(g Global, max uint32) uint32 {
	if max > 0 && g.Version > max {
		return max
	}
	return g.Version
}

// Error del compositor, recibido en wl_display.
type Error struct {
	ObjectID uint32
	Code     uint32
	Message  string
}

func (e Error) Error() string {
	return fmt.Sprintf("error de protocolo Wayland (objeto %d, código %d): %s", e.ObjectID, e.Code, e.Message)
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
