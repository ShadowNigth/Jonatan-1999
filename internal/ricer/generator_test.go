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

	// 1. Default SwayFX Visual-Twin bundle
	bundleFX := Generate(theme, settings)
	if len(bundleFX.Files) != 17 {
		t.Fatalf("expected 17 generated visual-twin dotfiles, got %d", len(bundleFX.Files))
	}
	swayFX := bundleFX.Files["swayfx/config"]
	if !strings.Contains(swayFX, "corner_radius 12") ||
		!strings.Contains(swayFX, `layer_effects "waybar"`) ||
		!strings.Contains(swayFX, `layer_effects "gtk-layer-shell"`) {
		t.Fatalf("swayfx/config missing SwayFX eye-candy directives: %s", swayFX)
	}
	wb := bundleFX.Files["waybar/config.jsonc"]
	if !strings.Contains(wb, `"sway/workspaces"`) || !strings.Contains(wb, `"一"`) {
		t.Fatalf("waybar/config.jsonc should use sway/workspaces with Kanji pills in SwayFX mode")
	}
	if _, ok := bundleFX.Files["nwg-dock/style.css"]; !ok {
		t.Fatalf("expected nwg-dock/style.css for floating Anime Dock")
	}
	if _, ok := bundleFX.Files["gtk-4.0/gtk.css"]; !ok {
		t.Fatalf("expected gtk-4.0/gtk.css for GTK4 anime styling")
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
	dockScriptInfo, err := os.Stat(filepath.Join(tmp, "sway", "start-dock.sh"))
	if err != nil {
		t.Fatalf("expected sway/start-dock.sh on disk: %v", err)
	}
	if dockScriptInfo.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected sway/start-dock.sh to be executable, got mode %v", dockScriptInfo.Mode())
	}
}
