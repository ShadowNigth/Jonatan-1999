package notify

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// GroupKey devuelve la clave con la que se agrupa una notificación.
//
// Si la aplicación manda una etiqueta de pila (hint `x-dunst-stack-tag`,
// `x-kde-group-id` o `group`), se usa esa: así un cliente puede separar «chat
// de Marta» de «chat de Luis» dentro de la misma app. Si no, se agrupa por
// aplicación.
func GroupKey(n Notification) string {
	app := normalize(strings.TrimSpace(n.App))
	if app == "" {
		app = "sistema"
	}
	if g := strings.TrimSpace(n.GroupID); g != "" {
		return app + "|" + normalize(g)
	}
	return app
}

// Group es un conjunto de notificaciones de la misma aplicación (o de la misma
// pila declarada por el cliente), con la más reciente primero.
type Group struct {
	Key   string
	App   string
	Items []Notification
}

// Len devuelve cuántas notificaciones tiene el grupo.
func (g Group) Len() int { return len(g.Items) }

// Latest devuelve la notificación más reciente del grupo.
func (g Group) Latest() Notification {
	if len(g.Items) == 0 {
		return Notification{}
	}
	return g.Items[0]
}

// Urgency devuelve la urgencia más alta del grupo.
func (g Group) Urgency() Urgency {
	u := Low
	for _, n := range g.Items {
		if n.Urgency > u {
			u = n.Urgency
		}
	}
	return u
}

// IDs devuelve los identificadores de todas sus notificaciones.
func (g Group) IDs() []uint32 {
	out := make([]uint32, 0, len(g.Items))
	for _, n := range g.Items {
		out = append(out, n.ID)
	}
	return out
}

// AppName es el nombre legible de la aplicación.
func (g Group) AppName() string {
	for _, n := range g.Items {
		if a := strings.TrimSpace(n.App); a != "" {
			return a
		}
	}
	if a := strings.TrimSpace(g.App); a != "" {
		return a
	}
	return "sistema"
}

// Label describe el grupo: «Telegram» o «Telegram · 3 avisos».
func (g Group) Label() string {
	app := g.AppName()
	if len(g.Items) <= 1 {
		return app
	}
	return fmt.Sprintf("%s · %d avisos", app, len(g.Items))
}

// Preview es la línea que resume el grupo cuando está plegado: el resumen de
// la notificación más reciente y, si hay más, un «y N más».
func (g Group) Preview() string {
	latest := g.Latest()
	text := Sanitize(latest.Summary, 0)
	if text == "" {
		text = Sanitize(StripMarkup(latest.Body), 0)
	}
	if rest := len(g.Items) - 1; rest > 0 {
		if text == "" {
			return plural(rest, "aviso más", "avisos más")
		}
		return text + " · y " + plural(rest, "aviso más", "avisos más")
	}
	return text
}

// GroupAll agrupa las notificaciones recibidas. Los grupos se ordenan por la
// notificación más reciente (el grupo con algo más nuevo va al final, que es
// donde se apilan las tarjetas) y dentro de cada grupo la más reciente va
// primero.
func GroupAll(items []Notification) []Group {
	index := make(map[string]int, len(items))
	var groups []Group
	for i := len(items) - 1; i >= 0; i-- {
		n := items[i]
		key := GroupKey(n)
		if at, ok := index[key]; ok {
			groups[at].Items = append(groups[at].Items, n)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, Group{Key: key, App: n.App, Items: []Notification{n}})
	}
	// De más antiguo a más reciente, como el orden de llegada.
	for i, j := 0, len(groups)-1; i < j; i, j = i+1, j-1 {
		groups[i], groups[j] = groups[j], groups[i]
	}
	return groups
}

// Newest devuelve la notificación más reciente del grupo (la de mayor hora).
func (g Group) Newest() Notification {
	if len(g.Items) == 0 {
		return Notification{}
	}
	best := g.Items[0]
	for _, n := range g.Items[1:] {
		if n.Created.After(best.Created) {
			best = n
		}
	}
	return best
}

// Stack recuerda qué grupos están desplegados. Es estado de presentación: no
// toca el almacén, así que se puede probar sin dibujar nada.
type Stack struct {
	open map[string]bool
}

// NewStack crea un estado de plegado vacío.
func NewStack() *Stack { return &Stack{open: map[string]bool{}} }

// Expanded indica si el grupo está desplegado.
func (s *Stack) Expanded(key string) bool {
	if s == nil || s.open == nil {
		return false
	}
	return s.open[key]
}

// Toggle cambia el estado del grupo y devuelve el nuevo valor.
func (s *Stack) Toggle(key string) bool {
	if s.open == nil {
		s.open = map[string]bool{}
	}
	if s.open[key] {
		delete(s.open, key)
		return false
	}
	s.open[key] = true
	return true
}

// Expand despliega un grupo.
func (s *Stack) Expand(key string) {
	if s.open == nil {
		s.open = map[string]bool{}
	}
	s.open[key] = true
}

// Collapse pliega un grupo.
func (s *Stack) Collapse(key string) {
	if s.open != nil {
		delete(s.open, key)
	}
}

// CollapseAll pliega todos los grupos.
func (s *Stack) CollapseAll() {
	s.open = map[string]bool{}
}

// Prune olvida los grupos que ya no existen, para que el mapa no crezca sin
// fin durante una sesión larga.
func (s *Stack) Prune(alive []Group) {
	if s.open == nil {
		return
	}
	keys := make(map[string]bool, len(alive))
	for _, g := range alive {
		keys[g.Key] = true
	}
	for k := range s.open {
		if !keys[k] {
			delete(s.open, k)
		}
	}
}

// OpenKeys devuelve las claves desplegadas, ordenadas (pruebas y depuración).
func (s *Stack) OpenKeys() []string {
	out := make([]string, 0, len(s.open))
	for k := range s.open {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TrimToGroups recorta la lista para que queden como mucho max grupos
// distintos, conservando enteros los grupos más recientes. Se usa cuando la
// pila está plegada: cinco mensajes del mismo chat ocupan una tarjeta, no
// cinco.
func TrimToGroups(items []Notification, max int) []Notification {
	if max <= 0 || len(items) == 0 {
		return nil
	}
	seen := map[string]bool{}
	cut := 0
	for i := len(items) - 1; i >= 0; i-- {
		key := GroupKey(items[i])
		if !seen[key] {
			if len(seen) == max {
				cut = i + 1
				break
			}
			seen[key] = true
		}
	}
	return items[cut:]
}

// CountByApp cuenta cuántos avisos hay por aplicación, para el resumen.
func CountByApp(items []Notification) map[string]int {
	out := map[string]int{}
	for _, n := range items {
		out[GroupKey(n)]++
	}
	return out
}

// SortRecent ordena notificaciones de la más reciente a la más antigua.
func SortRecent(items []Notification) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Created.After(items[j].Created)
	})
}

// Age devuelve cuánto hace que llegó la notificación más reciente del grupo.
func (g Group) Age(now time.Time) time.Duration {
	if len(g.Items) == 0 {
		return 0
	}
	return now.Sub(g.Newest().Created)
}
