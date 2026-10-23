package ui

import (
	"strings"
)

// Item es una entrada seleccionable de un listado.
type Item struct {
	Key   string // valor devuelto al elegir
	Label string // texto principal
	Hint  string // texto secundario alineado a la derecha
	Desc  string // descripción en una línea extra
	Icon  string // glifo a la izquierda
	Group string // agrupación opcional
}

// Selector es un listado con filtro difuso, navegación y selección.
type Selector struct {
	Theme       *Theme
	Title       string
	Placeholder string
	Empty       string
	Items       []Item
	Filter      string
	Cursor      int
	Offset      int
	Multi       bool
	Chosen      map[string]bool
	ShowHints   bool
	// BypassFilter muestra los elementos tal cual, sin aplicar el filtro.
	// Se usa cuando la aplicación construye la lista de resultados por su
	// cuenta (por ejemplo, la calculadora del lanzador).
	BypassFilter bool
}

// NewSelector crea un selector con las entradas indicadas.
func NewSelector(theme *Theme, title string, items []Item) *Selector {
	return &Selector{
		Theme: theme, Title: title, Items: items,
		Placeholder: "buscar…", Empty: "sin resultados",
		Chosen: map[string]bool{}, ShowHints: true,
	}
}

// Filtered devuelve las entradas que coinciden con el filtro, ya ordenadas.
func (s *Selector) Filtered() []Item {
	if s.BypassFilter || strings.TrimSpace(s.Filter) == "" {
		return s.Items
	}
	idx := FilterItems(s.Items, s.Filter, func(it Item) []string {
		return []string{it.Label, it.Hint, it.Group}
	})
	out := make([]Item, 0, len(idx))
	for _, i := range idx {
		out = append(out, s.Items[i])
	}
	return out
}

