package main

import (
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/wayland"
)

func TestAnchorsFor(t *testing.T) {
	cases := []struct {
		pos     string
		anchors uint32
		mx, my  int
	}{
		{"top-right", wayland.AnchorTop | wayland.AnchorRight, 14, 14},
		{"top-left", wayland.AnchorTop | wayland.AnchorLeft, 14, 14},
		{"bottom-right", wayland.AnchorBottom | wayland.AnchorRight, 14, 14},
		{"bottom-left", wayland.AnchorBottom | wayland.AnchorLeft, 14, 14},
		{"loquesea", wayland.AnchorTop | wayland.AnchorRight, 14, 14},
	}
	for _, c := range cases {
		a, mx, my := anchorsFor(c.pos)
		if a != c.anchors || mx != c.mx || my != c.my {
			t.Errorf("anchorsFor(%q) = %d, %d, %d", c.pos, a, mx, my)
		}
	}
}

func TestParseFlags(t *testing.T) {
	vals, pos := parseFlags([]string{
		"--app", "pacman", "--urgency=urgente", "--icon", "x.png",
		"--action", "default=Abrir", "--action", "later", "--timeout", "-1",
		"Resumen", "cuerpo",
	}, map[string]bool{"action": true})
	if last(vals, "app") != "pacman" {
		t.Errorf("app: %q", last(vals, "app"))
	}
	if last(vals, "urgency") != "urgente" {
		t.Errorf("urgency: %q", last(vals, "urgency"))
	}
	if len(vals["action"]) != 2 || vals["action"][0] != "default=Abrir" {
		t.Errorf("actions: %#v", vals["action"])
	}
	if last(vals, "timeout") != "-1" {
		t.Errorf("timeout: %q", last(vals, "timeout"))
	}
	if len(pos) != 2 || pos[0] != "Resumen" || pos[1] != "cuerpo" {
		t.Errorf("posicionales: %#v", pos)
	}
}

func TestAtoi(t *testing.T) {
	cases := map[string]int{"0": 0, "42": 42, "-1": -1, "6000": 6000}
	for in, want := range cases {
		got, err := atoi(in)
		if err != nil || got != want {
			t.Errorf("atoi(%q) = %d, %v", in, got, err)
		}
	}
	if _, err := atoi("12a"); err == nil {
		t.Error("se esperaba error con un valor no numérico")
	}
}

func TestItemsKeyIgnoresNothing(t *testing.T) {
	now := time.Now()
	a := []notify.Notification{{ID: 1, Summary: "uno", Progress: -1}}
	b := []notify.Notification{{ID: 1, Summary: "dos", Progress: -1}}
	if itemsKey(a, now) == itemsKey(b, now) {
		t.Error("un resumen distinto debe cambiar la clave")
	}
	// La clave cambia con el segundo (la cuenta atrás avanza).
	if itemsKey(a, now) == itemsKey(a, now.Add(1100*time.Millisecond)) {
		t.Error("la cuenta atrás debería repintar cada segundo")
	}
	if len(a) > 0 && itemsKey(nil, now) == itemsKey(a, now) {
		t.Error("sin tarjetas la clave debe ser distinta")
	}
}
