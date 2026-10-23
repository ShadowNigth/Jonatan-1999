package wayland

import (
	"fmt"
	"sync"
	"time"
)

// PanelSpec describe una superficie tipo panel (barra, OSD, dock).
type PanelSpec struct {
	Namespace     string
	Layer         uint32 // LayerBackground, LayerBottom, LayerTop, LayerOverlay
	Height        int    // alto en píxeles
	Anchors       uint32 // combinación de Anchor*
	ExclusiveZone int    // espacio reservado al compositor (0 = ninguno)
	MarginTop     int
	MarginRight   int
	MarginBottom  int
	MarginLeft    int
	Keyboard      bool // recibir el teclado (0 = ninguno, 1 = exclusivo, 2 = bajo demanda)
	Opaque        bool // declarar la superficie opaca (sin alfa)
}

// Panel es una superficie de wlr-layer-shell pensada para interfaces
// permanentes: barras, docks y avisos en pantalla.
//
// A diferencia de Background, admite márgenes, zona exclusiva y transparencia.
type Panel struct {
	conn    *Conn
	surface uint32
	layer   uint32
	shm     *Shm
	W, H    int
	spec    PanelSpec

	mu         sync.Mutex
	configured bool
	closed     bool
	ackSerial  uint32
	cfgW, cfgH int
}

// NewPanel crea y configura una superficie de panel.
func NewPanel(c *Conn, spec PanelSpec) (*Panel, error) {
	compositor, ok := c.FindGlobal("wl_compositor")
	if !ok {
		return nil, fmt.Errorf("el compositor no ofrece wl_compositor")
	}
	shell, ok := c.FindGlobal("zwlr_layer_shell_v1")
	if !ok {
		return nil, fmt.Errorf("el compositor no ofrece wlr-layer-shell (hace falta sway 1.x)")
	}
	if spec.Namespace == "" {
		spec.Namespace = "swaydesk-bar"
	}
	if spec.Height <= 0 {
		spec.Height = 32
	}
	if spec.Anchors == 0 {
		spec.Anchors = AnchorTop | AnchorLeft | AnchorRight
	}
	if spec.Layer > LayerOverlay {
		spec.Layer = LayerTop
	}

	p := &Panel{conn: c, H: spec.Height, spec: spec}

	p.surface = c.allocID("wl_surface", func(op uint16, d *decoder) {
		switch op {
		case surfaceEnterEvent:
			d.uint32()
		case surfaceLeaveEvent:
			d.uint32()
		}
	})
	_ = c.Send(newEncoder(compositor.Name, compositorCreateSurface).newID(p.surface).bytes())

	ver := shell.Version
	if ver > 4 {
		ver = 4
	}
	sheet := c.Bind(shell, ver, func(op uint16, d *decoder) {
		switch op {
		case layerSurfaceConfigureEvent:
			serial := d.uint32()
			w := d.uint32()
			h := d.uint32()
			p.mu.Lock()
			p.configured = true
			p.ackSerial = serial
			if w > 0 {
				p.cfgW = int(w)
			}
			if h > 0 {
				p.cfgH = int(h)
			}
			p.mu.Unlock()
		case layerSurfaceClosedEvent:
			p.mu.Lock()
			p.closed = true
			p.mu.Unlock()
		}
	})
	p.layer = sheet

	msg := newEncoder(sheet, layerShellGetSurface).
		newID(p.layer).
		newID(p.surface).
		uint32(0). // sin salida concreta
		uint32(spec.Layer).
		string(spec.Namespace).
		bytes()
	if err := c.Send(msg); err != nil {
		return nil, err
	}

	// Alto fijo; el ancho lo decide el compositor al anclar a izquierda y derecha.
	_ = c.Send(newEncoder(p.layer, layerSurfaceSetSize).uint32(0).uint32(uint32(spec.Height)).bytes())
	_ = c.Send(newEncoder(p.layer, layerSurfaceSetAnchor).uint32(spec.Anchors).bytes())
	_ = c.Send(newEncoder(p.layer, layerSurfaceSetExclusiveZone).int32(int32(spec.ExclusiveZone)).bytes())
	if spec.MarginTop != 0 || spec.MarginRight != 0 || spec.MarginBottom != 0 || spec.MarginLeft != 0 {
		_ = c.Send(newEncoder(p.layer, layerSurfaceSetMargin).
			int32(int32(spec.MarginTop)).int32(int32(spec.MarginRight)).
			int32(int32(spec.MarginBottom)).int32(int32(spec.MarginLeft)).bytes())
	}
	kbd := uint32(0)
	if spec.Keyboard {
		kbd = 1
	}
	_ = c.Send(newEncoder(p.layer, layerSurfaceSetKeyboard).uint32(kbd).bytes())
	_ = c.Send(newEncoder(p.layer, layerSurfaceSetLayer).uint32(spec.Layer).bytes())
	_ = c.Send(newEncoder(p.surface, surfaceCommit).bytes())

	deadline := time.Now().Add(3 * time.Second)
	for {
		p.mu.Lock()
		done, closed := p.configured, p.closed
		p.mu.Unlock()
		if done || closed {
			break
		}
		if time.Now().After(deadline) {
			return p, fmt.Errorf("el compositor no confirmó el panel")
		}
		if _, err := c.Dispatch(100 * time.Millisecond); err != nil {
			return p, err
		}
	}
	return p, nil
}

