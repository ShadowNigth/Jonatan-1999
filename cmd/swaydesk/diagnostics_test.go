package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
)

func TestAutostartStatusDetectsOldStartupScript(t *testing.T) {
	configHome := t.TempDir()
	cacheHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	swayDir := filepath.Join(configHome, "sway")
	startupDir := filepath.Join(configHome, "swaydesk")
	if err := os.MkdirAll(swayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(startupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(swayDir, "config"), []byte("exec_always ~/.config/swaydesk/autostart.sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	startup := filepath.Join(startupDir, "autostart.sh")
	if err := os.WriteFile(startup, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	ok, detail := autostartStatus(config.Default())
	if ok || !strings.Contains(detail, "autostart antiguo") || !strings.Contains(detail, "notificaciones") {
		t.Fatalf("autostart viejo no detectado: ok=%v detalle=%q", ok, detail)
	}
}

func TestAutostartStatusAcceptsCurrentScript(t *testing.T) {
	configHome := t.TempDir()
	cacheHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	swayDir := filepath.Join(configHome, "sway")
	startupDir := filepath.Join(configHome, "swaydesk")
	if err := os.MkdirAll(swayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(startupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(swayDir, "config"), []byte("exec_always ~/.config/swaydesk/autostart.sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	current := "#!/bin/sh\nquiet swaydesk boot\nstart_bar swaybar\nstart_bar waybar\nonce swaynotify daemon\nonce swayosd watch\nquiet swaydesk wallpaper daemon --if-needed\n"
	if err := os.WriteFile(filepath.Join(startupDir, "autostart.sh"), []byte(current), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Cache = filepath.Join(cacheHome, "swaydesk")
	ok, detail := autostartStatus(cfg)
	if !ok || !strings.Contains(detail, filepath.Join(cfg.Cache, "autostart.log")) {
		t.Fatalf("autostart actual no detectado: ok=%v detalle=%q", ok, detail)
	}
}
