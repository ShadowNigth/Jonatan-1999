package ricer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

func TestGenerateAndExport(t *testing.T) {
	store := config.NewStore("")
	theme := store.ActiveTheme()
	settings := store.Settings()

	bundle := Generate(theme, settings)
	if len(bundle.Files) < 6 {
		t.Fatalf("expected at least 6 generated files, got %d", len(bundle.Files))
	}
	hypr := bundle.Files["hypr/hyprland.conf"]
	if !strings.Contains(hypr, "gaps_in = 8") {
		t.Fatalf("hyprland.conf missing gaps_in: %s", hypr)
	}

	tmp := t.TempDir()
	written, err := ExportToDir(tmp, theme, settings)
	if err != nil {
		t.Fatalf("ExportToDir failed: %v", err)
	}
	if len(written) != len(bundle.Files) {
		t.Fatalf("expected %d written files, got %d", len(bundle.Files), len(written))
	}
	if _, err := os.Stat(filepath.Join(tmp, "waybar", "style.css")); err != nil {
		t.Fatalf("expected waybar/style.css on disk: %v", err)
	}
}
