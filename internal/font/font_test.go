package font

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinFace(t *testing.T) {
	f := Builtin()
	if f.W != 5 || f.H != 7 {
		t.Fatalf("fuente incorporada %dx%d", f.W, f.H)
	}
	// Todo el ASCII imprimible debe tener glifo definido.
	for r := rune(32); r < 127; r++ {
		if _, ok := f.glyphs[r]; !ok {
			t.Fatalf("falta el glifo %q (%d)", r, r)
		}
	}
	// Los caracteres desconocidos caen al bloque de interrogación, no al pánico.
	if got := string(f.Glyph('ß')); got == "" {
		t.Fatal("glifo de reserva vacío")
	}
	if _, ok := f.glyphs['?']; !ok {
		t.Fatal("falta el glifo de reserva")
	}
}

func TestGlyphPixels(t *testing.T) {
	f := Builtin()
	// La 'A' debe tener píxeles encendidos en su primera fila.
	rows := f.Glyph('A')
	if len(rows) != 7 {
		t.Fatalf("%d filas", len(rows))
	}
	if rows[0] == 0 {
		t.Fatal("la primera fila de la A está vacía")
	}
	// El espacio no tiene ningún píxel.
	for _, r := range f.Glyph(' ') {
		if r != 0 {
			t.Fatal("el espacio tiene píxeles encendidos")
		}
	}
}

func TestEachAndMeasure(t *testing.T) {
	f := Builtin()
	count := 0
	minX, maxX := 1<<30, -1
	minY, maxY := 1<<30, -1
	f.Each("Hola", 2, 0, func(x, y int) {
		count++
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	})
	if count == 0 {
		t.Fatal("no se dibujó ningún píxel")
	}
	if maxX >= f.Measure("Hola", 2, 0) {
		t.Fatalf("los píxeles (%d) se salen del ancho medido (%d)", maxX, f.Measure("Hola", 2, 0))
	}
	if maxY >= f.Rows(2) {
		t.Fatalf("los píxeles (%d) se salen del alto (%d)", maxY, f.Rows(2))
	}
	// La escala multiplica por cuatro el número de píxeles.
	if f.Measure("Hola", 2, 0) != 2*f.Measure("Hola", 1, 0) {
		t.Fatal("Measure no escala linealmente")
	}
	if f.Measure("", 1, 0) != 0 {
		t.Fatal("la cadena vacía debe medir 0")
	}
}

func TestGlyphFallbackForEllipsisAndTypographicPunctuation(t *testing.T) {
	face := &Face{W: 5, H: 1, glyphs: map[rune][]uint8{
		'.': {0x10}, '?': {0x20}, '-': {0x30}, '\'': {0x40},
	}}
	if !bytes.Equal(face.Glyph('…'), face.Glyph('.')) {
		t.Fatal("la elipsis sin glifo debe verse como un punto, no como un signo de interrogación")
	}
	if !bytes.Equal(face.Glyph('—'), face.Glyph('-')) {
		t.Fatal("la raya larga debería usar el guion ASCII como reserva")
	}
	if !bytes.Equal(face.Glyph('’'), face.Glyph('\'')) {
		t.Fatal("la comilla tipográfica debería usar la comilla ASCII como reserva")
	}
}

func TestMeasureSkipsControlsAndCombiningMarks(t *testing.T) {
	face := Builtin()
	for _, test := range []struct{ with, plain string }{
		{"e\u0301", "e"},
		{"A\x01B", "AB"},
		{"A\u200dB", "AB"},
	} {
		if got, want := face.Measure(test.with, 1, 1), face.Measure(test.plain, 1, 1); got != want {
			t.Errorf("Measure(%q) = %d, quería %d como %q", test.with, got, want, test.plain)
		}
	}
}

func TestAccentFallback(t *testing.T) {
	f := Builtin()
	if string(f.Glyph('á')) != string(f.Glyph('a')) {
		t.Fatal("«á» debería caer a «a»")
	}
	if string(f.Glyph('ñ')) != string(f.Glyph('n')) {
		t.Fatal("«ñ» debería caer a «n»")
	}
}

// buildPSF2 construye un PSF2 mínimo válido en memoria.
func buildPSF2(width, height, count int, withUnicode bool) []byte {
	charSize := height * ((width + 7) / 8)
	header := make([]byte, 32)
	binary.LittleEndian.PutUint32(header[0:], 0x864ab572)
	binary.LittleEndian.PutUint32(header[4:], 0)
	binary.LittleEndian.PutUint32(header[8:], 32) // headerSize
	flags := uint32(0)
	if withUnicode {
		flags = 1
	}
	binary.LittleEndian.PutUint32(header[12:], flags)
	binary.LittleEndian.PutUint32(header[16:], uint32(count))
	binary.LittleEndian.PutUint32(header[20:], uint32(charSize))
	binary.LittleEndian.PutUint32(header[24:], uint32(height))
	binary.LittleEndian.PutUint32(header[28:], uint32(width))

	glyphs := make([]byte, charSize*count)
	// Cada glifo tiene un patrón distinto para verificar los alias Unicode.
	for i := 0; i < count && i < 8; i++ {
		glyphs[i*charSize] = byte(0x80 >> i)
	}

	out := append(header, glyphs...)
	if withUnicode {
		// PSF2 usa UTF-8 y 0xFF como separador de cada glifo. B y ñ
		// comparten el segundo glifo como dos alias independientes.
		table := []byte{'A', 0xFF, 'B', 0xC3, 0xB1, 0xFF, 'Z', 0xFF, 'Q', 0xFF}
		out = append(out, table...)
	}
	return out
}

