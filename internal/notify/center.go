package notify

import (
	"strconv"
	"strings"
	"time"
)

// Entry es una fila del centro de notificaciones: una notificación viva o una
// entrada del historial, con lo necesario para dibujarla y accionarla.
type Entry struct {
	Notification
	// Live indica que sigue en pantalla (se puede cerrar o responder ahora).
	Live bool
	// ClosedAt y Reason solo tienen sentido en el historial.
	ClosedAt time.Time
	Reason   string
}

// Badges devuelve las etiquetas cortas que describen la entrada.
func (e Entry) Badges() []string {
	var out []string
	switch e.Urgency {
	case Critical:
		out = append(out, "urgente")
	case Low:
		out = append(out, "baja")
	}
	if e.InlineReply {
		out = append(out, "responder")
	}
	if len(e.Actions) > 1 {
		out = append(out, "acciones")
	}
	if e.Progress >= 0 {
		out = append(out, "progreso")
	}
	if !e.Live {
		switch e.Reason {
		case "expired":
			out = append(out, "caducada")
		case "dismissed":
			out = append(out, "cerrada")
		case "closed":
			out = append(out, "cerrada")
		case "replaced":
			out = append(out, "sustituida")
		}
	}
	return out
}

// Title devuelve la línea principal de la entrada.
func (e Entry) Title() string {
	app := strings.TrimSpace(e.App)
	if app == "" {
		app = "sistema"
	}
	return app + " · " + Sanitize(e.Summary, 0)
}

// Center es el modelo del centro de notificaciones: mezcla lo que hay en
// pantalla con el historial, filtra y lleva la selección. No dibuja nada, así
// que se puede probar sin terminal.
type Center struct {
	entries  []Entry
	query    string
	selected int
	dnd      bool
}

// NewCenter construye el centro a partir de las notificaciones visibles y del
// historial. Las vivas van primero y, dentro de cada grupo, lo más reciente
// arriba.
func NewCenter(live []Notification, history []Record, dnd bool) *Center {
	c := &Center{dnd: dnd}
	// El historial llega de más antigua a más reciente.
	for i := len(history) - 1; i >= 0; i-- {
		r := history[i]
		// Si sigue viva, ya aparecerá arriba como viva.
		skip := false
		for _, n := range live {
			if n.ID == r.ID {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		c.entries = append(c.entries, Entry{
			Notification: r.Notification, Live: false,
			ClosedAt: r.ClosedAt, Reason: r.Reason,
		})
	}
	// Las vivas se insertan delante conservando el orden recibido.
	vivas := make([]Entry, 0, len(live))
	for i := len(live) - 1; i >= 0; i-- {
		vivas = append(vivas, Entry{Notification: live[i], Live: true})
	}
	c.entries = append(vivas, c.entries...)
	return c
}

// Len devuelve cuántas entradas hay en total (sin filtrar).
func (c *Center) Len() int { return len(c.entries) }

// DND indica si está activo el modo «no molestar».
func (c *Center) DND() bool { return c.dnd }

// SetDND cambia el estado mostrado de «no molestar».
func (c *Center) SetDND(on bool) { c.dnd = on }

// Query devuelve el filtro actual.
func (c *Center) Query() string { return c.query }

// SetQuery cambia el filtro y reinicia la selección.
func (c *Center) SetQuery(q string) {
	c.query = q
	c.selected = 0
}

// Append añade una letra al filtro.
func (c *Center) Append(r rune) { c.SetQuery(c.query + string(r)) }

// Backspace quita la última letra del filtro.
func (c *Center) Backspace() {
	if c.query == "" {
		return
	}
	r := []rune(c.query)
	c.SetQuery(string(r[:len(r)-1]))
}

// matches aplica el filtro: busca en la aplicación, el resumen y el cuerpo sin
// distinguir mayúsculas ni acentos.
func (c *Center) matches(e Entry) bool {
	if c.query == "" {
		return true
	}
	hay := normalize(e.App + " " + e.Summary + " " + StripMarkup(e.Body))
	for _, word := range strings.Fields(normalize(c.query)) {
		if !strings.Contains(hay, word) {
			return false
		}
	}
	return true
}

// normalize pasa a minúsculas y quita los acentos más comunes, para que
// «grabacion» encuentre «Grabación».
func normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'á', 'à', 'ä', 'â':
			r = 'a'
		case 'é', 'è', 'ë', 'ê':
			r = 'e'
		case 'í', 'ì', 'ï', 'î':
			r = 'i'
		case 'ó', 'ò', 'ö', 'ô':
			r = 'o'
		case 'ú', 'ù', 'ü', 'û':
			r = 'u'
		case 'ñ':
			r = 'n'
		case 'ç':
			r = 'c'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Filtered devuelve las entradas que pasan el filtro, en orden.
func (c *Center) Filtered() []Entry {
	if c.query == "" {
		return c.entries
	}
	out := make([]Entry, 0, len(c.entries))
	for _, e := range c.entries {
		if c.matches(e) {
			out = append(out, e)
		}
	}
	return out
}

// Selected devuelve la entrada seleccionada y si hay alguna.
func (c *Center) Selected() (Entry, bool) {
	f := c.Filtered()
	if len(f) == 0 {
		return Entry{}, false
	}
	c.clamp(len(f))
	return f[c.selected], true
}

// Index devuelve la posición seleccionada dentro de la lista filtrada.
func (c *Center) Index() int { return c.selected }

// Move desplaza la selección delta puestos, sin salirse de la lista.
func (c *Center) Move(delta int) {
	n := len(c.Filtered())
	if n == 0 {
		c.selected = 0
		return
	}
	c.selected += delta
	if c.selected < 0 {
		c.selected = 0
	}
	if c.selected >= n {
		c.selected = n - 1
	}
}

// Select fija la selección en una posición concreta.
func (c *Center) Select(i int) {
	n := len(c.Filtered())
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	c.selected = i
}

func (c *Center) clamp(n int) {
	if c.selected >= n {
		c.selected = n - 1
	}
	if c.selected < 0 {
		c.selected = 0
	}
}

// Counts devuelve cuántas vivas y cuántas del historial hay (sin filtrar).
func (c *Center) Counts() (live, history int) {
	for _, e := range c.entries {
		if e.Live {
			live++
		} else {
			history++
		}
	}
	return
}

// Summary es el resumen de una línea que se muestra en la cabecera.
func (c *Center) Summary() string {
	live, hist := c.Counts()
	parts := []string{plural(live, "en pantalla", "en pantalla")}
	if hist > 0 {
		parts = append(parts, plural(hist, "en el historial", "en el historial"))
	}
	if c.dnd {
		parts = append(parts, "no molestar")
	}
	if c.query != "" {
		parts = append(parts, plural(len(c.Filtered()), "resultado", "resultados"))
	}
	return strings.Join(parts, " · ")
}

// plural devuelve "1 cosa" o "N cosas".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
