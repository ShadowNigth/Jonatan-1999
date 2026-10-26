package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockSettingsDefaultAndRoundTrip(t *testing.T) {
	cfg := Default()
	if cfg.LockBackground != "current" || cfg.LockTimeFormat != "%H:%M" ||
		cfg.LockDateFormat != "%A %d de %B" || cfg.LockClockSize != 48 ||
		cfg.LockIndicatorRadius != 148 || cfg.LockIndicatorThickness != 7 {
		t.Fatalf("valores iniciales del bloqueo inesperados: %+v", cfg)
	}

	cfg.LockBackground = "scene:petals"
	cfg.LockTimeFormat = "%I:%M %p"
	cfg.LockDateFormat = "%d/%m/%Y"
	cfg.LockClockSize = 42
	cfg.LockIndicatorRadius = 136
	cfg.LockIndicatorThickness = 6
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(DefaultFile(cfg)), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if loaded.LockBackground != cfg.LockBackground || loaded.LockTimeFormat != cfg.LockTimeFormat ||
		loaded.LockDateFormat != cfg.LockDateFormat || loaded.LockClockSize != cfg.LockClockSize ||
		loaded.LockIndicatorRadius != cfg.LockIndicatorRadius || loaded.LockIndicatorThickness != cfg.LockIndicatorThickness {
		t.Fatalf("opciones de bloqueo no sobrevivieron a la ida y vuelta:\nwant=%+v\ngot=%+v", cfg, loaded)
	}
}
