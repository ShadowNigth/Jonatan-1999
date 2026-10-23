package ricer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

func TestGenerateKitsuneGoWayland(t *testing.T) {
	store := config.NewStore("")
	theme := store.ActiveTheme()
	settings := store.Settings()

	bundle := Generate(theme, settings)
	if bundle.Engine != "kitsune-go-wayland" {
		t.Fatalf("expected engine 'kitsune-go-wayland', got %q", bundle.Engine)
	}
	if len(bundle.Files) != 11 {
		t.Fatalf("expected 11 Kitsune Go Wayland dotfiles, got %d", len(bundle.Files))
	}

	kw := bundle.Files["kitsune-wayland/config.toml"]
	if !strings.Contains(kw, `default_layout = "tabbed"`) || !strings.Contains(kw, `dock_enabled = false`) {
		t.Fatalf("kitsune-wayland/config.toml missing tabbed/no-dock settings: %s", kw)
	}

	// Ensure zero Sway / SwayFX / Hyprland files exist
	for k := range bundle.Files {
		if strings.HasPrefix(k, "sway") || strings.HasPrefix(k, "hypr") || strings.HasPrefix(k, "nwg-dock") {
			t.Fatalf("unexpected legacy compositor/dock file generated: %s", k)
		}
	}

	// Ensure Btop++, Yazi, Neovim, Waybar, and Rofi configs exist
	required := []string{
		"kitsune-wayland/waybar.jsonc",
		"kitsune-wayland/waybar.css",
		"rofi/config.rasi",
		"btop/themes/kitsune.theme",
		"yazi/theme.toml",
		"nvim/init.lua",
		"kitty/kitty.conf",
	}
	for _, k := range required {
		if _, ok := bundle.Files[k]; !ok {
			t.Fatalf("missing required file %s", k)
		}
	}

	tmp := t.TempDir()
	written, err := ExportToDir(tmp, theme, settings)
	if err != nil {
		t.Fatalf("ExportToDir failed: %v", err)
	}
	if len(written) != len(bundle.Files) {
		t.Fatalf("expected %d written files, got %d", len(bundle.Files), len(written))
	}
	if _, err := os.Stat(filepath.Join(tmp, "kitsune-wayland", "config.toml")); err != nil {
		t.Fatalf("expected kitsune-wayland/config.toml on disk: %v", err)
	}
}
