package wayland

import (
	"fmt"
	"sync"
	"time"
)

// Background es una superficie de fondo en la capa "background" del
// compositor, creada con wlr-layer-shell.
type Background struct {
	conn         *Conn
	surface      uint32
	layer        uint32
	damageBuffer bool
	shm          *Shm
	W, H         int

	mu         sync.Mutex
	configured bool
	closed     bool
	ackSerial  uint32
	cfgW, cfgH int

	// FrameDone se cierra cuando el compositor pide el siguiente fotograma.
	frameWait chan struct{}
}

// sendSurfaceDamage usa damage_buffer solo si la versión negociada de
// wl_surface es >= 4; en versiones anteriores damage toma coordenadas locales.
func sendSurfaceDamage(c *Conn, surface uint32, w, h int, bufferCoords bool) error {
	opcode := uint16(surfaceDamage)
	if bufferCoords {
		opcode = uint16(surfaceDamageBuf)
	}
	return c.Send(newEncoder(surface, opcode).
		int32(0).int32(0).int32(int32(w)).int32(int32(h)).bytes())
}

// NewBackground crea una superficie de fondo del tamaño indicado.
//
// Devuelve error si el compositor no ofrece wlr-layer-shell (por ejemplo, en
// sesiones que no sean wlroots/sway).
func NewBackground(c *Conn, w, h int, namespace string) (*Background, error) {
	compositor, ok := c.FindGlobal("wl_compositor")
	if !ok {
		return nil, fmt.Errorf("el compositor no ofrece wl_compositor")
	}
	layerShell, ok := c.FindGlobal("zwlr_layer_shell_v1")
	if !ok {
		return nil, fmt.Errorf("el compositor no ofrece wlr-layer-shell (se necesita sway 1.x)")
	}
	if namespace == "" {
		namespace = "swaydesk-wallpaper"
	}

	compositorVer := versionAtMost(compositor, 4)
	compositorID, err := c.Bind(compositor, compositorVer, nil)
	if err != nil {
		return nil, err
	}
	b := &Background{
		conn: c, W: w, H: h, frameWait: make(chan struct{}, 1),
		damageBuffer: compositorVer >= 4,
	}

	// El nombre de un global solo sirve para wl_registry.bind; las solicitudes
	// wl_compositor deben enviarse al id local que devuelve Bind.
	b.surface = c.allocID("wl_surface", func(op uint16, d *decoder) {
		switch op {
		case surfaceEnterEvent:
			d.uint32() // output
		case surfaceLeaveEvent:
			d.uint32()
		}
	})
	if err := c.Send(newEncoder(compositorID, compositorCreateSurface).newID(b.surface).bytes()); err != nil {
		return nil, fmt.Errorf("creando wl_surface: %w", err)
	}

	// zwlr_layer_shell_v1.get_layer_surface(surface, output=nil, layer=background, namespace).
	// El objeto zwlr_layer_shell_v1 y el zwlr_layer_surface_v1 devuelto tienen
	// ids distintos; reutilizar el primero como new_id provoca un error fatal.
	layerVer := versionAtMost(layerShell, 4)
	shellID, err := c.Bind(layerShell, layerVer, nil)
	if err != nil {
		return nil, err
	}
	b.layer = c.allocID("zwlr_layer_surface_v1", func(op uint16, d *decoder) {
		switch op {
		case layerSurfaceConfigureEvent:
			serial := d.uint32()
			w := d.uint32()
			h := d.uint32()
			b.mu.Lock()
			b.configured = true
			b.ackSerial = serial
			if w > 0 {
				b.cfgW = int(w)
			}
			if h > 0 {
				b.cfgH = int(h)
			}
			b.mu.Unlock()
		case layerSurfaceClosedEvent:
			b.mu.Lock()
			b.closed = true
			b.mu.Unlock()
		}
	})
	msg := newEncoder(shellID, layerShellGetSurface).
		newID(b.layer).
		newID(b.surface).
		uint32(0). // sin salida concreta: todas
		uint32(LayerBackground).
		string(namespace).
		bytes()
	if err := c.Send(msg); err != nil {
		return nil, fmt.Errorf("creando zwlr_layer_surface_v1: %w", err)
	}

	// set_size(0,0) = ocupar el espacio que indique el compositor,
	// set_anchor(todos los bordes) y zona exclusiva 0 (no reserva espacio).
	requests := []struct {
		name string
		msg  []byte
	}{
		{"zwlr_layer_surface_v1.set_size", newEncoder(b.layer, layerSurfaceSetSize).uint32(0).uint32(0).bytes()},
		{"zwlr_layer_surface_v1.set_anchor", newEncoder(b.layer, layerSurfaceSetAnchor).uint32(AnchorTop | AnchorBottom | AnchorLeft | AnchorRight).bytes()},
		{"zwlr_layer_surface_v1.set_exclusive_zone", newEncoder(b.layer, layerSurfaceSetExclusiveZone).int32(0).bytes()},
		{"zwlr_layer_surface_v1.set_keyboard_interactivity", newEncoder(b.layer, layerSurfaceSetKeyboard).uint32(0).bytes()},
		{"wl_surface.commit", newEncoder(b.surface, surfaceCommit).bytes()},
	}
	for _, req := range requests {
		if err := c.Send(req.msg); err != nil {
			return nil, fmt.Errorf("%s: %w", req.name, err)
		}
	}

	// Espera el primer configure.
	deadline := time.Now().Add(3 * time.Second)
	for {
		b.mu.Lock()
		ok := b.configured
		closed := b.closed
		b.mu.Unlock()
		if ok {
			break
		}
		if closed {
			return b, fmt.Errorf("el compositor cerró la superficie de fondo antes de configurarla")
		}
		if time.Now().After(deadline) {
			return b, fmt.Errorf("el compositor no envió configure para la superficie de fondo")
		}
		if _, err := c.Dispatch(100 * time.Millisecond); err != nil {
			return b, err
		}
	}
	return b, nil
}

