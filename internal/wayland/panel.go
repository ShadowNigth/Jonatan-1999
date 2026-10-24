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
	Width         int    // ancho en píxeles (0 = lo decide el compositor)
	Height        int    // alto en píxeles
	Anchors       uint32 // combinación de Anchor*
	ExclusiveZone int    // espacio reservado (0 evita zonas exclusivas; -1 permite solaparse)
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
	conn         *Conn
	compositorID uint32
	surface      uint32
	layer        uint32
	damageBuffer bool
	shm          *Shm
	W, H         int
	spec         PanelSpec

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

	compositorVer := versionAtMost(compositor, 4)
	compositorID, err := c.Bind(compositor, compositorVer, nil)
	if err != nil {
		return nil, err
	}
	p := &Panel{
		conn: c, compositorID: compositorID, W: spec.Width, H: spec.Height, spec: spec,
		damageBuffer: compositorVer >= 4,
	}

	p.surface = c.allocID("wl_surface", func(op uint16, d *decoder) {
		switch op {
		case surfaceEnterEvent:
			d.uint32()
		case surfaceLeaveEvent:
			d.uint32()
		}
	})
	if err := c.Send(newEncoder(compositorID, compositorCreateSurface).newID(p.surface).bytes()); err != nil {
		return nil, fmt.Errorf("creando wl_surface del panel: %w", err)
	}

	ver := versionAtMost(shell, 4)
	shellID, err := c.Bind(shell, ver, nil)
	if err != nil {
		return nil, err
	}
	p.layer = c.allocID("zwlr_layer_surface_v1", func(op uint16, d *decoder) {
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

	msg := newEncoder(shellID, layerShellGetSurface).
		newID(p.layer).
		newID(p.surface).
		uint32(0). // sin salida concreta
		uint32(spec.Layer).
		string(spec.Namespace).
		bytes()
	if err := c.Send(msg); err != nil {
		return nil, fmt.Errorf("creando zwlr_layer_surface_v1 del panel: %w", err)
	}

	// Con anclas a izquierda y derecha el ancho lo decide el compositor; si se
	// fija un ancho (paneles flotantes) se pide ese.
	requests := []struct {
		name string
		msg  []byte
	}{
		{"zwlr_layer_surface_v1.set_size", newEncoder(p.layer, layerSurfaceSetSize).uint32(uint32(maxInt(0, spec.Width))).uint32(uint32(spec.Height)).bytes()},
		{"zwlr_layer_surface_v1.set_anchor", newEncoder(p.layer, layerSurfaceSetAnchor).uint32(spec.Anchors).bytes()},
		{"zwlr_layer_surface_v1.set_exclusive_zone", newEncoder(p.layer, layerSurfaceSetExclusiveZone).int32(int32(spec.ExclusiveZone)).bytes()},
	}
	if spec.MarginTop != 0 || spec.MarginRight != 0 || spec.MarginBottom != 0 || spec.MarginLeft != 0 {
		requests = append(requests, struct {
			name string
			msg  []byte
		}{"zwlr_layer_surface_v1.set_margin", newEncoder(p.layer, layerSurfaceSetMargin).
			int32(int32(spec.MarginTop)).int32(int32(spec.MarginRight)).
			int32(int32(spec.MarginBottom)).int32(int32(spec.MarginLeft)).bytes()})
	}
	kbd := uint32(0)
	if spec.Keyboard {
		kbd = 1
	}
	requests = append(requests,
		struct {
			name string
			msg  []byte
		}{"zwlr_layer_surface_v1.set_keyboard_interactivity", newEncoder(p.layer, layerSurfaceSetKeyboard).uint32(kbd).bytes()},
		struct {
			name string
			msg  []byte
		}{"wl_surface.commit", newEncoder(p.surface, surfaceCommit).bytes()},
	)
	for _, req := range requests {
		if err := c.Send(req.msg); err != nil {
			return nil, fmt.Errorf("%s: %w", req.name, err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		p.mu.Lock()
		done, closed := p.configured, p.closed
		p.mu.Unlock()
		if done {
			break
		}
		if closed {
			return p, fmt.Errorf("el compositor cerró el panel antes de configurarlo")
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
	if err := p.conn.Send(newEncoder(p.surface, surfaceAttach).uint32(buf.ID).int32(0).int32(0).bytes()); err != nil {
		return fmt.Errorf("adjuntando el búfer del panel: %w", err)
	}
	if err := sendSurfaceDamage(p.conn, p.surface, buf.W, buf.H, p.damageBuffer); err != nil {
		return fmt.Errorf("marcando el área del panel: %w", err)
	}
	if err := p.conn.Send(newEncoder(p.surface, surfaceCommit).bytes()); err != nil {
		return fmt.Errorf("confirmando el fotograma del panel: %w", err)
	}
	return nil
}

// RequestSize pide al compositor un tamaño nuevo (por ejemplo, para crecer
// cuando hay más notificaciones). El compositor responde con un configure que
// se recoge en Dispatch; después NeedsResize y Resize hacen el resto.
func (p *Panel) RequestSize(w, h int) error {
	if w < 0 {
		w = 0
	}
	if h < 1 {
		h = 1
	}
	p.mu.Lock()
	pending := p.cfgW == w && p.cfgH == h
	p.mu.Unlock()
	if pending {
		return nil
	}
	if err := p.conn.Send(newEncoder(p.layer, layerSurfaceSetSize).
		uint32(uint32(w)).uint32(uint32(h)).bytes()); err != nil {
		return err
	}
	return p.conn.Send(newEncoder(p.surface, surfaceCommit).bytes())
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
