package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Color es un color en formato "#rrggbb" o una cadena vacía para "sin color".
type Color string

// Profile indica la capacidad de color del terminal.
type Profile int

const (
	ProfileNone Profile = iota
	ProfileANSI256
	ProfileTrueColor
)

var profile = detectProfile()

func detectProfile() Profile {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return ProfileNone
	}
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if strings.Contains(ct, "truecolor") || strings.Contains(ct, "24bit") {
		return ProfileTrueColor
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "kitty") || strings.Contains(term, "alacritty") ||
		strings.Contains(term, "foot") || strings.Contains(term, "direct") {
		return ProfileTrueColor
	}
	if strings.Contains(term, "256color") {
		return ProfileANSI256
	}
	if os.Getenv("TERM") == "" {
		return ProfileNone
	}
	return ProfileANSI256
}

// SetProfile fuerza el perfil de color (útil para pruebas o flags).
func SetProfile(p Profile) { profile = p }

// GetProfile devuelve el perfil de color activo.
func GetProfile() Profile { return profile }

// hexToRGB convierte "#rrggbb" (o "#rgb") en componentes RGB.
func hexToRGB(c Color) (int, int, int, bool) {
	s := strings.TrimPrefix(string(c), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v>>16) & 0xff, int(v>>8) & 0xff, int(v) & 0xff, true
}

// xterm256Index aproxima un RGB al índice más cercano de la paleta xterm-256.
func xterm256Index(r, g, b int) int {
	to6 := func(v int) int {
		if v < 48 {
			return 0
		}
		if v < 114 {
			return 1
		}
		return (v - 35) / 40
	}
	ri, gi, bi := to6(r), to6(g), to6(b)
	cube := 16 + 36*ri + 6*gi + bi
	// Comparar con la rampa de grises.
	avg := (r + g + b) / 3
	gray := 232
	if avg > 238 {
		gray = 255
	} else if avg > 8 {
		gray = 232 + (avg-8)/10
		if gray > 255 {
			gray = 255
		}
	}
	// Distancia al cubo frente a la rampa de grises.
	cr, cg, cb := cubeRGB(ri, gi, bi)
	gr, gg, gb := grayRGB(gray)
	dc := sq(cr-r) + sq(cg-g) + sq(cb-b)
	dg := sq(gr-r) + sq(gg-g) + sq(gb-b)
	if dg < dc {
		return gray
	}
	return cube
}

func sq(v int) int { return v * v }

func cubeRGB(r, g, b int) (int, int, int) {
	f := func(v int) int {
		if v == 0 {
			return 0
		}
		return 55 + v*40
	}
	return f(r), f(g), f(b)
}

func grayRGB(i int) (int, int, int) {
	if i == 255 {
		return 238, 238, 238
	}
	v := 8 + (i-232)*10
	return v, v, v
}

// SGR devuelve la secuencia de escape para aplicar un color de primer plano.
func (c Color) SGR(bg bool) string {
	if c == "" || profile == ProfileNone {
		return ""
	}
	r, g, b, ok := hexToRGB(c)
	if !ok {
		return ""
	}
	base := 38
	if bg {
		base = 48
	}
	if profile == ProfileTrueColor {
		return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", base, r, g, b)
	}
	return fmt.Sprintf("\x1b[%d;5;%dm", base, xterm256Index(r, g, b))
}

// Style describe el formato de un texto.
type Style struct {
	Fg, Bg    Color
	Bold, Dim bool
	Italic    bool
	Underline bool
	Reverse   bool
	Strike    bool
}

// NewStyle crea un estilo con los colores indicados.
func NewStyle(fg, bg Color) Style { return Style{Fg: fg, Bg: bg} }

// WithFg devuelve una copia con otro color de primer plano.
func (s Style) WithFg(c Color) Style { s.Fg = c; return s }

// WithBg devuelve una copia con otro color de fondo.
func (s Style) WithBg(c Color) Style { s.Bg = c; return s }

// B devuelve una copia en negrita.
func (s Style) B() Style { s.Bold = true; return s }

// D devuelve una copia atenuada.
func (s Style) D() Style { s.Dim = true; return s }

// I devuelve una copia en cursiva.
func (s Style) I() Style { s.Italic = true; return s }

// U devuelve una copia subrayada.
func (s Style) U() Style { s.Underline = true; return s }

// R devuelve una copia invertida.
func (s Style) R() Style { s.Reverse = true; return s }