// Size devuelve el tamaño real de la superficie tras el configure.
func (b *Background) Size() (int, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	w, h := b.W, b.H
	if b.cfgW > 0 {
		w = b.cfgW
	}
	if b.cfgH > 0 {
		h = b.cfgH
	}
	return w, h
}

// Configured indica si el compositor ya confirmó el tamaño.
func (b *Background) Configured() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.configured
}

// Closed indica si el compositor cerró la superficie.
func (b *Background) Closed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// Ack confirma el último configure recibido, si aún no se hizo.
func (b *Background) Ack() {
	b.mu.Lock()
	serial := b.ackSerial
	b.ackSerial = 0
	b.mu.Unlock()
	if serial > 0 {
		_ = b.conn.Send(newEncoder(b.layer, layerSurfaceAckConfigure).uint32(serial).bytes())
	}
}

// InitShm prepara los búferes de memoria compartida al tamaño actual.
func (b *Background) InitShm(count int) error {
	w, h := b.Size()
	if w <= 0 || h <= 0 {
		return fmt.Errorf("tamaño de superficie inválido: %dx%d", w, h)
	}
	shm, err := NewShm(b.conn, w, h, count)
	if err != nil {
		return err
	}
	shm.BindReleases()
	b.shm = shm
	b.W, b.H = w, h
	return nil
}

// Draw sube un fotograma RGB (3 bytes por píxel) a la superficie.
func (b *Background) Draw(pix []uint8, srcW, srcH int) error {
	if b.shm == nil {
		return fmt.Errorf("la memoria compartida no está inicializada")
	}
	if b.Closed() {
		return fmt.Errorf("la superficie de fondo fue cerrada por el compositor")
	}
	b.Ack()
	buf := b.shm.Next()
	buf.FillRGB(pix, srcW, srcH)
	if err := b.conn.Send(newEncoder(b.surface, surfaceAttach).uint32(buf.ID).int32(0).int32(0).bytes()); err != nil {
		return fmt.Errorf("adjuntando el búfer del fondo: %w", err)
	}
	if err := sendSurfaceDamage(b.conn, b.surface, buf.W, buf.H, b.damageBuffer); err != nil {
		return fmt.Errorf("marcando el área del fondo: %w", err)
	}
	if err := b.conn.Send(newEncoder(b.surface, surfaceCommit).bytes()); err != nil {
		return fmt.Errorf("confirmando el fotograma del fondo: %w", err)
	}
	return nil
}

// NeedsResize indica si el compositor cambió el tamaño y hay que rehacer la
// memoria compartida.
func (b *Background) NeedsResize() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.configured || b.cfgW == 0 || b.cfgH == 0 {
		return false
	}
	return b.cfgW != b.W || b.cfgH != b.H
}

// Dispatch procesa eventos pendientes del compositor.
func (b *Background) Dispatch(timeout time.Duration) error {
	if b.Closed() {
		return fmt.Errorf("la superficie de fondo fue cerrada por el compositor")
	}
	_, err := b.conn.Dispatch(timeout)
	return err
}

// Close destruye la superficie y libera la memoria.
func (b *Background) Close() {
	if b.shm != nil {
		b.shm.Close()
	}
	_ = b.conn.Send(newEncoder(b.layer, layerSurfaceDestroy).bytes())
	_ = b.conn.Send(newEncoder(b.surface, surfaceDestroy).bytes())
	_ = b.conn.Roundtrip()
	b.conn.Close()
}

// DisplayServer describe el entorno gráfico disponible.
type DisplayServer struct {
	Wayland    bool
	LayerShell bool
	Name       string
}

// Detect sondea el compositor para saber qué se puede usar.
func Detect() DisplayServer {
	ds := DisplayServer{Name: "ninguno"}
	if !Available() {
		return ds
	}
	c, err := NewConn()
	if err != nil {
		return ds
	}
	defer c.Close()
	ds.Wayland = true
	ds.Name = EnvDisplay()
	globals, err := c.GetRegistry()
	if err != nil {
		return ds
	}
	for _, g := range globals {
		if g.Interface == "zwlr_layer_shell_v1" {
			ds.LayerShell = true
		}
	}
	return ds
}

// ReinitShm vuelve a crear la memoria compartida con el nuevo tamaño tras un
// configure del compositor (por ejemplo, al cambiar la resolución).
func (b *Background) ReinitShm(count int) error {
	if b.shm != nil {
		b.shm.Close()
		b.shm = nil
	}
	w, h := b.Size()
	b.W, b.H = w, h
	return b.InitShm(count)
}
