package main

import (
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/setup"
)

func TestThunarOpacityRules(t *testing.T) {
	want := map[string]bool{
		"thunar": false, // app_id de Thunar nativo en Wayland
		"Thunar": true,  // clase de ventana sobre XWayland
	}
	for _, preset := range opacityPresets {
		if isClass, ok := want[preset.Match]; ok {
			if preset.Class != isClass {
				t.Errorf("el preset %q debe usar Class=%t", preset.Match, isClass)
			}
			delete(want, preset.Match)
			if preset.Default != 0.94 {
				t.Errorf("el preset %q debe usar la opacidad predeterminada 0.94, obtuvo %.2f", preset.Match, preset.Default)
			}
		}
	}
	if len(want) > 0 {
		t.Errorf("faltan presets de Thunar: %v", want)
	}

	files, err := setup.Render(config.Default())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	swayConfig := files["sway/config"]
	for _, rule := range []string{
		`for_window [app_id="thunar"] opacity`,
		`for_window [class="Thunar"] opacity`,
	} {
		if !strings.Contains(swayConfig, rule) {
			t.Errorf("falta la regla de opacidad de Thunar %q", rule)
		}
	}
}
