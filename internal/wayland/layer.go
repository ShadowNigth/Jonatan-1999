package wayland

import (
	"fmt"
	"sync"
	"time"
)

// Background es una superficie de fondo en la capa "background" del
// compositor, creada con wlr-layer-shell.
type Background struct {
	conn    *Conn
	surface uint32
	layer   uint32
	shm     *Shm
	W, H    int

	mu         sync.Mutex
	configured bool
	closed     bool
	ackSerial  uint32
	cfgW, cfgH int

	// FrameDone se cierra cuando el compositor pide el siguiente fotograma.
	frameWait chan struct{}
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

	b := &Background{conn: c, W: w, H: h, frameWait: make(chan struct{}, 1)}

	// wl_surface
	b.surface = c.allocID("wl_surface", func(op uint16, d *decoder) {
		switch op {
		case surfaceEnterEvent:
			d.uint32() // output
		case surfaceLeaveEvent:
			d.uint32()
		}
	})
	_ = c.Send(newEncoder(compositor.Name, compositorCreateSurface).newID(b.surface).bytes())

	// zwlr_layer_shell_v1.get_layer_surface(surface, output=nil, layer=background, namespace)
	layerVer := layerShell.Version
	if layerVer > 4 {
		layerVer = 4
	}
	sheetID := c.Bind(layerShell, layerVer, func(op uint16, d *decoder) {
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
	b.layer = sheetID
	msg := newEncoder(sheetID, layerShellGetSurface).
		newID(b.layer).
		newID(b.surface).
		uint32(0). // sin salida concreta: todas
		uint32(LayerBackground).
		string(namespace).
		bytes()
	if err := c.Send(msg); err != nil {
		return nil, err
	}

	// set_size(0,0) = ocupar el espacio que indique el compositor,
	// set_anchor(todos los bordes) y zona exclusiva 0 (no reserva espacio).
	_ = c.Send(newEncoder(b.layer, layerSurfaceSetSize).uint32(0).uint32(0).bytes())
	_ = c.Send(newEncoder(b.layer, layerSurfaceSetAnchor).
		uint32(AnchorTop | AnchorBottom | AnchorLeft | AnchorRight).bytes())
	_ = c.Send(newEncoder(b.layer, layerSurfaceSetExclusiveZone).int32(0).bytes())
	_ = c.Send(newEncoder(b.layer, layerSurfaceSetKeyboard).uint32(0).bytes())
	// Superficie opaca (sin alfa) para que el compositor pueda optimizar.
	_ = c.Send(newEncoder(b.layer, layerSurfaceSetLayer).uint32(LayerBackground).bytes())
	_ = c.Send(newEncoder(b.surface, surfaceCommit).bytes())

	// Espera el primer configure.
	deadline := time.Now().Add(3 * time.Second)
	for {
		b.mu.Lock()
		ok := b.configured
		closed := b.closed
		b.mu.Unlock()
		if ok || closed {
			break
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
	_ = b.conn.Send(newEncoder(b.surface, surfaceAttach).uint32(buf.ID).int32(0).int32(0).bytes())
	_ = b.conn.Send(newEncoder(b.surface, surfaceDamageBuf).
		int32(0).int32(0).int32(int32(buf.W)).int32(int32(buf.H)).bytes())
	_ = b.conn.Send(newEncoder(b.surface, surfaceCommit).bytes())
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
