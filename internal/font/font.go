// Package font rasteriza texto en mapas de bits sin dependencias externas.
//
// Usa dos fuentes:
//
//   - Las fuentes de consola del sistema en formato PSF1/PSF2 (las instala el
//     paquete kbd, que forma parte de la base de Arch Linux). Soportan Unicode
//     completo, incluidas letras acentuadas y algunos iconos.
//   - Una fuente 5×7 incorporada en el binario, que garantiza que la barra se
//     dibuje siempre, incluso en un sistema recién instalado.
package font

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Face es una fuente de mapa de bits ya cargada.
type Face struct {
	Name   string // nombre legible (para diagnóstico)
	W, H   int    // tamaño de la celda en píxeles
	glyphs map[rune][]uint8
	// fallback se usa cuando falta un carácter: se toma del juego básico.
	upper bool
}

// Glyph devuelve el mapa de bits de un carácter: H filas de W bits (1 = píxel
// encendido), empaquetados en bytes con el bit más significativo a la izquierda.
func (f *Face) Glyph(r rune) []byte {
	if b, ok := f.glyphs[r]; ok {
		return b
	}
	// Se prueban variantes razonables antes de rendirse con el bloque lleno.
	if f.upper {
		if up := toUpper(r); up != r {
			if b, ok := f.glyphs[up]; ok {
				return b
			}
		}
	}
	// Sin acentos: "á" → "a".
	if base := stripAccent(r); base != r {
		if b, ok := f.glyphs[base]; ok {
			return b
		}
	}
	if b, ok := f.glyphs['?']; ok {
		return b
	}
	return f.glyphs[' ']
}

// Advance es el ancho de avance de un carácter (celda + separación).
func (f *Face) Advance(spacing int) int { return f.W + spacing }

// Measure devuelve el ancho en píxeles de un texto.
func (f *Face) Measure(s string, scale, spacing int) int {
	n := 0
	for range s {
		n++
	}
	if n == 0 {
		return 0
	}
	return n*((f.W+spacing)*scale) - spacing*scale
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	switch r {
	case 'á':
		return 'Á'
	case 'é':
		return 'É'
	case 'í':
		return 'Í'
	case 'ó':
		return 'Ó'
	case 'ú':
		return 'Ú'
	case 'ñ':
		return 'Ñ'
	case 'ü':
		return 'Ü'
	}
	return r
}

// stripAccent quita el acento de las vocales españolas más habituales.
func stripAccent(r rune) rune {
	switch r {
	case 'á', 'à', 'ä', 'â', 'ã':
		return 'a'
	case 'é', 'è', 'ë', 'ê':
		return 'e'
	case 'í', 'ì', 'ï', 'î':
		return 'i'
	case 'ó', 'ò', 'ö', 'ô', 'õ':
		return 'o'
	case 'ú', 'ù', 'ü', 'û':
		return 'u'
	case 'ñ':
		return 'n'
	case 'ç':
		return 'c'
	case 'Á', 'À', 'Ä', 'Â', 'Ã':
		return 'A'
	case 'É', 'È', 'Ë', 'Ê':
		return 'E'
	case 'Í', 'Ì', 'Ï', 'Î':
		return 'I'
	case 'Ó', 'Ò', 'Ö', 'Ô', 'Õ':
		return 'O'
	case 'Ú', 'Ù', 'Ü', 'Û':
		return 'U'
	case 'Ñ':
		return 'N'
	case 'Ç':
		return 'C'
	}
	return r
}

// ───────────────────────── fuentes del sistema (PSF) ─────────────────────────

// SearchDirs son los directorios donde se buscan fuentes de consola.
var SearchDirs = []string{
	"/usr/share/kbd/consolefonts",
	"/usr/share/consolefonts",
	"/usr/local/share/consolefonts",
	"/lib/kbd/consolefonts",
}

// DefaultNames son las fuentes preferidas, por orden. Se eligen por ser
// legibles a tamaños pequeños y de ancho fijo.
var DefaultNames = []string{
	"ter-132n", "ter-124n", "ter-120n", "ter-118n", "ter-116n", "ter-114n",
	"Lat2-Terminus16", "Lat15-Terminus16", "Terminus16",
	"default8x16", "default16", "LatArCyrHeb-16", "sun12x22", "default8x8",
	"eurlatgr",
}

// LoadSystem busca y carga la mejor fuente de consola disponible. Devuelve
// (nil, error) si no hay ninguna utilizable; el llamador debe caer a Builtin.
func LoadSystem(preferred string) (*Face, error) {
	var candidates []string
	if preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			candidates = append(candidates, preferred)
		}
	}
	for _, dir := range SearchDirs {
		for _, name := range DefaultNames {
			for _, ext := range []string{".psf.gz", ".psfu.gz", ".psf", ".psfu"} {
				p := filepath.Join(dir, name+ext)
				if _, err := os.Stat(p); err == nil {
					candidates = append(candidates, p)
				}
			}
		}
	}
	// Cualquier otra .psf que haya, por si solo existe una.
	if len(candidates) == 0 {
		for _, dir := range SearchDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			var extra []string
			for _, e := range entries {
				n := e.Name()
				if strings.HasSuffix(n, ".psf") || strings.HasSuffix(n, ".psf.gz") ||
					strings.HasSuffix(n, ".psfu") || strings.HasSuffix(n, ".psfu.gz") {
					extra = append(extra, filepath.Join(dir, n))
				}
			}
			sort.Strings(extra)
			candidates = append(candidates, extra...)
		}
	}
	var lastErr error = fmt.Errorf("no se encontró ninguna fuente de consola (instala el paquete kbd)")
	for _, path := range candidates {
		f, err := Load(path)
		if err == nil {
			return f, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// Available indica si hay alguna fuente de consola en el sistema.
func Available() bool {
	for _, dir := range SearchDirs {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}
	return false
}

// Builtin devuelve la fuente 5×7 incorporada.
func Builtin() *Face {
	if builtinFace == nil {
		builtinFace = buildBuiltin()
	}
	return builtinFace
}

// Best intenta cargar una fuente del sistema y cae a la incorporada.
func Best(preferred string) *Face {
	if f, err := LoadSystem(preferred); err == nil {
		return f
	}
	return Builtin()
}
