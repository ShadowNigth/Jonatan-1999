package ricer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

func TestGenerateSwayAndSwayFX(t *testing.T) {
	store := config.NewStore("")
	theme := store.ActiveTheme()
	settings := store.Settings()

	// 1. Default SwayFX bundle
	bundleFX := Generate(theme, settings)
	if len(bundleFX.Files) < 10 {
		t.Fatalf("expected >= 10 generated files, got %d", len(bundleFX.Files))
	}
	swayFX := bundleFX.Files["swayfx/config"]
	if !strings.Contains(swayFX, "corner_radius 12") || !strings.Contains(swayFX, `layer_effects "waybar"`) {
		t.Fatalf("swayfx/config missing SwayFX eye-candy directives: %s", swayFX)
	}
	wb := bundleFX.Files["waybar/config.jsonc"]
	if !strings.Contains(wb, `"sway/workspaces"`) {
		t.Fatalf("waybar/config.jsonc should use sway/workspaces in SwayFX mode")
	}

	// 2. Pure Sway bundle (no swayfx-effects include in sway/config)
	settings.Compositor = "sway"
	bundleSway := Generate(theme, settings)
	swayPure := bundleSway.Files["sway/config"]
	if strings.Contains(swayPure, "include ~/.config/sway/swayfx-effects.conf") {
		t.Fatalf("pure sway/config must not include swayfx-effects.conf")
	}
	if !strings.Contains(swayPure, "gaps inner 8") {
		t.Fatalf("sway/config missing gaps inner 8")
	}

	tmp := t.TempDir()
	written, err := ExportToDir(tmp, theme, settings)
	if err != nil {
		t.Fatalf("ExportToDir failed: %v", err)
	}
	if len(written) != len(bundleSway.Files) {
		t.Fatalf("expected %d written files, got %d", len(bundleSway.Files), len(written))
	}
	if _, err := os.Stat(filepath.Join(tmp, "sway", "config")); err != nil {
		t.Fatalf("expected sway/config on disk: %v", err)
	}
}