// Render aplica el estilo al texto.
func (s Style) Render(text string) string {
	if profile == ProfileNone {
		return text
	}
	var codes []string
	if s.Bold {
		codes = append(codes, "1")
	}
	if s.Dim {
		codes = append(codes, "2")
	}
	if s.Italic {
		codes = append(codes, "3")
	}
	if s.Underline {
		codes = append(codes, "4")
	}
	if s.Reverse {
		codes = append(codes, "7")
	}
	if s.Strike {
		codes = append(codes, "9")
	}
	if c := s.Fg.SGR(false); c != "" {
		codes = append(codes, strings.TrimSuffix(strings.TrimPrefix(c, "\x1b["), "m"))
	}
	if c := s.Bg.SGR(true); c != "" {
		codes = append(codes, strings.TrimSuffix(strings.TrimPrefix(c, "\x1b["), "m"))
	}
	if len(codes) == 0 {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + "\x1b[0m"
}

// Paint aplica color de primer plano y fondo a un texto.
func Paint(text string, fg, bg Color) string {
	return Style{Fg: fg, Bg: bg}.Render(text)
}

// Bold aplica negrita.
func Bold(text string) string { return Style{Bold: true}.Render(text) }

// Dim aplica atenuación.
func Dim(text string) string { return Style{Dim: true}.Render(text) }

// BorderStyle define el juego de caracteres de un borde.
type BorderStyle int

// Estilos de borde disponibles.
const (
	BorderNone BorderStyle = iota
	BorderRounded
	BorderSquare
	BorderDouble
	BorderThick
	BorderAscii
)

type borderChars struct {
	tl, tr, bl, br, h, v string
}

var borders = map[BorderStyle]borderChars{
	BorderRounded: {"╭", "╮", "╰", "╯", "─", "│"},
	BorderSquare:  {"┌", "┐", "└", "┘", "─", "│"},
	BorderDouble:  {"╔", "╗", "╚", "╝", "═", "║"},
	BorderThick:   {"┏", "┓", "┗", "┛", "━", "┃"},
	BorderAscii:   {"+", "+", "+", "+", "-", "|"},
}

// BoxBorder dibuja un marco alrededor de un bloque de texto ya renderizado.
// title, si no está vacío, se inserta en el borde superior.
func BoxBorder(body string, width int, style BorderStyle, fg, bg Color, title string) string {
	if style == BorderNone {
		return body
	}
	ch := borders[style]
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	paint := Style{Fg: fg, Bg: bg}
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")

	var b strings.Builder
	// Borde superior con título.
	top := ch.h
	titlePart := ""
	if title != "" {
		t := Truncate(title, inner-2)
		if t != "" {
			titlePart = ch.h + " " + t + " "
			pad := inner - Width(titlePart) - 1
			if pad < 0 {
				pad = 0
			}
			top = titlePart + strings.Repeat(ch.h, pad)
		}
	}
	if titlePart == "" {
		top = strings.Repeat(ch.h, inner)
	}
	b.WriteString(paint.Render(ch.tl + top + ch.tr))
	b.WriteString("\n")
	for _, ln := range lines {
		b.WriteString(paint.Render(ch.v))
		b.WriteString(PadRight(ln, inner))
		b.WriteString(paint.Render(ch.v))
		b.WriteString("\n")
	}
	b.WriteString(paint.Render(ch.bl + strings.Repeat(ch.h, inner) + ch.br))
	return b.String()
}

// HRule devuelve una línea horizontal de n caracteres.
func HRule(n int, ch string) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(ch, n)
}

// JoinHorizontal une bloques de texto lado a lado, alineados por su primera
// línea (rellenando con espacios el bloque más corto).
func JoinHorizontal(gap int, blocks ...string) string {
	if len(blocks) == 0 {
		return ""
	}
	split := make([][]string, len(blocks))
	widths := make([]int, len(blocks))
	height := 0
	for i, b := range blocks {
		split[i] = strings.Split(strings.TrimSuffix(b, "\n"), "\n")
		if len(split[i]) > height {
			height = len(split[i])
		}
		for _, ln := range split[i] {
			if w := Width(ln); w > widths[i] {
				widths[i] = w
			}
		}
	}
	var out strings.Builder
	for row := 0; row < height; row++ {
		for i := range blocks {
			cell := ""
			if row < len(split[i]) {
				cell = split[i][row]
			}
			out.WriteString(PadRight(cell, widths[i]))
			if i < len(blocks)-1 {
				out.WriteString(strings.Repeat(" ", gap))
			}
		}
		if row < height-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

// JoinVertical apila bloques de texto.
func JoinVertical(blocks ...string) string {
	return strings.Join(blocks, "\n")
}
