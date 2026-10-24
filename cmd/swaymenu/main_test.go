package main

import (
	"path/filepath"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func TestPowerMenuUsesFloatingTerminalOutsideTTY(t *testing.T) {
	for _, tc := range []struct {
		name        string
		forceFloat  bool
		noFloat     bool
		interactive bool
		want        bool
	}{
		{name: "barra sin TTY", want: true},
		{name: "terminal directa", interactive: true, want: false},
		{name: "forzar flotante", forceFloat: true, interactive: true, want: true},
		{name: "hijo en terminal no vuelve a relanzarse", noFloat: true, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := powerMenuNeedsTerminal(tc.forceFloat, tc.noFloat, tc.interactive); got != tc.want {
				t.Fatalf("powerMenuNeedsTerminal() = %v, esperado %v", got, tc.want)
			}
		})
	}
}

func TestShellJoinQuotesSwayExecArguments(t *testing.T) {
	got := launcher.ShellQuoteArgs([]string{"/usr/bin/foot", "--app-id=swaydesk-menu", "/tmp/my menu", "a'b"})
	want := `'/usr/bin/foot' '--app-id=swaydesk-menu' '/tmp/my menu' 'a'\''b'`
	if got != want {
		t.Fatalf("shellJoin() = %q, esperado %q", got, want)
	}
}

func TestBuildItemsOmitsPowerAndLockActions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(home, "share"))
	t.Setenv("PATH", filepath.Join(home, "bin"))

	cfg := config.Default()
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	items, _ := buildItems(cfg, theme, false)
	foundPower, foundLock, foundTheme := false, false, false
	for _, item := range items {
		switch item.Key {
		case "act:power":
			foundPower = true
		case "act:lock":
			foundLock = true
		case "act:theme":
			foundTheme = true
		}
	}
	if foundPower || foundLock {
		t.Fatalf("el lanzador no debe listar energía ni bloqueo: power=%v lock=%v", foundPower, foundLock)
	}
	if foundTheme {
		t.Fatal("el selector de temas no debe aparecer en el lanzador de aplicaciones")
	}
}
