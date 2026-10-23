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
	// El glifo 0 lleva la primera fila encendida para poder comprobarlo.
	glyphs[0] = 0xff

	out := append(header, glyphs...)
	if withUnicode {
		// Tabla: secuencia 0xFFFE "A" "B" 0xFFFF, luego un código suelto.
		table := []uint16{0xFFFE, 'A', 'B', 0xFFFF, 'Z'}
		for _, v := range table {
			var b [2]byte
			binary.LittleEndian.PutUint16(b[:], v)
			out = append(out, b[:]...)
		}
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
	// La secuencia de la tabla Unicode asigna A y B al primer glifo.
	if string(f.Glyph('A')) != string(f.Glyph(rune(0))) {
		t.Fatal("la tabla Unicode no mapeó la A")
	}
	if string(f.Glyph('B')) != string(f.Glyph(rune(0))) {
		t.Fatal("la tabla Unicode no mapeó la B")
	}
	// El código suelto posterior va al glifo siguiente.
	if string(f.Glyph('Z')) != string(f.Glyph(rune(1))) {
		t.Fatal("la tabla Unicode no mapeó la Z")
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
