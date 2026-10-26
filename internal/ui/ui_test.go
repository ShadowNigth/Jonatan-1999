package ui

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWidthAndStrip(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"hola", 4},
		{"\x1b[31mhola\x1b[0m", 4},
		{"ñandú", 5},
		{"日本語", 6},
		{"e\u0301", 1}, // e + acento combinante
		{"", 0},
	}
	for _, c := range cases {
		if got := Width(c.in); got != c.want {
			t.Fatalf("Width(%q) = %d, esperaba %d", c.in, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("abcdef", 4); got != "abc…" {
		t.Fatalf("Truncate = %q", got)
	}
	if got := Truncate("abc", 4); got != "abc" {
		t.Fatalf("Truncate sin recorte = %q", got)
	}
	if got := Truncate("日本語です", 4); Width(got) > 4 {
		t.Fatalf("Truncate ancho = %q (%d)", got, Width(got))
	}
	if got := TruncateANSI("\x1b[31mabcdef\x1b[0m", 4); !strings.Contains(got, "abc") || !strings.HasSuffix(got, "\x1b[0m…") {
		t.Fatalf("TruncateANSI = %q", got)
	}
}

func TestPadAndWrap(t *testing.T) {
	if PadRight("ab", 5) != "ab   " {
		t.Fatal("PadRight")
	}
	if PadLeft("ab", 4) != "  ab" {
		t.Fatal("PadLeft")
	}
	if Center("ab", 6) != "  ab  " {
		t.Fatal("Center")
	}
	lines := Wrap("uno dos tres cuatro cinco", 10)
	for _, l := range lines {
		if Width(l) > 10 {
			t.Fatalf("línea demasiado larga: %q", l)
		}
	}
	if strings.Join(lines, " ") != "uno dos tres cuatro cinco" {
		t.Fatalf("Wrap perdió texto: %q", lines)
	}
	// Una palabra más larga que el ancho se parte.
	long := Wrap("supercalifragilisticoespialidoso", 8)
	if len(long) < 3 {
		t.Fatalf("Wrap de palabra larga = %q", long)
	}
	for _, l := range long {
		if Width(l) > 8 {
			t.Fatalf("trozo demasiado largo: %q", l)
		}
	}
	if got := strings.Join(long, ""); got != "supercalifragilisticoespialidoso" {
		t.Fatalf("se perdió texto al partir: %q", got)
	}
}

func TestFuzzy(t *testing.T) {
	cases := []struct {
		pattern, target string
		want            bool
	}{
		{"ffx", "Firefox", true},
		{"trm", "Terminal", true},
		{"xyz", "Terminal", false},
		{"", "lo que sea", true},
		{"FI", "firefox", true},
	}
	for _, c := range cases {
		_, ok := Fuzzy(c.pattern, c.target)
		if ok != c.want {
			t.Fatalf("Fuzzy(%q, %q) = %v", c.pattern, c.target, ok)
		}
	}
	// Un prefijo exacto debe puntuar más que una coincidencia dispersa.
	good, _ := Fuzzy("fire", "firefox")
	bad, _ := Fuzzy("fire", "f-i-r-e-something")
	if good.Score <= bad.Score {
		t.Fatalf("puntuación inesperada: %d <= %d", good.Score, bad.Score)
	}

	match, ok := Fuzzy("ffx", "Firefox")
	if !ok || len(match.Positions) != 3 || match.Positions[0] != 0 || match.Positions[1] != 4 || match.Positions[2] != 6 {
		t.Fatalf("posiciones de coincidencia inesperadas: %+v, ok=%v", match, ok)
	}
	match, ok = Fuzzy("ñ", "Ñandú")
	if !ok || len(match.Positions) != 1 || match.Positions[0] != 0 {
		t.Fatalf("coincidencia Unicode inesperada: %+v, ok=%v", match, ok)
	}
}

func TestFilterItems(t *testing.T) {
	type item struct{ name string }
	items := []item{{"Firefox"}, {"Files"}, {"Terminal"}, {"Calculadora"}}
	idx := FilterItems(items, "fi", func(i item) []string { return []string{i.name} })
	if len(idx) != 2 {
		t.Fatalf("esperaba 2 resultados, obtuve %v", idx)
	}
	if items[idx[0]].name != "Files" && items[idx[0]].name != "Firefox" {
		t.Fatalf("resultado inesperado: %v", items[idx[0]])
	}
}

func TestCtrlRune(t *testing.T) {
	k, err := (&KeyReader{}).decodeFirst(0x04) // Ctrl+D
	if err != nil {
		t.Fatalf("decodificando Ctrl+D: %v", err)
	}
	if k.Rune != 'd' || !k.Ctrl || k.Name != "ctrl+d" {
		t.Fatalf("Ctrl+D se decodificó como %+v", k)
	}
	if !k.IsCtrlRune('d') {
		t.Fatal("IsCtrlRune no reconoció Ctrl+D")
	}
	if k.IsCtrlRune('x') || k.IsRune('d') || (Key{Rune: 'd', Ctrl: true, Alt: true}).IsCtrlRune('d') {
		t.Fatal("la tecla de control coincidió con una tecla distinta o con un modificador extra")
	}
}

func TestKeyReader(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	kr := NewKeyReader(pr)
	defer kr.Close()
	go func() {
		for _, chunk := range []string{
			"a", "\x1b[A", "\x1b[B", "\x1b[Z", "\x03", "\r", "\x7f", "ñ", "\x1b[1;5C", "\x1bx",
		} {
			_, _ = pw.Write([]byte(chunk))
		}
	}()
	want := []Key{
		{Rune: 'a'},
		{Name: "up"},
		{Name: "down"},
		{Name: "backtab"},
		{Rune: 'c', Ctrl: true, Name: "ctrl+c"},
		{Name: "enter"},
		{Name: "backspace"},
		{Rune: 'ñ'},
		{Name: "right", Ctrl: true},
		{Rune: 'x', Alt: true},
	}
	for i, w := range want {
		got, err := kr.Read()
		if err != nil {
			t.Fatalf("clave %d: %v", i, err)
		}
		if got.Rune != w.Rune || got.Name != w.Name || got.Ctrl != w.Ctrl || got.Alt != w.Alt {
			t.Fatalf("clave %d = %+v, esperaba %+v", i, got, w)
		}
	}
}

func TestKeyReaderCloseStopsBlockedPipe(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	kr := NewKeyReader(pr)
	keys := kr.Keys()
	kr.Close()

	select {
	case _, ok := <-keys:
		if ok {
			t.Fatal("el lector publicó una tecla al cerrarse sin entrada")
		}
	case <-time.After(time.Second):
		t.Fatal("Close no liberó el lector bloqueado")
	}
}

func TestFileKeyReaderClosesWithoutClosingFile(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	defer writeEnd.Close()

	kr := NewKeyReader(readEnd)
	keys := kr.Keys()
	if _, err := writeEnd.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case key, ok := <-keys:
		if !ok || key.Rune != 'q' {
			t.Fatalf("tecla inesperada: %+v, abierto=%v", key, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("el lector de archivos no recibió la tecla")
	}

	kr.Close()
	select {
	case _, ok := <-keys:
		if ok {
			t.Fatal("quedó una tecla tras cerrar el reader")
		}
	case <-time.After(time.Second):
		t.Fatal("el reader de archivo no se detuvo al cerrar")
	}
	kr2 := NewKeyReader(readEnd)
	keys2 := kr2.Keys()
	defer kr2.Close()
	if _, err := writeEnd.Write([]byte("x")); err != nil {
		t.Fatalf("Close cerró el descriptor que pertenece al llamador: %v", err)
	}
	select {
	case key, ok := <-keys2:
		if !ok || key.Rune != 'x' {
			t.Fatalf("segundo menú recibió una tecla inesperada: %+v, abierto=%v", key, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("el segundo menú no recibió entrada tras cerrar el primero")
	}
}

func TestSelectorFilteredCacheInvalidation(t *testing.T) {
	sel := NewSelector(NewTheme("catppuccin", false), "cache", []Item{
		{Key: "a", Label: "Alpha"},
		{Key: "b", Label: "Beta"},
	})
	sel.SetFilter("al")
	first := sel.Filtered()
	second := sel.Filtered()
	if len(first) != 1 || first[0].Key != "a" || &first[0] != &second[0] {
		t.Fatalf("el resultado filtrado no se conservó: %v / %v", first, second)
	}

	sel.SetItems([]Item{{Key: "b", Label: "Beta"}})
	if got := sel.Filtered(); len(got) != 0 {
		t.Fatalf("SetItems dejó una búsqueda obsoleta: %+v", got)
	}
	sel.SetFilter("be")
	if got := sel.Filtered(); len(got) != 1 || got[0].Key != "b" {
		t.Fatalf("SetFilter no invalidó el resultado: %+v", got)
	}
	sel.SetBypassFilter(true)
	if got := sel.Filtered(); len(got) != 1 || got[0].Key != "b" {
		t.Fatalf("BypassFilter no aplicó: %+v", got)
	}
}

func TestSelectorView(t *testing.T) {
	SetProfile(ProfileNone)
	defer SetProfile(ProfileTrueColor)

	theme := NewTheme("catppuccin", true)
	sel := NewSelector(theme, "prueba", []Item{
		{Key: "1", Label: "Firefox", Hint: "navegador", Icon: "@"},
		{Key: "2", Label: "Terminal", Hint: "consola", Icon: ">"},
		{Key: "3", Label: "Archivos", Hint: "files", Icon: "~"},
	})
	view := sel.View(60, 16)
	lines := strings.Split(view, "\n")
	if len(lines) != 16 {
		t.Fatalf("esperaba 16 líneas, obtuve %d", len(lines))
	}
	for i, l := range lines {
		if Width(l) != 60 {
			t.Fatalf("línea %d mide %d celdas: %q", i, Width(l), l)
		}
	}
	if !strings.Contains(view, "Firefox") {
		t.Fatal("no aparece la primera entrada")
	}
	sel.SetFilter("term")
	if got := sel.Filtered(); len(got) != 1 || got[0].Label != "Terminal" {
		t.Fatalf("filtro inesperado: %+v", got)
	}
	sel.Move(5)
	if it, _ := sel.Selected(); it.Key != "2" {
		t.Fatalf("cursor fuera de rango: %+v", it)
	}
}

func TestFrame(t *testing.T) {
	SetProfile(ProfileNone)
	defer SetProfile(ProfileTrueColor)
	theme := NewTheme("nord", false)
	frame := theme.Frame(30, 6, "título", "pie", []string{"hola", "mundo"})
	lines := strings.Split(frame, "\n")
	if len(lines) != 6 {
		t.Fatalf("alto = %d", len(lines))
	}
	for i, l := range lines {
		if Width(l) != 30 {
			t.Fatalf("línea %d: %q (%d)", i, l, Width(l))
		}
	}
	if !strings.Contains(lines[0], "título") || !strings.Contains(lines[5], "pie") {
		t.Fatalf("título/pie no dibujados:\n%s", frame)
	}
}

func TestScroll(t *testing.T) {
	thumb := Scroll(10, 100, 10, 0)
	count := 0
	for _, c := range thumb {
		if c != " " {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("pulgador de tamaño %d", count)
	}
	if got := Scroll(10, 10, 10, 0); strings.Join(got, "") != strings.Repeat(" ", 10) {
		t.Fatal("sin barra cuando todo cabe")
	}
}

func TestColorSGR(t *testing.T) {
	SetProfile(ProfileTrueColor)
	if got := Color("#ff0000").SGR(false); got != "\x1b[38;2;255;0;0m" {
		t.Fatalf("SGR truecolor = %q", got)
	}
	SetProfile(ProfileANSI256)
	if got := Color("#ff0000").SGR(false); !strings.HasPrefix(got, "\x1b[38;5;") {
		t.Fatalf("SGR 256 = %q", got)
	}
	SetProfile(ProfileNone)
	if got := Color("#ff0000").SGR(false); got != "" {
		t.Fatalf("SGR sin color = %q", got)
	}
	SetProfile(ProfileTrueColor)
	defer SetProfile(ProfileTrueColor)
}

func TestPalettesCompletas(t *testing.T) {
	for _, name := range PaletteNames() {
		p := BuiltinPalettes[name]
		if p.Name == "" {
			t.Fatalf("%s sin nombre", name)
		}
		for _, c := range []Color{p.Bg, p.BgAlt, p.BgSel, p.Fg, p.FgDim, p.Muted, p.Accent, p.Accent2, p.Ok, p.Warn, p.Err, p.Border} {
			if _, _, _, ok := hexToRGB(c); !ok {
				t.Fatalf("%s: color inválido %q", name, c)
			}
		}
	}
}

func TestTimeoutDeEscape(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	kr := NewKeyReader(pr)
	defer kr.Close()
	done := make(chan Key, 1)
	go func() {
		k, _ := kr.Read()
		done <- k
	}()
	_, _ = pw.Write([]byte{0x1b})
	select {
	case k := <-done:
		if k.Name != "esc" {
			t.Fatalf("esperaba esc, obtuve %+v", k)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el lector se bloqueó con un ESC aislado")
	}
}