// Selected devuelve la entrada bajo el cursor.
func (s *Selector) Selected() (Item, bool) {
	list := s.Filtered()
	if len(list) == 0 {
		return Item{}, false
	}
	if s.Cursor >= len(list) {
		s.Cursor = len(list) - 1
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	return list[s.Cursor], true
}

// Move desplaza el cursor.
func (s *Selector) Move(delta int) {
	n := len(s.Filtered())
	if n == 0 {
		s.Cursor = 0
		return
	}
	s.Cursor += delta
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	if s.Cursor >= n {
		s.Cursor = n - 1
	}
}

// Type añade texto al filtro.
func (s *Selector) Type(r rune) {
	s.Filter += string(r)
	s.Cursor = 0
	s.Offset = 0
}

// Backspace borra el último carácter del filtro.
func (s *Selector) Backspace() {
	if s.Filter == "" {
		return
	}
	r := []rune(s.Filter)
	s.Filter = string(r[:len(r)-1])
	s.Cursor = 0
	s.Offset = 0
}

// ClearFilter vacía el filtro.
func (s *Selector) ClearFilter() {
	s.Filter = ""
	s.Cursor = 0
	s.Offset = 0
}

// ensureVisible ajusta el desplazamiento para que el cursor quede a la vista.
func (s *Selector) ensureVisible(rows int) {
	if rows <= 0 {
		return
	}
	if s.Cursor < s.Offset {
		s.Offset = s.Cursor
	}
	if s.Cursor >= s.Offset+rows {
		s.Offset = s.Cursor - rows + 1
	}
	if s.Offset < 0 {
		s.Offset = 0
	}
}

// ToggleMarks marca o desmarca la entrada seleccionada (modo múltiple).
func (s *Selector) ToggleMarks() {
	it, ok := s.Selected()
	if !ok {
		return
	}
	if s.Chosen[it.Key] {
		delete(s.Chosen, it.Key)
	} else {
		s.Chosen[it.Key] = true
	}
	s.Move(1)
}

// MarkedKeys devuelve las claves marcadas en orden de la lista.
func (s *Selector) MarkedKeys() []string {
	var out []string
	for _, it := range s.Items {
		if s.Chosen[it.Key] {
			out = append(out, it.Key)
		}
	}
	return out
}

// View dibuja el selector completo en un área de w×h celdas.
func (s *Selector) View(w, h int) string {
	t := s.Theme
	rows := h - 6 // título, filtro, separador, pie, dos bordes
	if rows < 1 {
		rows = 1
	}
	list := s.Filtered()
	s.ensureVisible(rows)

	innerW := w - 2
	body := make([]string, 0, h)

	// Línea de filtro.
	prompt := Style{Fg: t.Palette.Accent, Bg: t.Palette.BgAlt}.B().Render(" " + t.Icons.Search + " ")
	filterText := s.Filter
	if filterText == "" {
		filterText = Style{Fg: t.Palette.Muted, Bg: t.Palette.BgAlt}.Render(s.Placeholder)
	} else {
		filterText = Style{Fg: t.Palette.Fg, Bg: t.Palette.BgAlt}.Render(filterText)
	}
	cursor := Style{Fg: t.Palette.Accent, Bg: t.Palette.BgAlt}.Render("▏")
	counter := ""
	if n := len(list); n > 0 && s.Filter != "" {
		counter = Style{Fg: t.Palette.Muted, Bg: t.Palette.BgAlt}.Render(itoa(n) + " ")
	}
	fill := innerW - Width(prompt) - Width(filterText) - 1 - Width(counter)
	if fill < 0 {
		fill = 0
	}
	body = append(body, prompt+filterText+cursor+Style{Bg: t.Palette.BgAlt}.Render(strings.Repeat(" ", fill))+counter)

	// Separador.
	body = append(body, Style{Fg: t.Palette.Border, Bg: t.Palette.Bg}.Render(strings.Repeat("─", innerW)))

	// Listado.
	if len(list) == 0 {
		body = append(body, Style{Fg: t.Palette.Muted, Bg: t.Palette.Bg}.Render("  "+s.Empty))
		for len(body) < rows+2 {
			body = append(body, "")
		}
	}
	for i := 0; i < rows && s.Offset+i < len(list); i++ {
		it := list[s.Offset+i]
		selected := s.Offset+i == s.Cursor
		body = append(body, s.renderItem(it, selected, innerW))
	}
	for len(body) < rows+2 && len(list) > 0 {
		body = append(body, "")
	}

	// Relleno hasta el borde inferior.
	for len(body) < h-2 {
		body = append(body, "")
	}

	footer := ""
	if s.ShowHints {
		footer = t.HintBar(
			[2]string{"↵", "elegir"},
			[2]string{"↑↓", "mover"},
			[2]string{"esc", "cerrar"},
		)
	}
	title := s.Title
	if s.Multi {
		title += Style{Fg: t.Palette.Accent2, Bg: t.Palette.Bg}.Render("  [" + itoa(len(s.Chosen)) + "]")
	}
	return t.Frame(w, h, title, footer, body)
}

func (s *Selector) renderItem(it Item, selected bool, w int) string {
	t := s.Theme
	bg := t.Palette.Bg
	if selected {
		bg = t.Palette.BgSel
	}
	mark := "  "
	if s.Multi {
		if s.Chosen[it.Key] {
			mark = Style{Fg: t.Palette.Ok, Bg: bg}.Render(t.Icons.Check + " ")
		} else {
			mark = Style{Fg: t.Palette.Muted, Bg: bg}.Render("· ")
		}
	}
	it.Label = Sanitize(it.Label)
	it.Hint = Sanitize(it.Hint)
	icon := it.Icon
	if icon == "" {
		icon = t.Icons.Dot
	}
	iconStyle := Style{Fg: t.Palette.Muted, Bg: bg}
	labelStyle := Style{Fg: t.Palette.Fg, Bg: bg}
	hintStyle := Style{Fg: t.Palette.Muted, Bg: bg}
	if selected {
		iconStyle = Style{Fg: t.Palette.Accent, Bg: bg}.B()
		labelStyle = Style{Fg: t.Palette.Fg, Bg: bg}.B()
		hintStyle = Style{Fg: t.Palette.FgDim, Bg: bg}
	}

	hintW := 0
	hint := ""
	if it.Hint != "" {
		hint = hintStyle.Render(Truncate(it.Hint, 20))
		hintW = Width(hint) + 1
	}
	prefix := mark + iconStyle.Render(icon) + " "
	avail := w - 2 - Width(prefix) - hintW
	if avail < 4 {
		avail = 4
	}
	label := Truncate(it.Label, avail)
	label = labelStyle.Render(label)
	pad := w - 2 - Width(prefix) - Width(label) - hintW
	if pad < 0 {
		pad = 0
	}
	line := " " + prefix + label + Style{Bg: bg}.Render(strings.Repeat(" ", pad)) + hint + Style{Bg: bg}.Render(" ")
	if selected {
		// Marca lateral de selección.
		line = Style{Fg: t.Palette.Accent, Bg: bg}.Render("▌") + line[1:]
	}
	return line
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
