package notify

import (
	"fmt"
	"sort"
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
	if e.HasImage() || e.ImagePath != "" {
		out = append(out, "imagen")
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
	grouped  bool
	open     map[string]bool
}

// NewCenter construye el centro a partir de las notificaciones visibles y del
// historial. Las vivas van primero y, dentro de cada grupo, lo más reciente
// arriba.
func NewCenter(live []Notification, history []Record, dnd bool) *Center {
	// Por defecto se agrupa por aplicación, igual que el daemon: tres mensajes
	// del mismo chat ocupan una fila, no tres.
	c := &Center{dnd: dnd, grouped: true, open: map[string]bool{}}
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

// Row es una línea del centro: una notificación suelta, un grupo plegado o la
// franja de un grupo desplegado.
type Row struct {
	Key      string
	Group    bool // representa a varias notificaciones
	Header   bool // franja de un grupo desplegado
	Count    int
	Expanded bool
	Entry    Entry   // la más reciente del grupo (o la única)
	Entries  []Entry // todas las del grupo
}

// Title es la línea principal de la fila.
func (r Row) Title() string {
	if r.Header {
		return fmt.Sprintf("%s · %d avisos", appOf(r.Entry), r.Count)
	}
	if r.Group && !r.Expanded {
		return fmt.Sprintf("%s · %d avisos", appOf(r.Entry), r.Count)
	}
	return r.Entry.Title()
}

// Badges devuelve las insignias de la fila. En una pila el recuento ya va en
// el título, así que solo se marcan las urgencias.
func (r Row) Badges() []string {
	if r.Group || r.Header {
		var out []string
		if u := newestUrgency(r.Entries); u == Critical {
			out = append(out, "urgente")
		}
		if r.Header {
			out = append(out, "abierto")
		}
		return out
	}
	return r.Entry.Badges()
}

// Newest devuelve la entrada más reciente de la fila.
func (r Row) Newest() Entry {
	if len(r.Entries) == 0 {
		return r.Entry
	}
	best := r.Entries[0]
	for _, e := range r.Entries[1:] {
		if e.Created.After(best.Created) {
			best = e
		}
	}
	return best
}

// IDs devuelve las notificaciones que representa la fila.
func (r Row) IDs() []uint32 {
	if len(r.Entries) == 0 {
		return []uint32{r.Entry.ID}
	}
	out := make([]uint32, 0, len(r.Entries))
	for _, e := range r.Entries {
		out = append(out, e.ID)
	}
	return out
}

func appOf(e Entry) string {
	if a := strings.TrimSpace(e.App); a != "" {
		return a
	}
	return "sistema"
}

func newestUrgency(entries []Entry) Urgency {
	u := Low
	for _, e := range entries {
		if e.Urgency > u {
			u = e.Urgency
		}
	}
	return u
}

// SetGrouped activa o desactiva las pilas por aplicación.
func (c *Center) SetGrouped(on bool) {
	if c.grouped == on {
		return
	}
	c.grouped = on
	c.selected = 0
	if !on && c.open != nil {
		c.open = map[string]bool{}
	}
}

// Grouped indica si las notificaciones se agrupan por aplicación.
func (c *Center) Grouped() bool { return c.grouped }

// ToggleGrouped cambia el modo y devuelve el nuevo valor.
func (c *Center) ToggleGrouped() bool {
	c.SetGrouped(!c.grouped)
	if c.open == nil {
		c.open = map[string]bool{}
	}
	if !c.grouped {
		c.open = map[string]bool{}
	}
	return c.grouped
}

// Expanded indica si el grupo está desplegado.
func (c *Center) Expanded(key string) bool {
	if c.open == nil {
		return false
	}
	return c.open[key]
}

// Expand despliega un grupo por su clave.
func (c *Center) Expand(key string) {
	if c.open == nil {
		c.open = map[string]bool{}
	}
	c.open[key] = true
}

// Collapse pliega un grupo.
func (c *Center) Collapse(key string) {
	if c.open != nil {
		delete(c.open, key)
	}
}

// ToggleGroup despliega o pliega un grupo y devuelve el nuevo estado.
func (c *Center) ToggleGroup(key string) bool {
	if c.open == nil {
		c.open = map[string]bool{}
	}
	if c.open[key] {
		delete(c.open, key)
		return false
	}
	c.open[key] = true
	return true
}

// OpenKeys devuelve las claves de los grupos desplegados, ordenadas.
func (c *Center) OpenKeys() []string {
	out := make([]string, 0, len(c.open))
	for k := range c.open {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// GroupCount devuelve cuántos grupos distintos hay entre las filas (sin
// filtrar), útil para el resumen.
func (c *Center) GroupCount() int {
	keys := map[string]bool{}
	for _, e := range c.entries {
		keys[GroupKey(e.Notification)] = true
	}
	return len(keys)
}

// Rows devuelve las filas visibles: una por notificación o una por grupo,
// según el modo y qué grupos estén desplegados.
func (c *Center) Rows() []Row {
	entries := c.Filtered()
	if !c.grouped {
		out := make([]Row, 0, len(entries))
		for _, e := range entries {
			out = append(out, Row{Key: GroupKey(e.Notification), Count: 1,
				Entry: e, Entries: []Entry{e}})
		}
		return out
	}
	var order []string
	groups := map[string][]Entry{}
	for _, e := range entries {
		key := GroupKey(e.Notification)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], e)
	}
	var out []Row
	for _, key := range order {
		items := groups[key]
		if len(items) == 1 {
			out = append(out, Row{Key: key, Count: 1, Entry: items[0], Entries: items})
			continue
		}
		newest := items[0]
		for _, e := range items[1:] {
			if e.Created.After(newest.Created) {
				newest = e
			}
		}
		if !c.Expanded(key) {
			out = append(out, Row{Key: key, Group: true, Count: len(items),
				Entry: newest, Entries: items})
			continue
		}
		out = append(out, Row{Key: key, Group: true, Header: true, Count: len(items),
			Expanded: true, Entry: newest, Entries: items})
		for _, e := range items {
			out = append(out, Row{Key: key, Count: len(items), Entry: e, Entries: []Entry{e}})
		}
	}
	return out
}

// SelectedRow devuelve la fila seleccionada y si hay alguna.
func (c *Center) SelectedRow() (Row, bool) {
	rows := c.Rows()
	if len(rows) == 0 {
		return Row{}, false
	}
	c.clamp(len(rows))
	return rows[c.selected], true
}

// ToggleSelected despliega o pliega el grupo de la fila seleccionada. Devuelve
// la fila y si la tecla hizo algo (las notificaciones sueltas no se pliegan).
func (c *Center) ToggleSelected() (Row, bool) {
	row, ok := c.SelectedRow()
	if !ok || !row.Group {
		return row, false
	}
	c.ToggleGroup(row.Key)
	return row, true
}

// SelectedIDs devuelve las notificaciones que representa la fila seleccionada.
func (c *Center) SelectedIDs() []uint32 {
	row, ok := c.SelectedRow()
	if !ok {
		return nil
	}
	return row.IDs()
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

// Index devuelve la posición seleccionada dentro de la lista de filas.
func (c *Center) Index() int { return c.selected }

// Move desplaza la selección delta puestos, sin salirse de la lista.
func (c *Center) Move(delta int) {
	n := len(c.Rows())
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
	n := len(c.Rows())
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
