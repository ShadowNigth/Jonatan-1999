package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTOML(t *testing.T) {
	src := `
# comentario
[general]
theme = "nord"   # al final de la línea
nerd_font = true
font = "JetBrainsMono Nerd Font"
numero = 42
flotante = 1.5

[ui]
width = 80

[launcher]
historial = ["a", "b", 3]

[input]
keyboard_layout = 'latam'
hash_en_texto = "no # es comentario"
`
	vals, err := parseTOML(src)
	if err != nil {
		t.Fatalf("parseTOML: %v", err)
	}
	tests := map[string]any{
		"general.theme":         "nord",
		"general.nerd_font":     true,
		"general.font":          "JetBrainsMono Nerd Font",
		"general.numero":        42,
		"general.flotante":      1.5,
		"ui.width":              80,
		"input.keyboard_layout": "latam",
		"input.hash_en_texto":   "no # es comentario",
	}
	for key, want := range tests {
		got, ok := vals[key]
		if !ok {
			t.Fatalf("falta la clave %q", key)
		}
		if got != want {
			t.Fatalf("%s = %#v, esperaba %#v", key, got, want)
		}
	}
	arr, ok := vals["launcher.historial"].([]string)
	if !ok || len(arr) != 3 || arr[2] != "3" {
		t.Fatalf("array mal parseado: %#v", vals["launcher.historial"])
	}
}

func TestParseTOMLErrores(t *testing.T) {
	for _, src := range []string{
		"[general\ntheme = \"x\"",
		"general.theme",
		"general.theme = ",
		"general.theme = \"sin cerrar",
	} {
		if _, err := parseTOML(src); err == nil {
			t.Fatalf("debía fallar: %q", src)
		}
	}
}

func TestLoadFileYAplicar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[general]
theme = "gruvbox"
nerd_font = false
terminal = "kitty"
confirm_power = false

[ui]
font_size = 13
bar_font_size = 15
border = "double"

[input]
keyboard_layout = "us"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if cfg.Theme != "gruvbox" || cfg.NerdFont || cfg.Terminal != "kitty" || cfg.ConfirmPower {
		t.Fatalf("valores no aplicados: %+v", cfg)
	}
	if cfg.FontSize != 13 || cfg.BarFontSize != 15 || cfg.Border != "double" {
		t.Fatalf("ui no aplicada: %+v", cfg)
	}
	if cfg.KeyboardLayout != "us" {
		t.Fatalf("layout = %q", cfg.KeyboardLayout)
	}
	// Los valores no presentes conservan el valor por defecto.
	if cfg.Editor != "nvim" {
		t.Fatalf("editor por defecto alterado: %q", cfg.Editor)
	}
	if cfg.BorderStyle() != 3 {
		t.Fatalf("BorderStyle = %d", cfg.BorderStyle())
	}
}

func TestLoadSinArchivo(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load sin archivo no debía fallar: %v", err)
	}
	if cfg.Theme != "catppuccin" {
		t.Fatalf("tema por defecto = %q", cfg.Theme)
	}
}

func TestSaveThemeConservaContenido(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	original := "# mi comentario\n[general]\ntheme = \"nord\"\nterminal = \"foot\"\n\n[ui]\nwidth = 99\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTheme("dracula"); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !contains(text, `theme = "dracula"`) {
		t.Fatalf("tema no guardado:\n%s", text)
	}
	if !contains(text, "# mi comentario") || !contains(text, "width = 99") || !contains(text, `terminal = "foot"`) {
		t.Fatalf("se perdió contenido:\n%s", text)
	}
	if cfg.Theme != "dracula" {
		t.Fatalf("cfg.Theme = %q", cfg.Theme)
	}
}

func TestDefaultFileEsParseable(t *testing.T) {
	cfg := Default()
	text := DefaultFile(cfg)
	if _, err := parseTOML(text); err != nil {
		t.Fatalf("el archivo por defecto no es parseable: %v\n%s", err, text)
	}
}

func TestSet(t *testing.T) {
	cfg := Default()
	if err := cfg.Set("general.theme=nord"); err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "nord" {
		t.Fatalf("Set no aplicó el tema: %q", cfg.Theme)
	}
	if err := cfg.Set("sin-igual"); err == nil {
		t.Fatal("Set debía fallar sin '='")
	}
	if got := cfg.Int("ui.no_existe", 10); got != 10 {
		t.Fatalf("Int con valor por defecto = %d", got)
	}
	if err := cfg.Set("ui.width=80"); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Int("ui.width", 10); got != 80 {
		t.Fatalf("Int = %d", got)
	}
	if got := cfg.String("no.existe", "defecto"); got != "defecto" {
		t.Fatalf("String = %q", got)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestBootAndEffectsDefaults(t *testing.T) {
	c := Default()
	if !c.BootEnabled {
		t.Fatal("la animación de arranque debería venir activada")
	}
	if c.BootStyle != "sakura" {
		t.Fatalf("estilo por defecto = %q", c.BootStyle)
	}
	if c.BootDuration <= 0 || !c.BootLogo {
		t.Fatalf("valores de arranque inesperados: %+v", c)
	}
	if c.Browser != "chromium" {
		t.Fatalf("navegador por defecto = %q", c.Browser)
	}
	if c.TerminalOpacity <= 0 || c.TerminalOpacity > 1 {
		t.Fatalf("opacidad de terminal = %v", c.TerminalOpacity)
	}
	if c.WallpaperFPS <= 0 || c.WallpaperKind == "" {
		t.Fatalf("fondo por defecto incompleto: %+v", c)
	}
	// El archivo generado incluye las secciones nuevas y sin marcadores.
	text := DefaultFile(c)
	for _, want := range []string{"[boot]", "style = \"sakura\"", "[effects]", "[wallpaper]"} {
		if !contains(text, want) {
			t.Fatalf("falta %q en la configuración generada", want)
		}
	}
	if contains(text, "{{") {
		t.Fatal("la configuración generada contiene marcadores sin sustituir")
	}
}

func TestLoadBootSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := "[boot]\nenabled = false\nstyle = \"matrix\"\nduration_ms = 1200\nlogo = false\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BootEnabled || cfg.BootStyle != "matrix" || cfg.BootDuration != 1200 || cfg.BootLogo {
		t.Fatalf("sección [boot] mal aplicada: %+v", cfg)
	}
}
