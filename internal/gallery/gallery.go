// Package gallery contiene las galerías interactivas de temas y fondos de
// pantalla, con previsualización animada en vivo.
package gallery

import (
	"fmt"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// Entry es un elemento seleccionable de una galería.
type Entry struct {
	Key   string
	Title string
	Sub   string
	Tag   string
	Icon  string
	Pal   ui.Palette
	Spec  wallpaper.Spec
}

// listPane dibuja una lista con cursor, desplazamiento y agrupación por etiqueta.
type listPane struct {
	entries  []Entry
	cursor   int
	offset   int
	selected func(Entry) bool
}

func (l *listPane) move(delta int) {
	if len(l.entries) == 0 {
		l.cursor = 0
		return
	}
	l.cursor += delta
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.cursor >= len(l.entries) {
		l.cursor = len(l.entries) - 1
	}
}

func (l *listPane) current() (Entry, bool) {
	if len(l.entries) == 0 {
		return Entry{}, false
	}
	if l.cursor >= len(l.entries) {
		l.cursor = len(l.entries) - 1
	}
	return l.entries[l.cursor], true
}

// render dibuja la lista en w×h celdas.
func (l *listPane) render(t *ui.Theme, w, h int, title string) []string {
	p := t.Palette
	out := make([]string, 0, h)
	if title != "" {
		out = append(out, t.Accent().Render(" "+title))
		h--
	}
	rows := h
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+rows {
		l.offset = l.cursor - rows + 1
	}
	if l.offset < 0 {
		l.offset = 0
	}
	lastTag := ""
	for i := 0; i < rows; i++ {
		idx := l.offset + i
		if idx >= len(l.entries) {
			out = append(out, strings.Repeat(" ", w))
			continue
		}
		e := l.entries[idx]
		if e.Tag != "" && e.Tag != lastTag {
			lastTag = e.Tag
			if i > 0 {
				out = append(out, t.Muted().Render(" "+strings.ToUpper(lastTag)))
				i++
				if i >= rows {
					break
				}
			}
		}
		selected := idx == l.cursor
		bg := p.Bg
		if selected {
			bg = p.BgSel
		}
		marker := "  "
		if selected {
			marker = ui.Style{Fg: p.Accent, Bg: bg}.Render("▌ ")
		}
		icon := e.Icon
		if icon == "" {
			icon = t.Icons.Theme
		}
		iconStr := ui.Style{Fg: p.Accent2, Bg: bg}.Render(icon + " ")
		titleStyle := ui.Style{Fg: p.Fg, Bg: bg}
		if selected {
			titleStyle = titleStyle.B()
		}
		subW := 0
		sub := ""
		if e.Sub != "" {
			sub = ui.Style{Fg: p.Muted, Bg: bg}.Render(ui.Truncate(e.Sub, 12))
			subW = ui.Width(sub) + 1
		}
		avail := w - 3 - ui.Width(iconStr) - subW
		if avail < 4 {
			avail = 4
		}
		label := titleStyle.Render(ui.Truncate(e.Title, avail))
		pad := w - 2 - ui.Width(marker) - ui.Width(iconStr) - ui.Width(label) - subW
		if pad < 0 {
			pad = 0
		}
		out = append(out, marker+iconStr+label+ui.Style{Bg: bg}.Render(strings.Repeat(" ", pad))+sub+" ")
	}
	for len(out) < h+1 {
		out = append(out, "")
	}
	return out[:max(h+1, 0)]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// joinColumns compone dos bloques de texto lado a lado con una separación.
func joinColumns(left []string, leftW int, right []string, gap int, sepFg ui.Color, bg ui.Color) []string {
	h := max(len(left), len(right))
	out := make([]string, 0, h)
	sep := ui.Style{Fg: sepFg, Bg: bg}.Render("│")
	for i := 0; i < h; i++ {
		l := ""
		if i < len(left) {
			l = left[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}
		out = append(out, ui.PadRight(l, leftW)+strings.Repeat(" ", gap)+sep+strings.Repeat(" ", gap)+r)
	}
	return out
}

// badge dibuja una etiqueta pequeña con el color indicado.
func badge(t *ui.Theme, text string, fg, bg ui.Color) string {
	return ui.Style{Fg: fg, Bg: bg}.B().Render(" " + text + " ")
}

// header dibuja el encabezado con degradado y pestañas opcionales.
func header(t *ui.Theme, w int, title string, tabs []string, active int) []string {
	p := t.Palette
	out := make([]string, 0, 2)
	left := " " + t.Icons.Star + " "
	titleStr := t.GradientText(title, p.Accent, p.Accent2)
	line := ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render(left) + titleStr
	if len(tabs) > 0 {
		var parts []string
		for i, tab := range tabs {
			if i == active {
				parts = append(parts, ui.Style{Fg: p.Bg, Bg: p.Accent}.B().Render(" "+tab+" "))
			} else {
				parts = append(parts, ui.Style{Fg: p.Muted, Bg: p.Bg}.Render(" "+tab+" "))
			}
		}
		tabsStr := strings.Join(parts, ui.Style{Fg: p.Border, Bg: p.Bg}.Render("·"))
		gap := w - ui.Width(line) - ui.Width(tabsStr) - 2
		if gap < 1 {
			gap = 1
		}
		line += strings.Repeat(" ", gap) + tabsStr
	}
	out = append(out, line)
	out = append(out, t.Muted().Render(strings.Repeat("─", w)))
	return out
}

// searchBar dibuja la barra de filtro.
func searchBar(t *ui.Theme, w int, filter, placeholder, icon string) string {
	p := t.Palette
	bg := p.BgAlt
	prefix := ui.Style{Fg: p.Accent, Bg: bg}.B().Render(" " + icon + " ")
	var body string
	if filter == "" {
		body = ui.Style{Fg: p.Muted, Bg: bg}.Render(placeholder)
	} else {
		body = ui.Style{Fg: p.Fg, Bg: bg}.Render(filter)
	}
	cur := ui.Style{Fg: p.Accent, Bg: bg}.Render("▏")
	line := prefix + body + cur
	return ui.PadRight(line, w)
}

// footerBar compone las pistas de teclado.
func footerBar(t *ui.Theme, hints ...[2]string) string {
	return t.HintBar(hints...)
}

// metaLine dibuja "clave   valor".
func metaLine(t *ui.Theme, key, value string, w int) string {
	p := t.Palette
	keyW := 13
	return " " + ui.Style{Fg: p.Muted, Bg: p.Bg}.Render(ui.PadRight(key, keyW)) +
		ui.Style{Fg: p.Fg, Bg: p.Bg}.Render(ui.Truncate(value, max(4, w-keyW-2)))
}

// formatInt formatea un entero con separador de miles (para tamaños y números).
func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ".")
	if neg {
		out = "-" + out
	}
	return out
}
