package ui

import "strings"

// Frame dibuja un panel con marco, título, contenido y pie.
//
//	╭─ título ──────────────────────────╮
//	│ contenido                          │
//	╰─ pie ─────────────────────────────╯
func (t *Theme) Frame(w, h int, title, footer string, body []string) string {
	if w < 4 || h < 3 {
		return strings.Join(body, "\n")
	}
	tc := borders[t.Border]
	paint := Style{Fg: t.Palette.Border, Bg: t.Palette.Bg}
	innerW := w - 2
	innerH := h - 2

	lines := make([]string, 0, innerH)
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		lines = append(lines, line)
	}

	var b strings.Builder
	// Borde superior con título a la izquierda.
	top := tc.h + tc.h
	topTitle := ""
	if title != "" {
		topTitle = " " + Truncate(title, innerW-6) + " "
		top += topTitle + strings.Repeat(tc.h, max(0, innerW-2-Width(topTitle)))
	} else {
		top += strings.Repeat(tc.h, max(0, innerW-2))
	}
	b.WriteString(paint.Render(tc.tl + top + tc.tr))
	b.WriteString("\n")

	for _, ln := range lines {
		b.WriteString(paint.Render(tc.v))
		b.WriteString(PadRight(ln, innerW))
		b.WriteString(paint.Render(tc.v))
		b.WriteString("\n")
	}

	// Borde inferior con pie a la derecha.
	bot := ""
	if footer != "" {
		f := Truncate(footer, innerW-4)
		bot = strings.Repeat(tc.h, max(0, innerW-2-Width(f))) + " " + f + " "
	} else {
		bot = strings.Repeat(tc.h, innerW)
	}
	b.WriteString(paint.Render(tc.bl + bot + tc.br))
	return b.String()
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

// KeyHint dibuja una pista de teclado con la tecla resaltada.
func (t *Theme) KeyHint(key, desc string) string {
	k := Style{Fg: t.Palette.Accent, Bg: t.Palette.Bg}.B().Render(key)
	d := Style{Fg: t.Palette.Muted, Bg: t.Palette.Bg}.Render(desc)
	return k + " " + d
}

// HintBar une varias pistas con separadores.
func (t *Theme) HintBar(hints ...[2]string) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, t.KeyHint(h[0], h[1]))
	}
	return strings.Join(parts, Style{Fg: t.Palette.Border}.Render(" · "))
}

// Badge dibuja una etiqueta compacta con color de acento.
func (t *Theme) Badge(text string, fg, bg Color) string {
	return Style{Fg: fg, Bg: bg}.B().Render(" " + text + " ")
}

// Scroll indicador vertical: devuelve un bloque de líneas del tamaño dado con
// el pulgador en la posición proporcional.
func Scroll(h, total, visible, offset int) []string {
	out := make([]string, h)
	for i := range out {
		out[i] = " "
	}
	if total <= visible || h <= 0 {
		return out
	}
	thumb := h * visible / total
	if thumb < 1 {
		thumb = 1
	}
	if thumb > h {
		thumb = h
	}
	start := 0
	if total-visible > 0 {
		start = (h - thumb) * offset / (total - visible)
	}
	for i := start; i < start+thumb && i < h; i++ {
		out[i] = "▐"
	}
	return out
}
