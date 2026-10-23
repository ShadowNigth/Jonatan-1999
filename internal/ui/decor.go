package ui

import (
	"fmt"
	"math"
	"strings"
)

// LerpColor interpola entre dos colores en espacio RGB.
func LerpColor(a, b Color, t float64) Color {
	ar, ag, ab, ok1 := hexToRGB(a)
	br, bg, bb, ok2 := hexToRGB(b)
	if !ok1 || !ok2 {
		return a
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	mix := func(x, y int) int { return int(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return Color(fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb)))
}

// GradientText colorea cada carácter interpolando de a a b.
//
// El degradado se calcula sobre celdas visibles, no sobre bytes.
func (t *Theme) GradientText(s string, a, b Color) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}
	var out strings.Builder
	for i, r := range runes {
		f := 0.0
		if len(runes) > 1 {
			f = float64(i) / float64(len(runes)-1)
		}
		out.WriteString(Style{Fg: LerpColor(a, b, f), Bg: t.Palette.Bg}.Render(string(r)))
	}
	return out.String()
}

// GradientBar dibuja una barra horizontal con degradado.
func (t *Theme) GradientBar(w int, a, b Color) string {
	if w <= 0 {
		return ""
	}
	var out strings.Builder
	for i := 0; i < w; i++ {
		f := float64(i) / float64(max(1, w-1))
		out.WriteString(Style{Fg: LerpColor(a, b, f), Bg: t.Palette.Bg}.Render("█"))
	}
	return out.String()
}

// GradientBlock pinta un bloque de w×h con un degradado diagonal.
func (t *Theme) GradientBlock(w, h int, a, b Color) []string {
	lines := make([]string, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			f := (float64(x)/float64(max(1, w-1))*0.6 + float64(y)/float64(max(1, h-1))*0.4)
			sb.WriteString(Style{Fg: LerpColor(a, b, f)}.Render("▄"))
		}
		lines[y] = sb.String()
	}
	return lines
}

// Swatches devuelve una fila de muestras de color de 2 celdas cada una.
func (t *Theme) Swatches(colors []Color, cell int) string {
	var sb strings.Builder
	for _, c := range colors {
		sb.WriteString(Style{Fg: c, Bg: c}.Render(strings.Repeat(" ", cell)))
	}
	return sb.String()
}

// MockWindow dibuja una ventana de ejemplo con barra de título y contenido, para
// previsualizar cómo se verá un tema en el escritorio real.
func (t *Theme) MockWindow(w, h int, title, appID string) []string {
	if w < 8 || h < 3 {
		return nil
	}
	p := t.Palette
	out := make([]string, 0, h)
	// Barra de título con los "botones" del cliente.
	btn := Style{Fg: p.Muted, Bg: p.BgSel}.Render(" ● ○ ○ ")
	titleStyle := Style{Fg: p.Fg, Bg: p.BgSel}.B()
	pad := w - Width(btn) - Width(title) - 4
	if pad < 0 {
		pad = 0
	}
	out = append(out, Style{Bg: p.BgSel}.Render(" ")+btn+titleStyle.Render(" "+Truncate(title, max(4, pad)))+
		Style{Bg: p.BgSel}.Render(strings.Repeat(" ", max(0, pad-Width(title)-1)+2)))
	// Área de contenido con un esqueleto de interfaz.
	lines := []string{
		Style{Fg: p.Accent, Bg: p.Bg}.B().Render("  " + appID),
		Style{Fg: p.Muted, Bg: p.Bg}.Render("  " + strings.Repeat("─", max(1, w-6))),
		Style{Fg: p.Fg, Bg: p.Bg}.Render("  Usa " + Style{Fg: p.Accent, Bg: p.Bg}.B().Render("Super+d") + " para abrir el lanzador"),
		Style{Fg: p.FgDim, Bg: p.Bg}.Render("  " + t.GradientText("● ● ● ● ● ● ● ●", p.Accent, p.Accent2)),
		Style{Fg: p.Ok, Bg: p.Bg}.Render("  ✔ sistema") + Style{Fg: p.Warn, Bg: p.Bg}.Render("  ▲ aviso") +
			Style{Fg: p.Err, Bg: p.Bg}.Render("  ✘ error"),
	}
	for _, l := range lines {
		if len(out) >= h-1 {
			break
		}
		out = append(out, PadRight(l, w))
	}
	for len(out) < h-1 {
		out = append(out, Style{Bg: p.Bg}.Render(strings.Repeat(" ", w)))
	}
	// Barra inferior.
	footer := Style{Fg: p.FgDim, Bg: p.BgAlt}.Render(" " + t.Icons.Workspace + " 1  2  3  " +
		strings.Repeat(" ", max(0, w-Width(" 1  2  3  ")-14)) + Style{Fg: p.Accent, Bg: p.BgAlt}.Render("100% ▮") + " ")
	out = append(out, PadRight(footer, w))
	return out
}

// Box dibuja un marco simple alrededor de líneas ya renderizadas.
func (t *Theme) Box(lines []string, w int, title string, fg Color) []string {
	tc := borders[t.Border]
	paint := Style{Fg: fg, Bg: t.Palette.Bg}
	inner := w - 2
	out := make([]string, 0, len(lines)+2)
	top := ""
	if title != "" {
		top = " " + Truncate(title, inner-4) + " "
		top += strings.Repeat(tc.h, max(0, inner-Width(top)-1))
	} else {
		top = strings.Repeat(tc.h, inner)
	}
	out = append(out, paint.Render(tc.tl+top+tc.tr))
	for _, l := range lines {
		out = append(out, paint.Render(tc.v)+PadRight(l, inner)+paint.Render(tc.v))
	}
	out = append(out, paint.Render(tc.bl+strings.Repeat(tc.h, inner)+tc.br))
	return out
}

// Centered coloca líneas dentro de un área de w×h, centradas horizontal y
// verticalmente.
func Centered(lines []string, w, h int) []string {
	out := make([]string, 0, h)
	top := (h - len(lines)) / 2
	for i := 0; i < top; i++ {
		out = append(out, strings.Repeat(" ", w))
	}
	for _, l := range lines {
		out = append(out, Center(l, w))
	}
	for len(out) < h {
		out = append(out, strings.Repeat(" ", w))
	}
	return out
}
