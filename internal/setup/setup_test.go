package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func TestRenderSinMarcadores(t *testing.T) {
	cfg := config.Default()
	files, err := Render(cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(files) < 8 {
		t.Fatalf("se esperaban al menos 8 plantillas, hay %d", len(files))
	}
	for name, content := range files {
		if strings.Contains(content, "{{") && !strings.Contains(name, "config.d") {
			idx := strings.Index(content, "{{")
			end := idx + 40
			if end > len(content) {
				end = len(content)
			}
			t.Fatalf("%s tiene marcadores sin sustituir: %q", name, content[idx:end])
		}
	}
	for _, required := range []string{"sway/config", "waybar/config.json", "waybar/style.css", "foot/foot.ini", "mako/config", "swaylock/config", "gtk-3.0/settings.ini", "gtk-3.0/gtk.css", "swaydesk/config.toml", "swaydesk/autostart.sh", "applications/swaydesk-menu.desktop"} {
		if _, ok := files[required]; !ok {
			t.Fatalf("falta la plantilla %q", required)
		}
	}
	for _, service := range []string{"quiet swaydesk boot", "start_bar swaybar", "start_bar waybar", `--config "$WAYBAR_CONFIG"`, "waybar_uses_generated_config", "once swaynotify daemon", "once swayosd watch", "swaydesk wallpaper daemon --if-needed", "autostart.log"} {
		if !strings.Contains(files["swaydesk/autostart.sh"], service) {
			t.Errorf("el autostart debe diagnosticar el servicio %q", service)
		}
	}
	for _, provider := range []string{"stop_exact waybar", "stop_exact swaybar", "stop_exact mako", "stop_exact dunst", "swaynotify status"} {
		if !strings.Contains(files["swaydesk/autostart.sh"], provider) {
			t.Errorf("el autostart debe seleccionar un solo proveedor: falta %q", provider)
		}
	}
}

func TestRenderPaleta(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "gruvbox"
	cfg.BarOpacity = 0.68
	files, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	style := files["waybar/style.css"]
	if !strings.Contains(style, "#282828") || !strings.Contains(style, "#fabd2f") {
		t.Fatalf("los colores de gruvbox no aparecen en el CSS:\n%s", style[:400])
	}
	if !strings.Contains(style, "background: alpha(@bg, 0.68)") || !strings.Contains(style, "border: 1px solid alpha(@border, 0.68)") {
		t.Fatalf("Waybar debe respetar la opacidad configurada en fondo y borde:\n%s", style)
	}
	if !strings.Contains(style, "Gruvbox Dark") {
		t.Fatal("el nombre del tema no aparece en el CSS")
	}
	swayConf := files["sway/config"]
	if !strings.Contains(swayConf, "bindsym $mod+Return exec") {
		t.Fatal("faltan atajos en la configuración de sway")
	}
	if strings.Contains(swayConf, "{{") {
		t.Fatal("quedan marcadores en la configuración de sway")
	}
	if !strings.Contains(swayConf, `mode "redimensionar"`) || strings.Contains(swayConf, `mode "resize"`) {
		t.Fatal("el modo indicado en Waybar debe tener nombre en español")
	}
}

func TestWaybarBackgroundShowsWallpaperByDefault(t *testing.T) {
	files, err := Render(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	style := files["waybar/style.css"]
	for _, want := range []string{
		"background: alpha(@bg, 0.00);",
		"border: 1px solid alpha(@border, 0.00);",
		"tooltip {\n    background: alpha(@bg, 0.94);",
	} {
		if !strings.Contains(style, want) {
			t.Errorf("la hoja de Waybar debe incluir %q para dejar ver el wallpaper y conservar tooltips legibles", want)
		}
	}
}

func TestWaybarCentersClockAndDateInsteadOfWindowTitle(t *testing.T) {
	files, err := Render(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	var waybar struct {
		ModulesCenter []string `json:"modules-center"`
		ModulesRight  []string `json:"modules-right"`
		Clock         struct {
			Format        string `json:"format"`
			FormatAlt     string `json:"format-alt"`
			Tooltip       bool   `json:"tooltip"`
			TooltipFormat string `json:"tooltip-format"`
			Locale        string `json:"locale"`
			Calendar      struct {
				Mode           string `json:"mode"`
				FirstDayOfWeek int    `json:"first-day-of-week"`
				OnScroll       int    `json:"on-scroll"`
			} `json:"calendar"`
			Actions struct {
				OnScrollUp   string `json:"on-scroll-up"`
				OnScrollDown string `json:"on-scroll-down"`
			} `json:"actions"`
		} `json:"clock"`
	}
	if err := json.Unmarshal([]byte(files["waybar/config.json"]), &waybar); err != nil {
		t.Fatalf("configuración Waybar no válida: %v", err)
	}
	if len(waybar.ModulesCenter) != 1 || waybar.ModulesCenter[0] != "clock" {
		t.Fatalf("el centro debe contener solo el reloj, obtuvo %v", waybar.ModulesCenter)
	}
	for _, module := range waybar.ModulesRight {
		if module == "clock" || module == "sway/window" {
			t.Fatalf("módulo duplicado o título de ventana en la derecha: %v", waybar.ModulesRight)
		}
	}
	if strings.Contains(files["waybar/config.json"], "sway/window") {
		t.Fatal("la configuración aún contiene el módulo del título de ventana")
	}
	if !strings.Contains(waybar.Clock.Format, "{:L%H:%M") || !strings.Contains(waybar.Clock.Format, "%A %d/%m/%Y") {
		t.Fatalf("el reloj debe mostrar hora y día/fecha con locale español: %q", waybar.Clock.Format)
	}
	if waybar.Clock.FormatAlt != "" {
		t.Fatal("al pulsar el reloj no debe cambiar a un formato alternativo")
	}
	if strings.Contains(waybar.Clock.Format, "ICON_CLOCK") {
		t.Fatal("el reloj no debe llevar un icono delante de la hora")
	}
	if !strings.Contains(files["waybar/style.css"], "#clock {\n    font-size: 15px;") {
		t.Fatal("el reloj central debe tener un tamaño de texto algo mayor")
	}
	if !waybar.Clock.Tooltip || !strings.Contains(waybar.Clock.TooltipFormat, "{calendar}") {
		t.Fatal("al pasar el puntero por el reloj debe mostrarse el calendario")
	}
	if !strings.Contains(waybar.Clock.TooltipFormat, "size='large'") {
		t.Fatal("el calendario del tooltip debe tener texto ampliado")
	}
	if waybar.Clock.Locale != "es_AR.UTF-8" || waybar.Clock.Calendar.Mode != "month" || waybar.Clock.Calendar.FirstDayOfWeek != 1 {
		t.Fatalf("calendario debe quedar en español y empezar en lunes: %+v", waybar.Clock)
	}
	if waybar.Clock.Calendar.OnScroll != 1 || waybar.Clock.Actions.OnScrollUp != "shift_up" || waybar.Clock.Actions.OnScrollDown != "shift_down" {
		t.Fatalf("la rueda debe cambiar el mes de uno en uno: %+v", waybar.Clock)
	}
}

func TestWaybarLabelsAreSpanish(t *testing.T) {
	files, err := Render(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	waybarConfig := files["waybar/config.json"]
	for _, label := range []string{
		"Wi-Fi:",
		"Sin conexión",
		"Silenciado",
		"Volumen: {volume}%",
		"Uso de CPU:",
		"Memoria usada:",
		"Temperatura:",
		"Batería:",
	} {
		if !strings.Contains(waybarConfig, label) {
			t.Errorf("falta etiqueta en español %q", label)
		}
	}
	if strings.Contains(waybarConfig, "sway/window") {
		t.Fatal("el título de ventana no debe aparecer en Waybar")
	}
}

func TestRenderGTKThemeFollowsPalette(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "gruvbox"
	cfg.GTKTheme = "Adwaita-dark"
	cfg.GTKIconTheme = "Papirus-Dark"
	files, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	settings := files["gtk-3.0/settings.ini"]
	if !strings.Contains(settings, "gtk-theme-name=Adwaita-dark") ||
		!strings.Contains(settings, "gtk-icon-theme-name=Papirus-Dark") ||
		!strings.Contains(settings, "gtk-application-prefer-dark-theme=1") {
		t.Fatalf("configuración GTK inesperada:\n%s", settings)
	}
	css := files["gtk-3.0/gtk.css"]
	if !strings.Contains(css, "#282828") || !strings.Contains(css, "#83a598") {
		t.Fatalf("la paleta Gruvbox no llegó al CSS GTK:\n%s", css)
	}
	if !strings.Contains(css, gtkCSSStart) || !strings.Contains(css, gtkCSSEnd) {
		t.Fatal("la paleta GTK debe quedar delimitada para conservar CSS ajeno")
	}
	if strings.Contains(settings+css, "{{") {
		t.Fatal("quedaron marcadores GTK sin sustituir")
	}
}

func TestGTKAppearanceAutoPrefersInstalledThemes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, path := range []string{
		filepath.Join(home, ".themes", "Adwaita-dark", "gtk-3.0", "gtk.css"),
		filepath.Join(home, ".themes", "Adwaita", "gtk-3.0", "gtk.css"),
		filepath.Join(home, ".local", "share", "icons", "Papirus-Dark", "index.theme"),
		filepath.Join(home, ".local", "share", "icons", "Papirus", "index.theme"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name, theme, wantTheme, wantIcons, wantDark string
	}{
		{"oscuro", "dracula", "Adwaita-dark", "Papirus-Dark", "1"},
		{"claro", "rei", "Adwaita", "Papirus", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Theme = tc.theme
			cfg.GTKTheme = "auto"
			cfg.GTKIconTheme = "auto"
			theme, icons, preferDark := gtkAppearance(cfg, ui.PaletteByName(cfg.Theme))
			if theme != tc.wantTheme || icons != tc.wantIcons || preferDark != tc.wantDark {
				t.Fatalf("gtkAppearance() = (%q, %q, %q), esperado (%q, %q, %q)",
					theme, icons, preferDark, tc.wantTheme, tc.wantIcons, tc.wantDark)
			}
		})
	}
}

func TestMergeGTKSettingsPreservesUserPreferences(t *testing.T) {
	generated := "[Settings]\n" +
		"gtk-theme-name=Adwaita-dark\n" +
		"gtk-icon-theme-name=Papirus-Dark\n" +
		"gtk-application-prefer-dark-theme=1\n" +
		"gtk-enable-animations=1\n"
	existing := "# Preferencias propias\n[Settings]\n" +
		"gtk-theme-name=OldTheme\n" +
		"gtk-icon-theme-name=OldIcons\n" +
		"gtk-application-prefer-dark-theme=0\n" +
		"gtk-enable-animations=0\n" +
		"gtk-cursor-theme-name=MyCursor\n" +
		"[Other]\nkeep=this\n"

	got := mergeGTKSettings(existing, generated)
	for _, want := range []string{
		"gtk-theme-name=Adwaita-dark",
		"gtk-icon-theme-name=Papirus-Dark",
		"gtk-application-prefer-dark-theme=1",
		"gtk-enable-animations=0",
		"gtk-cursor-theme-name=MyCursor",
		"[Other]\nkeep=this",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("mergeGTKSettings perdió o no actualizó %q:\n%s", want, got)
		}
	}
	if count := strings.Count(got, "gtk-theme-name="); count != 1 {
		t.Errorf("se duplicó gtk-theme-name (%d):\n%s", count, got)
	}
	if again := mergeGTKSettings(got, generated); again != got {
		t.Errorf("la fusión GTK no es idempotente:\n%s\n--- segunda aplicación ---\n%s", got, again)
	}
}

func TestMergeGTKCSSPreservesUserRules(t *testing.T) {
	generated := "/* BEGIN SWAYDESK GTK PALETTE */\n" +
		"@define-color swaydesk_bg #101010;\n" +
		"/* END SWAYDESK GTK PALETTE */"
	existing := "/* CSS propio */\n.user-widget { color: red; }\n\n" +
		"/* BEGIN SWAYDESK GTK PALETTE */\n" +
		"@define-color swaydesk_bg #000000;\n" +
		"/* END SWAYDESK GTK PALETTE */\n\n" +
		".another-user-widget { opacity: 0.9; }\n"

	got := mergeGTKCSS(existing, generated)
	for _, want := range []string{
		"/* CSS propio */",
		".user-widget { color: red; }",
		"@define-color swaydesk_bg #101010;",
		".another-user-widget { opacity: 0.9; }",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("mergeGTKCSS perdió o no actualizó %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "@define-color swaydesk_bg #000000;") {
		t.Errorf("sobrevive la paleta GTK antigua:\n%s", got)
	}
	if count := strings.Count(got, gtkCSSStart); count != 1 {
		t.Errorf("hay %d bloques de paleta swaydesk, esperado 1:\n%s", count, got)
	}
	if again := mergeGTKCSS(got, generated); again != got {
		t.Errorf("la fusión CSS no es idempotente:\n%s\n--- segunda aplicación ---\n%s", got, again)
	}
}

func TestRenderNerdFontToggle(t *testing.T) {
	cfg := config.Default()
	cfg.NerdFont = false
	files, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(files["waybar/config.json"], "\uf269") {
		t.Fatal("con nerd_font=false no deben aparecer glifos Nerd Font")
	}
	cfg.NerdFont = true
	files, _ = Render(cfg)
	if !strings.Contains(files["waybar/config.json"], "\uf269") {
		t.Fatal("con nerd_font=true deben aparecer los glifos")
	}
}

func TestInstallYRespaldo(t *testing.T) {
	cfg := config.Default()
	dest := t.TempDir()

	results, err := Install(cfg, dest, false, []string{"sway"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("esperaba varios archivos, obtuve %d", len(results))
	}
	target := filepath.Join(dest, "sway", "config")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("no se escribió %s: %v", target, err)
	}

	// Segundo intento sin force: debe omitir.
	res, err := Install(cfg, dest, false, []string{"sway"})
	if err != nil {
		t.Fatal(err)
	}
	skipped := 0
	for _, r := range res {
		if strings.HasPrefix(r.Action, "omitido") {
			skipped++
		}
	}
	if skipped == 0 {
		t.Fatal("no se omitió ningún archivo existente")
	}

	// Con force: respaldo + sobrescritura.
	res, err = Install(cfg, dest, true, []string{"sway"})
	if err != nil {
		t.Fatal(err)
	}
	backups := 0
	for _, r := range res {
		if r.Action == "respaldo" {
			backups++
		}
	}
	if backups == 0 {
		t.Fatal("no se creó ningún respaldo")
	}
}

func TestApplyRemovesLegacyWaybarPowerOnly(t *testing.T) {
	cfg := config.Default()
	dest := t.TempDir()
	waybarDir := filepath.Join(dest, "waybar")
	if err := os.MkdirAll(waybarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(waybarDir, "config.json")
	legacy := `{"modules-right":["cpu","custom/power","clock"],"custom/power":{"on-click":"swaymenu --power"},"custom/keep":{"format":"still here"}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Apply(cfg, dest)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if strings.Contains(text, "custom/power") {
		t.Fatalf("no se quitó el módulo de energía: %s", text)
	}
	if !strings.Contains(text, `"cpu"`) || !strings.Contains(text, `"clock"`) || !strings.Contains(text, "custom/keep") {
		t.Fatalf("se perdió contenido ajeno de Waybar: %s", text)
	}
	found := false
	for _, result := range results {
		if result.Path == path && result.Action == "botón custom/power retirado" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Apply no reportó la migración de Waybar: %+v", results)
	}
}

func TestApplySinArchivos(t *testing.T) {
	cfg := config.Default()
	dest := t.TempDir()
	results, err := Apply(cfg, dest)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("solo debía inicializar los archivos GTK que faltaban, obtuvo %+v", results)
	}
	for _, r := range results {
		rel, err := filepath.Rel(dest, r.Path)
		if err != nil || !gtkThemeAssets[rel] {
			t.Fatalf("Apply escribió un componente no instalado: %s", r.Path)
		}
	}
}

func TestApplyCreaTemaGTKParaInstalacionesExistentes(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "dracula"
	dest := t.TempDir()
	results, err := Apply(cfg, dest)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Apply debía crear los dos archivos GTK, obtuvo %d resultados: %+v", len(results), results)
	}
	settings, err := os.ReadFile(filepath.Join(dest, "gtk-3.0", "settings.ini"))
	if err != nil || !strings.Contains(string(settings), "gtk-theme-name=") {
		t.Fatalf("no se creó la configuración GTK: %v", err)
	}
	css, err := os.ReadFile(filepath.Join(dest, "gtk-3.0", "gtk.css"))
	if err != nil {
		t.Fatalf("no se creó el CSS GTK: %v", err)
	}
	if !strings.Contains(string(css), "#282a36") || !strings.Contains(string(css), "#bd93f9") {
		t.Fatalf("el CSS no refleja la paleta activa:\n%s", css)
	}
	if _, err := os.Stat(filepath.Join(dest, "sway", "config")); !os.IsNotExist(err) {
		t.Fatal("Apply no debe generar sway/config al añadir solo el tema GTK")
	}
}

func TestApplyPreservesGTKCustomization(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "dracula"
	dest := t.TempDir()
	gtkDir := filepath.Join(dest, "gtk-3.0")
	if err := os.MkdirAll(gtkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(gtkDir, "settings.ini")
	cssPath := filepath.Join(gtkDir, "gtk.css")
	settings := "[Settings]\n" +
		"gtk-theme-name=MyTheme\n" +
		"gtk-icon-theme-name=MyIcons\n" +
		"gtk-application-prefer-dark-theme=0\n" +
		"gtk-enable-animations=0\n" +
		"gtk-cursor-theme-name=MyCursor\n" +
		"[Other]\nkeep=this\n"
	css := "/* Mi CSS personalizado */\n.user-widget { color: red; }\n"
	if err := os.WriteFile(settingsPath, []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cssPath, []byte(css), 0o644); err != nil {
		t.Fatal(err)
	}

	generated, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	results, err := Apply(cfg, dest)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Apply debía actualizar solo los dos archivos GTK existentes, obtuvo %+v", results)
	}
	gotSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range gtkSettings(generated["gtk-3.0/settings.ini"]) {
		if pair.key == "gtk-enable-animations" {
			continue
		}
		if !strings.Contains(string(gotSettings), pair.key+"="+pair.value) {
			t.Errorf("no se aplicó el ajuste generado %s=%s:\n%s", pair.key, pair.value, gotSettings)
		}
	}
	for _, preserved := range []string{
		"gtk-enable-animations=0",
		"gtk-cursor-theme-name=MyCursor",
		"[Other]\nkeep=this",
	} {
		if !strings.Contains(string(gotSettings), preserved) {
			t.Errorf("se perdió la preferencia GTK del usuario %q:\n%s", preserved, gotSettings)
		}
	}
	gotCSS, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, preserved := range []string{"/* Mi CSS personalizado */", ".user-widget { color: red; }"} {
		if !strings.Contains(string(gotCSS), preserved) {
			t.Errorf("se perdió CSS del usuario %q:\n%s", preserved, gotCSS)
		}
	}
	if !strings.Contains(string(gotCSS), gtkCSSStart) || !strings.Contains(string(gotCSS), gtkCSSEnd) {
		t.Errorf("no se añadió el bloque de paleta delimitado:\n%s", gotCSS)
	}
}

func TestDetectFont(t *testing.T) {
	if got := DetectFont("Mi Fuente", true); got != "Mi Fuente" {
		t.Fatalf("la fuente preferida debe respetarse, obtuve %q", got)
	}
	if got := DetectFont("", true); got == "" {
		t.Fatal("DetectFont nunca debe devolver vacío")
	}
}

// El render no debe dejar marcadores sin sustituir ni escribir directivas que
// solo entiende swayfx cuando no está instalado.
func TestRenderSwaySinSwayfx(t *testing.T) {
	cfg := config.Default()
	files, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sway := files["sway/config"]
	for _, marcador := range []string{"{{", "}}"} {
		if strings.Contains(sway, marcador) {
			t.Fatalf("marcador sin sustituir en sway/config: %s", marcador)
		}
	}
	swayfxOnly := []string{"default_border_color", "floating_border_color", "focused_border_color", "urgent_border_color"}
	for _, d := range swayfxOnly {
		if IsSwayfx() {
			continue
		}
		for _, line := range strings.Split(sway, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, d) {
				t.Fatalf("directiva de swayfx en sway puro: %q", line)
			}
		}
	}
	if !IsSwayfx() {
		if !strings.Contains(sway, "client.focused") || !strings.Contains(sway, "client.urgent") {
			t.Fatal("faltan los colores de borde client.* para sway puro")
		}
	}
}

// Ninguna tecla puede estar asignada dos veces: sway se queda con la última y
// el usuario pierde un atajo sin enterarse.
func TestSwayBindingsSinDuplicados(t *testing.T) {
	files, err := Render(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, line := range strings.Split(files["sway/config"], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "bindsym") {
			continue
		}
		campos := strings.Fields(line)
		if len(campos) < 3 {
			continue
		}
		tecla := campos[1]
		if previa, ok := seen[tecla]; ok {
			t.Fatalf("%s está asignada dos veces:\n  %s\n  %s", tecla, previa, line)
		}
		seen[tecla] = line
	}
	if len(seen) < 30 {
		t.Fatalf("solo se encontraron %d atajos", len(seen))
	}
}

// El pie de foot necesita colores RGB sin '#' y una sección que exista en la
// versión instalada.
func TestRenderFootColores(t *testing.T) {
	files, err := Render(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	foot := files["foot/foot.ini"]
	if strings.Contains(foot, "[colors]") && !strings.Contains(foot, "[colors-dark]") {
		t.Fatal("foot 1.26+ exige [colors-dark] en lugar de [colors]")
	}
	if !strings.Contains(foot, "[colors-dark]") && !strings.Contains(foot, "[colors]") {
		t.Fatal("falta la sección de colores")
	}
	for _, line := range strings.Split(foot, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		partes := strings.SplitN(line, "=", 2)
		clave := strings.TrimSpace(partes[0])
		valor := strings.TrimSpace(partes[1])
		esColor := clave == "background" || clave == "foreground" || clave == "cursor" ||
			clave == "selection-background" || clave == "selection-foreground" ||
			strings.HasPrefix(clave, "regular") || strings.HasPrefix(clave, "bright")
		if !esColor {
			continue
		}
		for _, v := range strings.Fields(valor) {
			if strings.HasPrefix(v, "#") {
				t.Fatalf("%s no admite '#': %s", clave, line)
			}
			if len(v) != 6 {
				t.Fatalf("%s: color inesperado %q", clave, v)
			}
		}
		if clave == "cursor" && len(strings.Fields(valor)) != 2 {
			t.Fatalf("cursor necesita dos colores: %s", line)
		}
	}
}

func TestFootConfigIssue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "foot"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "foot", "foot.ini")
	// Sin archivo no hay nada que reprochar.
	if issue := FootConfigIssue(); issue != "" {
		t.Fatalf("sin foot.ini: %q", issue)
	}
	// Colores con '#'.
	if err := os.WriteFile(path, []byte("[colors-dark]\nbackground=#1e1e2e\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if issue := FootConfigIssue(); !strings.Contains(issue, "background") {
		t.Fatalf("no se detectó el '#': %q", issue)
	}
	// Correcto.
	if err := os.WriteFile(path, []byte("[colors-dark]\nbackground=1e1e2e\ncursor=89b4fa 1e1e2e\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if issue := FootConfigIssue(); issue != "" {
		t.Fatalf("archivo válido: %q", issue)
	}
}

func TestSwayDuplicateBinding(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "sway"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sway", "config")
	ok := "bindsym $mod+d exec menu\nbindsym $mod+Return exec foot\n"
	if err := os.WriteFile(path, []byte(ok), 0o644); err != nil {
		t.Fatal(err)
	}
	if tecla := SwayDuplicateBinding(); tecla != "" {
		t.Fatalf("duplicado inesperado: %q", tecla)
	}
	dup := ok + "bindsym $mod+d exec otra\n"
	if err := os.WriteFile(path, []byte(dup), 0o644); err != nil {
		t.Fatal(err)
	}
	if tecla := SwayDuplicateBinding(); tecla != "$mod+d" {
		t.Fatalf("no se detectó el duplicado: %q", tecla)
	}
	// Los comentarios no cuentan.
	comentado := ok + "# bindsym $mod+d exec otra\n"
	if err := os.WriteFile(path, []byte(comentado), 0o644); err != nil {
		t.Fatal(err)
	}
	if tecla := SwayDuplicateBinding(); tecla != "" {
		t.Fatalf("un comentario no es un atajo: %q", tecla)
	}
}

func TestMatchOnlyNoArrastraPrefijos(t *testing.T) {
	only := map[string]bool{"sway": true}
	if !matchOnly("sway/config", only) {
		t.Fatal("sway/config debería entrar con --only sway")
	}
	if matchOnly("swaydesk/config.toml", only) {
		t.Fatal("--only sway no debe tocar swaydesk/config.toml")
	}
	if matchOnly("foot/foot.ini", only) {
		t.Fatal("--only sway no debe tocar foot")
	}
	if !matchOnly("foot/foot.ini", map[string]bool{"foot/foot.ini": true}) {
		t.Fatal("el archivo exacto debe entrar")
	}
}

// --only sway debe reescribir la configuración de sway sin tocar config.toml.
func TestInstallOnlySwayConservaConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	if _, err := Install(cfg, dir, false, nil); err != nil {
		t.Fatal(err)
	}
	propio := filepath.Join(dir, "swaydesk", "config.toml")
	if err := os.WriteFile(propio, []byte("# editado a mano\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Install(cfg, dir, true, []string{"sway"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(propio)
	if string(data) != "# editado a mano\n" {
		t.Fatal("config.toml se sobrescribió con --only sway")
	}
	encontrado := false
	for _, r := range res {
		if strings.HasSuffix(r.Path, filepath.Join("sway", "config")) {
			encontrado = true
		}
	}
	if !encontrado {
		t.Fatalf("no se reescribió sway/config: %+v", res)
	}
}

func TestPickProgram(t *testing.T) {
	// El valor configurado manda si existe (/bin/sh está siempre).
	if got := pickProgram("/bin/sh", []string{"nada"}); got != "/bin/sh" {
		t.Fatalf("debería conservar el configurado: %q", got)
	}
	// Si no existe, se cae al primer candidato disponible.
	dir := t.TempDir()
	fake := filepath.Join(dir, "mi-gestor")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := pickProgram("no-existe-jamas", []string{"tampoco", "mi-gestor"}); got != "mi-gestor" {
		t.Fatalf("no se eligió el candidato instalado: %q", got)
	}
	// Sin candidatos disponibles se conserva el valor (y keys lo señalará).
	if got := pickProgram("no-existe-jamas", []string{"tampoco"}); got != "no-existe-jamas" {
		t.Fatalf("valor = %q", got)
	}
	// Un valor con argumentos se comprueba por su binario.
	if got := pickProgram("/bin/sh -e ranger", nil); got != "/bin/sh -e ranger" {
		t.Fatalf("con argumentos: %q", got)
	}
	if binName("foot -e ranger") != "foot" || binName("") != "" {
		t.Fatal("binName mal")
	}
}

// Los respaldos dentro de config.d no deben volver a cargarse en sway.
func TestSwayIncludeSoloConf(t *testing.T) {
	files, err := Render(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	sway := files["sway/config"]
	if !strings.Contains(sway, "include ~/.config/sway/config.d/*.conf") {
		t.Fatal("el include debe filtrar *.conf para no cargar los .bak")
	}
	if strings.Contains(sway, "config.d/*\n") || strings.Contains(sway, "config.d/* ") {
		t.Fatal("queda un include sin filtro de extensión")
	}
}