// Size devuelve el tamaño confirmado por el compositor.
func (p *Panel) Size() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w, h := p.W, p.H
	if p.cfgW > 0 {
		w = p.cfgW
	}
	if p.cfgH > 0 {
		h = p.cfgH
	}
	return w, h
}

// Closed indica si el compositor cerró la superficie.
func (p *Panel) Closed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

// Ack confirma el último configure pendiente.
func (p *Panel) Ack() {
	p.mu.Lock()
	serial := p.ackSerial
	p.ackSerial = 0
	p.mu.Unlock()
	if serial > 0 {
		_ = p.conn.Send(newEncoder(p.layer, layerSurfaceAckConfigure).uint32(serial).bytes())
	}
}

// InitShm prepara los búferes con el tamaño actual.
func (p *Panel) InitShm(count int) error {
	w, h := p.Size()
	if w <= 0 || h <= 0 {
		return fmt.Errorf("tamaño de panel inválido: %dx%d", w, h)
	}
	shm, err := NewShm(p.conn, w, h, count)
	if err != nil {
		return err
	}
	shm.BindReleases()
	p.shm = shm
	p.W, p.H = w, h
	return nil
}

// Draw sube un fotograma en RGBA de 8 bits (alfa incluido).
func (p *Panel) Draw(pix []uint8, srcW, srcH int) error {
	if p.shm == nil {
		return fmt.Errorf("la memoria compartida del panel no está inicializada")
	}
	if p.Closed() {
		return fmt.Errorf("el compositor cerró el panel")
	}
	p.Ack()
	buf := p.shm.Next()
	buf.FillRGBA(pix, srcW, srcH)
	_ = p.conn.Send(newEncoder(p.surface, surfaceAttach).uint32(buf.ID).int32(0).int32(0).bytes())
	_ = p.conn.Send(newEncoder(p.surface, surfaceDamageBuf).
		int32(0).int32(0).int32(int32(buf.W)).int32(int32(buf.H)).bytes())
	_ = p.conn.Send(newEncoder(p.surface, surfaceCommit).bytes())
	return nil
}

// NeedsResize indica si el tamaño cambió.
func (p *Panel) NeedsResize() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.configured || p.cfgW == 0 || p.cfgH == 0 {
		return false
	}
	return p.cfgW != p.W || p.cfgH != p.H
}

// Resize rehace la memoria compartida al nuevo tamaño.
func (p *Panel) Resize(count int) error {
	if p.shm != nil {
		p.shm.Close()
		p.shm = nil
	}
	return p.InitShm(count)
}

// Dispatch procesa eventos con un tiempo límite.
func (p *Panel) Dispatch(timeout time.Duration) error {
	if p.Closed() {
		return fmt.Errorf("el compositor cerró el panel")
	}
	_, err := p.conn.Dispatch(timeout)
	return err
}

// Conn devuelve la conexión subyacente (para suscribirse a eventos de sway no
// hace falta, pero permite reutilizarla).
func (p *Panel) Conn() *Conn { return p.conn }

// Close destruye la superficie.
func (p *Panel) Close() {
	if p.shm != nil {
		p.shm.Close()
	}
	_ = p.conn.Send(newEncoder(p.layer, layerSurfaceDestroy).bytes())
	_ = p.conn.Send(newEncoder(p.surface, surfaceDestroy).bytes())
	_ = p.conn.Roundtrip()
	p.conn.Close()
}
