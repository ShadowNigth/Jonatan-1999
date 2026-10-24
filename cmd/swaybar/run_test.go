package main

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func TestPowerMenuInlineToggleAndConfirmation(t *testing.T) {
	cfg := config.Default()
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	state := powerMenuState{}

	if !state.handle(cfg, theme, "power") || !state.open {
		t.Fatal("al pulsar PWR debe desplegar el menú dentro de la barra")
	}
	if !state.handle(cfg, theme, "power:reboot") || state.pending != "reboot" {
		t.Fatal("reiniciar debe pedir confirmación con la configuración predeterminada")
	}
	if !state.handle(cfg, theme, "power:cancel") || state.pending != "" || !state.open {
		t.Fatal("cancelar debe cerrar la confirmación y dejar abierto el menú")
	}
	if !state.handle(cfg, theme, "power:close") || state.open {
		t.Fatal("cerrar debe plegar el menú")
	}
}
