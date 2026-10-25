package main

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
)

func TestScaleForDefaultsToCompactOne(t *testing.T) {
	t.Setenv("SWAYDESK_OSD_SCALE", "")
	cfg := config.Default()
	if got := scaleFor(cfg, ""); got != 1 {
		t.Fatalf("escala automática del OSD = %d, quería 1", got)
	}

	cfg.OSDFontScale = 2
	if got := scaleFor(cfg, ""); got != 2 {
		t.Fatalf("escala configurada del OSD = %d, quería 2", got)
	}
	if got := scaleFor(cfg, "1"); got != 1 {
		t.Fatalf("la escala explícita debe prevalecer: %d", got)
	}
}
