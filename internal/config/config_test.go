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

[gtk]
theme = "Adwaita-dark"
icon_theme = "Papirus-Dark"
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
	if cfg.GTKTheme != "Adwaita-dark" || cfg.GTKIconTheme != "Papirus-Dark" {
		t.Fatalf("configuración GTK no aplicada: %q / %q", cfg.GTKTheme, cfg.GTKIconTheme)
	}
	// Los valores no presentes conservan el valor por defecto.
	if cfg.Editor != "nvim" {
		t.Fatalf("editor por defecto alterado: %q", cfg.Editor)
	}
	if cfg.BorderStyle() != 3 {
		t.Fatalf("BorderStyle = %d", cfg.BorderStyle())
	}
}

func TestDefaultBarModulesExcludePowerCPUAndRAM(t *testing.T) {
	for _, module := range Default().BarModules {
		switch module {
		case "power":
			t.Fatal("el botón de energía ya no es un módulo de la barra")
		case "cpu", "ram":
			t.Fatalf("CPU y RAM no deben activarse en la barra por defecto: %q", module)
		}
	}
}

func TestBarOpacityDefaultsAreTranslucent(t *testing.T) {
	cfg := Default()
	if cfg.BarOpacity != 0.68 || cfg.BarBackgroundOpacity != 0.68 {
		t.Fatalf("opacidades por defecto: waybar=%.2f swaybar=%.2f; ambas deben ser 0.68", cfg.BarOpacity, cfg.BarBackgroundOpacity)
	}
	text := DefaultFile(cfg)
	for _, want := range []string{"opacity = 0.68", "bar_opacity = 0.68"} {
		if !contains(text, want) {
			t.Errorf("la configuración generada debe incluir %q", want)
		}
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

func TestNotificationDefaultsAreCompact(t *testing.T) {
	cfg := Default()
	if cfg.NotifyWidth != 300 || cfg.NotifyPadding != 10 || cfg.NotifyGap != 6 ||
		cfg.NotifyRadius != 12 || cfg.NotifyMaxLines != 2 {
		t.Fatalf("valores compactos de notificación inesperados: ancho=%d relleno=%d separación=%d radio=%d líneas=%d",
			cfg.NotifyWidth, cfg.NotifyPadding, cfg.NotifyGap, cfg.NotifyRadius, cfg.NotifyMaxLines)
	}
	text := DefaultFile(cfg)
	for _, want := range []string{"width = 300", "padding = 10", "gap = 6", "max_lines = 2"} {
		if !contains(text, want) {
			t.Errorf("la configuración compacta debe incluir %q", want)
		}
	}
}

func TestOSDDefaults(t *testing.T) {
	c := Default()
	if !c.OSDEnabled || !c.OSDMediaEnabled {
		t.Fatal("los OSD de volumen y reproducción deberían venir activados")
	}
	if c.OSDMediaTimeout < 1000 {
		t.Fatalf("duración del OSD de reproducción poco razonable: %d", c.OSDMediaTimeout)
	}
	if c.OSDPosition != "bottom-center" {
		t.Fatalf("posición por defecto = %q", c.OSDPosition)
	}
	if c.OSDWidth != 260 || c.OSDPadding != 12 || c.OSDRadius != 14 || c.OSDTimeout < 500 || c.OSDSegments < 8 {
		t.Fatalf("valores compactos del OSD poco razonables: %+v", c)
	}
	if c.OSDOpacity <= 0 || c.OSDOpacity > 1 {
		t.Fatalf("opacidad del OSD = %v", c.OSDOpacity)
	}
	// El archivo generado trae la sección con sus valores.
	text := DefaultFile(c)
	for _, want := range []string{"[osd]", "media_enabled = true", "media_timeout_ms = 3200", "position = \"bottom-center\"", "width = 260", "padding = 12", "fade_ms = "} {
		if !contains(text, want) {
			t.Fatalf("falta %q en la configuración generada", want)
		}
	}
}

func TestLoadOSDSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := "[osd]\nenabled = false\nmedia_enabled = false\nmedia_timeout_ms = 4500\n" +
		"position = \"bottom-right\"\nwidth = 320\nmargin = 40\npadding = 12\n" +
		"radius = 8\nopacity = 0.8\ntimeout_ms = 900\nfade_ms = 0\nsegments = 12\nfont_scale = 3\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSDEnabled || cfg.OSDMediaEnabled || cfg.OSDMediaTimeout != 4500 ||
		cfg.OSDPosition != "bottom-right" || cfg.OSDWidth != 320 ||
		cfg.OSDMargin != 40 || cfg.OSDPadding != 12 || cfg.OSDRadius != 8 ||
		cfg.OSDTimeout != 900 || cfg.OSDFade != 0 || cfg.OSDSegments != 12 ||
		cfg.OSDFontScale != 3 {
		t.Fatalf("sección [osd] mal aplicada: %+v", cfg)
	}
	if cfg.OSDOpacity < 0.79 || cfg.OSDOpacity > 0.81 {
		t.Fatalf("opacidad mal leída: %v", cfg.OSDOpacity)
	}
	// Valores negativos imposibles se descartan.
	data = "[osd]\nradius = -5\nfade_ms = -10\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSDRadius < 0 || cfg.OSDFade < 0 {
		t.Fatalf("se aceptaron valores negativos: radio %d, fundido %d", cfg.OSDRadius, cfg.OSDFade)
	}
}

func TestWallpaperWorkspacesSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := `[wallpaper]
transition = "slide"
transition_ms = 700

[wallpaper.workspaces]
"1" = "scene:petals"
"3:web" = "image:~/fondos/ciudad.png"
web = "scene:grid"
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WallpaperTransition != "slide" || cfg.WallpaperTransitionMS != 700 {
		t.Fatalf("transición = %q %d", cfg.WallpaperTransition, cfg.WallpaperTransitionMS)
	}
	want := map[string]string{
		"1":     "scene:petals",
		"3:web": "image:~/fondos/ciudad.png",
		"web":   "scene:grid",
	}
	if len(cfg.WallpaperWorkspaces) != len(want) {
		t.Fatalf("reglas = %v", cfg.WallpaperWorkspaces)
	}
	for k, v := range want {
		if cfg.WallpaperWorkspaces[k] != v {
			t.Fatalf("%q = %q, se esperaba %q", k, cfg.WallpaperWorkspaces[k], v)
		}
	}
}

func TestWallpaperWorkspacesInline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := "[wallpaper]\nworkspaces = [\"1=scene:petals\", \"web=scene:city\", \"sin-igual\"]\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WallpaperWorkspaces["1"] != "scene:petals" || cfg.WallpaperWorkspaces["web"] != "scene:city" {
		t.Fatalf("forma compacta mal leída: %v", cfg.WallpaperWorkspaces)
	}
	if len(cfg.WallpaperWorkspaces) != 2 {
		t.Fatalf("entradas sobrantes: %v", cfg.WallpaperWorkspaces)
	}
}

func TestDefaultFileRoundTripsWorkspaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := Default()
	cfg.WallpaperWorkspaces = map[string]string{"1": "scene:petals", "chat de tarde": "image:~/a.png"}
	cfg.WallpaperTransition = "wipe"
	cfg.WallpaperTransitionMS = 250
	cfg.Path = path
	cfg.Dir = dir
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := LoadFile(path)
	if err != nil {
		t.Fatalf("el archivo guardado no se puede volver a leer: %v", err)
	}
	if back.WallpaperTransition != "wipe" || back.WallpaperTransitionMS != 250 {
		t.Fatalf("transición perdida: %q %d", back.WallpaperTransition, back.WallpaperTransitionMS)
	}
	if len(back.WallpaperWorkspaces) != 2 || back.WallpaperWorkspaces["chat de tarde"] != "image:~/a.png" {
		t.Fatalf("reglas perdidas: %v", back.WallpaperWorkspaces)
	}
}
