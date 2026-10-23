package swayipc

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event es un evento recibido de sway.
type Event struct {
	Type    MessageType
	Payload json.RawMessage
	// Raw conserva el objeto completo (incluye la clave "change").
	Raw map[string]json.RawMessage
}

// Change devuelve el campo "change" del evento, si existe.
func (e *Event) Change() string {
	var s string
	if v, ok := e.Raw["change"]; ok {
		_ = json.Unmarshal(v, &s)
	}
	return s
}

// WorkspaceEvent agrupa los datos de un evento de workspace.
type WorkspaceEvent struct {
	Change  string    `json:"change"`
	Old     Workspace `json:"old"`
	Current Workspace `json:"current"`
}

// WindowEvent agrupa los datos de un evento de ventana.
type WindowEvent struct {
	Change    string `json:"change"`
	Container Node   `json:"container"`
}

// ModeEvent informa de un cambio de modo de binding.
type ModeEvent struct {
	Change string `json:"change"`
	Pango  string `json:"pango_markup"`
}

// TickEvent es el payload de un evento TICK.
type TickEvent struct {
	First   bool   `json:"first"`
	Payload string `json:"payload"`
}

// OutputEvent informa de cambios en las salidas.
type OutputEvent struct {
	Change string `json:"change"`
}

// Subscribe se suscribe a una o más categorías ("workspace", "window",
// "mode", "tick", "output", "binding", ...). Devuelve la primera respuesta
// (confirmación) y deja la conexión lista para leer eventos.
func (c *Conn) Subscribe(events ...string) error {
	raw, err := c.RoundTrip(MsgSubscribe, events)
	if err != nil {
		return err
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil {
		return fmt.Errorf("respuesta SUBSCRIBE inesperada: %w", err)
	}
	if !ok {
		return fmt.Errorf("sway rechazó la suscripción a %v", events)
	}
	return nil
}

// Next lee el siguiente evento. Bloquea hasta que llegue uno o falle la conexión.
func (c *Conn) Next() (*Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}
	if c.timeout > 0 {
		_ = c.nc.SetDeadline(time.Now().Add(c.timeout))
	} else {
		_ = c.nc.SetDeadline(time.Time{})
	}
	payload, t, err := readFrame(c.br)
	if err != nil {
		return nil, err
	}
	ev := &Event{Type: t, Payload: payload, Raw: map[string]json.RawMessage{}}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &ev.Raw)
	}
	return ev, nil
}

// Events lee eventos y los entrega por el canal devuelto hasta que la
// conexión falle o se llame a Close.
func (c *Conn) Events(buf int) <-chan *Event {
	ch := make(chan *Event, buf)
	go func() {
		defer close(ch)
		for {
			ev, err := c.Next()
			if err != nil {
				return
			}
			ch <- ev
		}
	}()
	return ch
}

// Decode decodifica el payload del evento en v.
func (e *Event) Decode(v any) error {
	return json.Unmarshal(e.Payload, v)
}
