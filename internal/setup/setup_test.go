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
