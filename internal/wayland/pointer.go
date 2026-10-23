package wayland

import (
	"fmt"
	"sync"
)

// PointerHandler recibe los eventos del ratón. Todos los métodos pueden ser
// nil; solo se llaman los que estén definidos.
//
// Las coordenadas son píxeles de la superficie en coordenadas lógicas
// (las mismas que el tamaño lógico del panel).
type PointerHandler struct {
	Enter  func(x, y float64)
	Leave  func()
	Motion func(x, y float64)
	// Button recibe el botón (0x110 = izquierdo, 0x111 = central,
	// 0x112 = derecho) y si se pulsó (true) o se soltó (false).
	Button func(x, y float64, button uint32, pressed bool)
	Axis   func(x, y float64, value float64)
}

// Pointer es un wl_pointer asociado a un wl_seat.
type Pointer struct {
	conn    *Conn
	seat    uint32
	pointer uint32
	handler PointerHandler

	mu      sync.Mutex
	x, y    float64
	inside  bool
	havePos bool
	closed  bool
}

// NewPointer se asocia al primer asiento del compositor y empieza a recibir
// eventos del ratón. Si el compositor no ofrece wl_seat devuelve un puntero
// inerte (sin error), para que la interfaz siga funcionando con teclado.
func NewPointer(c *Conn, h PointerHandler) (*Pointer, error) {
	p := &Pointer{conn: c, handler: h}
	seat, ok := c.FindGlobal("wl_seat")
	if !ok {
		return p, nil
	}
	ver := seat.Version
	if ver > 7 {
		ver = 7
	}
	p.seat = c.Bind(seat, ver, func(op uint16, d *decoder) {
		switch op {
		case seatCapabilities:
			caps := d.uint32()
			p.mu.Lock()
			havePointer := p.pointer != 0
			p.mu.Unlock()
			if caps&capPointer != 0 && !havePointer {
				p.grabPointer()
			}
		}
	})
	return p, nil
}

// grabPointer crea el objeto wl_pointer dentro del asiento.
func (p *Pointer) grabPointer() {
	p.mu.Lock()
	if p.pointer != 0 || p.closed {
		p.mu.Unlock()
		return
	}
	id := p.conn.allocID("wl_pointer", func(op uint16, d *decoder) {
		switch op {
		case pointerEnterEvent:
			d.uint32() // serial
			d.uint32() // superficie
			x, y := d.fixed(), d.fixed()
			p.mu.Lock()
			p.x, p.y, p.inside, p.havePos = x, y, true, true
			p.mu.Unlock()
			if p.handler.Enter != nil {
				p.handler.Enter(x, y)
			}
		case pointerLeaveEvent:
			d.uint32()
			d.uint32()
			p.mu.Lock()
			p.inside = false
			p.mu.Unlock()
			if p.handler.Leave != nil {
				p.handler.Leave()
			}
		case pointerMotionEvent:
			d.uint32() // tiempo
			x, y := d.fixed(), d.fixed()
			p.mu.Lock()
			p.x, p.y, p.havePos = x, y, true
			p.mu.Unlock()
			if p.handler.Motion != nil {
				p.handler.Motion(x, y)
			}
		case pointerButtonEvent:
			d.uint32() // serial
			d.uint32() // tiempo
			button := d.uint32()
			state := d.uint32()
			p.mu.Lock()
			x, y, ok := p.x, p.y, p.havePos
			p.mu.Unlock()
			if ok && p.handler.Button != nil {
				p.handler.Button(x, y, button, state == 1)
			}
		case pointerAxisEvent:
			d.uint32()
			d.uint32() // eje
			value := d.fixed()
			p.mu.Lock()
			x, y, ok := p.x, p.y, p.havePos
			p.mu.Unlock()
			if ok && p.handler.Axis != nil {
				p.handler.Axis(x, y, value)
			}
		}
	})
	p.pointer = id
	p.mu.Unlock()
	msg := newEncoder(p.seat, seatGetPointer).newID(id).bytes()
	_ = p.conn.Send(msg)
}

// Position devuelve la última posición conocida y si el cursor está dentro.
func (p *Pointer) Position() (float64, float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.x, p.y, p.inside
}

// Close libera el objeto.
func (p *Pointer) Close() {
	p.mu.Lock()
	p.closed = true
	id := p.pointer
	p.mu.Unlock()
	if id != 0 {
		_ = p.conn.Send(newEncoder(id, pointerRelease).bytes())
	}
}

// SetInputRegion limita qué zonas de la superficie reciben clics; las demás
// dejan pasar el ratón a lo que haya debajo. Un rectángulo vacío (w o h <= 0)
// deja toda la superficie transparente al ratón.
func (p *Panel) SetInputRegion(rects [][4]int) error {
	comp, ok := p.conn.FindGlobal("wl_compositor")
	if !ok {
		return fmt.Errorf("el compositor no ofrece wl_compositor")
	}
	region := p.conn.allocID("wl_region", nil)
	_ = p.conn.Send(newEncoder(comp.Name, compositorCreateRegion).newID(region).bytes())
	for _, r := range rects {
		if r[2] <= 0 || r[3] <= 0 {
			continue
		}
		_ = p.conn.Send(newEncoder(region, regionAdd).
			int32(int32(r[0])).int32(int32(r[1])).
			int32(int32(r[2])).int32(int32(r[3])).bytes())
	}
	_ = p.conn.Send(newEncoder(p.surface, surfaceSetInput).uint32(region).bytes())
	_ = p.conn.Send(newEncoder(region, regionDestroy).bytes())
	return p.conn.Send(newEncoder(p.surface, surfaceCommit).bytes())
}

// Surface devuelve el id de la wl_surface del panel.
func (p *Panel) Surface() uint32 { return p.surface }
