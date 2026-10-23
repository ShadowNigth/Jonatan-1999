package ui

import (
	"strings"
	"time"
	"unicode/utf8"
)

// stripANSI elimina las secuencias de escape ANSI de una cadena.
func stripANSI(s string) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) {
			j := i + 1
			switch s[j] {
			case '[': // CSI: ESC [ ... byte final en 0x40-0x7e
				j++
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				if j < len(s) {
					j++
				}
				i = j
				continue
			case ']': // OSC: ESC ] ... BEL o ST
				j++
				for j < len(s) {
					if s[j] == 0x07 {
						j++
						break
					}
					if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			default:
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// runeWidth devuelve el número de celdas que ocupa una runa en el terminal.
// Es una implementación reducida de wcwidth: 0 para marcas combinantes y
// caracteres de control, 2 para ideogramas y emojis anchos, 1 para el resto.
func runeWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 32 || (r >= 0x7f && r < 0xa0):
		return 0
	case r >= 0x0300 && r <= 0x036f, // marcas combinantes
		r >= 0x0483 && r <= 0x0489,
		r >= 0x0591 && r <= 0x05bd,
		r >= 0x0610 && r <= 0x061a,
		r >= 0x064b && r <= 0x065f,
		r >= 0x0e31 && r <= 0x0e3a,
		r >= 0x1ab0 && r <= 0x1aff,
		r >= 0x1dc0 && r <= 0x1dff,
		r >= 0x20d0 && r <= 0x20f0,
		r >= 0xfe00 && r <= 0xfe0f, // selectores de variación
		r >= 0xfe20 && r <= 0xfe2f,
		r >= 0xfeff && r <= 0xfeff,
		r >= 0xe0100 && r <= 0xe01ef:
		return 0
	case r >= 0x1100 && r <= 0x115f, // Hangul Jamo inicial
		r >= 0x2e80 && r <= 0x303e, // CJK radicals y puntuación
		r >= 0x3041 && r <= 0x33ff,
		r >= 0x3400 && r <= 0x4dbf,
		r >= 0x4e00 && r <= 0x9fff, // CJK unificado
		r >= 0xa000 && r <= 0xa4cf,
		r >= 0xa960 && r <= 0xa97f,
		r >= 0xac00 && r <= 0xd7a3, // silabas Hangul
		r >= 0xf900 && r <= 0xfaff,
		r >= 0xfe10 && r <= 0xfe19,
		r >= 0xfe30 && r <= 0xfe6f,
		r >= 0xff00 && r <= 0xff60, // formas anchas
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x1f300 && r <= 0x1f64f, // emojis
		r >= 0x1f900 && r <= 0x1f9ff,
		r >= 0x1f680 && r <= 0x1f6ff,
		r >= 0x1fa70 && r <= 0x1faff,
		r >= 0x20000 && r <= 0x3fffd:
		return 2
	}
	return 1
}

// Width devuelve el ancho visible en celdas de una cadena, ignorando escapes
// ANSI y contabilizando caracteres anchos.
func Width(s string) int {
	w := 0
	for _, r := range stripANSI(s) {
		w += runeWidth(r)
	}
	return w
}

// Truncate recorta la cadena (sin escapes ANSI) a w celdas, añadiendo "…" si
// se recortó algo. Es segura con texto UTF-8.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if Width(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := runeWidth(r)
		if used+rw > w-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	b.WriteString("…")
	return b.String()
}

// TruncateANSI recorta respetando las secuencias ANSI presentes.
func TruncateANSI(s string, w int) string {
	if Width(s) <= w {
		return s
	}
	if w <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				if j < len(s) {
					j++
				}
				b.WriteString(s[i:j])
				i = j
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runeWidth(r)
		if used+rw > w-1 {
			break
		}
		b.WriteString(s[i : i+size])
		used += rw
		i += size
	}
	b.WriteString("\x1b[0m…")
	return b.String()
}

// PadRight rellena con espacios hasta w celdas visibles.
func PadRight(s string, w int) string {
	d := w - Width(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}

// PadLeft añade espacios a la izquierda hasta w celdas visibles.
func PadLeft(s string, w int) string {
	d := w - Width(s)
	if d <= 0 {
		return s
	}
	return strings.Repeat(" ", d) + s
}

// Center centra el texto en w celdas.
func Center(s string, w int) string {
	d := w - Width(s)
	if d <= 0 {
		return s
	}
	left := d / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", d-left)
}

// Wrap divide el texto en líneas de como máximo w celdas respetando palabras.
// Las palabras más largas que el ancho disponible se parten en trozos.
func Wrap(text string, w int) []string {
	if w <= 0 {
		return []string{text}
	}
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := ""
		for _, word := range words {
			if cur != "" && Width(cur)+1+Width(word) <= w {
				cur += " " + word
				continue
			}
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			for Width(word) > w {
				head, rest := splitAt(word, w)
				lines = append(lines, head)
				word = rest
			}
			cur = word
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	return lines
}

// splitAt parte la cadena en la primera posición que ocupa w celdas.
func splitAt(s string, w int) (string, string) {
	used := 0
	for i, r := range s {
		rw := runeWidth(r)
		if used+rw > w {
			return s[:i], s[i:]
		}
		used += rw
	}
	return s, ""
}

// Sanitize limpia caracteres de control que romperían la alineación del
// terminal: los tabuladores se convierten en espacios y el resto de controles
// C0 se eliminan. Los saltos de línea se conservan.
func Sanitize(s string) string {
	needs := false
	for _, r := range s {
		if (r < 32 && r != '\n') || r == 0x7f {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\n':
			b.WriteByte('\n')
		case r < 32 || r == 0x7f:
			// se descarta
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

var mesesES = [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

var diasES = [...]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"}

// Fecha devuelve una fecha corta en español, por ejemplo "vie 23 oct".
func Fecha(t time.Time) string {
	return diasES[int(t.Weekday())] + " " + itoaFecha(t.Day()) + " " + mesesES[int(t.Month())-1]
}

// FechaHora devuelve "vie 23 oct · 04:31".
func FechaHora(t time.Time) string {
	return Fecha(t) + " · " + t.Format("15:04")
}

// Hora devuelve la hora en formato 15:04.
func Hora(t time.Time) string { return t.Format("15:04") }

func itoaFecha(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
