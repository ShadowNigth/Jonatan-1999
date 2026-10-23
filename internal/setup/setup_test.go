package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
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
	for _, required := range []string{"sway/config", "waybar/config.json", "waybar/style.css", "foot/foot.ini", "mako/config", "swaylock/config", "swaydesk/config.toml", "applications/swaydesk-menu.desktop"} {
		if _, ok := files[required]; !ok {
			t.Fatalf("falta la plantilla %q", required)
		}
	}
}

func TestRenderPaleta(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "gruvbox"
	files, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	style := files["waybar/style.css"]
	if !strings.Contains(style, "#282828") || !strings.Contains(style, "#fabd2f") {
		t.Fatalf("los colores de gruvbox no aparecen en el CSS:\n%s", style[:400])
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

func TestApplySinArchivos(t *testing.T) {
	cfg := config.Default()
	dest := t.TempDir()
	results, err := Apply(cfg, dest)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("no debía actualizar nada, actualizó %d", len(results))
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