func TestLoadPSF2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.psf")
	if err := os.WriteFile(path, buildPSF2(8, 16, 4, true), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.W != 8 || f.H != 16 {
		t.Fatalf("tamaño %dx%d", f.W, f.H)
	}
	if f.Name != "test.psf" {
		t.Fatalf("nombre %q", f.Name)
	}
	if got := f.Glyph('A')[0]; got != 0x80 {
		t.Fatalf("A se asignó al glifo equivocado: primer byte %02x", got)
	}
	if got := f.Glyph('B')[0]; got != 0x40 {
		t.Fatalf("B se asignó al glifo equivocado: primer byte %02x", got)
	}
	if got := f.Glyph('ñ')[0]; got != 0x40 {
		t.Fatalf("el alias UTF-8 ñ se asignó al glifo equivocado: %02x", got)
	}
	if got := f.Glyph('Z')[0]; got != 0x20 {
		t.Fatalf("Z se asignó al glifo equivocado: primer byte %02x", got)
	}
	if _, ok := f.glyphs['C']; ok {
		t.Fatal("la tabla Unicode no debe inventar un alias ASCII según el índice del glifo")
	}
}

func TestPSF2UnicodeSequenceKeepsOnlyItsFirstRune(t *testing.T) {
	face := &Face{glyphs: make(map[rune][]uint8)}
	glyphs := [][]uint8{{0x80}}
	if err := applyPSF2UnicodeTable(face, []byte{0xFE, 'f', 'i', 0xFF}, glyphs); err != nil {
		t.Fatal(err)
	}
	if face.glyphs['f'][0] != 0x80 {
		t.Fatal("la primera runa de la secuencia debe conservar su alias individual")
	}
	if _, ok := face.glyphs['i']; ok {
		t.Fatal("la continuación de la secuencia no debe convertirse en alias del glifo")
	}
}

func TestPSF1UnicodeTableMapsAliases(t *testing.T) {
	face := &Face{glyphs: make(map[rune][]uint8)}
	glyphs := [][]uint8{{0x80}, {0x40}}
	var table []byte
	for _, codepoint := range []uint16{'A', 'Á', 0xFFFE, 'f', 'i', 0xFFFF, 'B', 0xFFFF} {
		var word [2]byte
		binary.LittleEndian.PutUint16(word[:], codepoint)
		table = append(table, word[:]...)
	}
	applyPSF1UnicodeTable(face, table, glyphs)
	if face.glyphs['A'][0] != 0x80 || face.glyphs['Á'][0] != 0x80 || face.glyphs['B'][0] != 0x40 {
		t.Fatal("la tabla PSF1 no asoció correctamente los alias a sus glifos")
	}
	if face.glyphs['f'][0] != 0x80 {
		t.Fatal("la primera runa de la secuencia PSF1 debe conservar su alias individual")
	}
	if _, ok := face.glyphs['i']; ok {
		t.Fatal("la continuación de una secuencia PSF1 no debe ser alias independiente")
	}
}

func TestLoadPSF2Gzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.psfu.gz")
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(buildPSF2(8, 8, 2, false)); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.H != 8 {
		t.Fatalf("alto %d", f.H)
	}
}

// buildPSF1 construye un PSF1 mínimo.
func buildPSF1(width, height, count int) []byte {
	out := []byte{0x36, 0x04}
	mode := byte(0)
	if count == 512 {
		mode |= 1
	}
	out = append(out, mode, byte(height))
	for i := 0; i < count; i++ {
		row := make([]byte, height)
		if i == 'X' {
			row[0] = 0x80
		}
		out = append(out, row...)
	}
	return out
}

func TestLoadPSF1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vga.psf")
	if err := os.WriteFile(path, buildPSF1(8, 16, 256), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.W != 8 || f.H != 16 {
		t.Fatalf("tamaño %dx%d", f.W, f.H)
	}
	if f.Glyph('X')[0] != 0x80 {
		t.Fatal("el glifo de la X no se cargó")
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]byte{
		"vacio.psf":    {},
		"basura.psf":   []byte("no soy una fuente"),
		"truncada.psf": buildPSF2(8, 16, 8, false)[:40],
	}
	for name, data := range cases {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("%s debería fallar", name)
		}
	}
	if _, err := Load(filepath.Join(dir, "no-existe.psf")); err == nil {
		t.Fatal("un archivo inexistente debería fallar")
	}
}

func TestBestAndSystem(t *testing.T) {
	// No se exige que haya fuentes: solo que la función devuelva algo usable.
	f := Best("")
	if f == nil || f.W <= 0 || f.H <= 0 {
		t.Fatal("Best no devolvió una fuente válida")
	}
	_ = Available()
}
