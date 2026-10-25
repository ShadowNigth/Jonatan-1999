package wallpaper

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

func TestAllActiveOutputsFullscreen(t *testing.T) {
	outputs := []swayipc.Output{
		{Name: "DP-1", Active: true},
		{Name: "HDMI-A-1", Active: true},
	}
	workspaces := []swayipc.Workspace{
		{Name: "1:web", Output: "DP-1", Visible: true},
		{Name: "2:chat", Output: "HDMI-A-1", Visible: true},
	}
	tree := &swayipc.Node{Type: "root", Nodes: []*swayipc.Node{
		{Type: "output", Name: "DP-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "1:web", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 1},
			}},
		}},
		{Type: "output", Name: "HDMI-A-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "2:chat", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 1},
			}},
		}},
	}}
	if !allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("debe pausar cuando todas las salidas activas están cubiertas")
	}

	tree.Nodes[1].Nodes[0].Nodes[0].FullscreenMode = 0
	if allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("no debe pausar si queda una salida con el fondo visible")
	}
}

func TestAllActiveOutputsFullscreenIgnoresHiddenWorkspace(t *testing.T) {
	outputs := []swayipc.Output{{Name: "DP-1", Active: true}}
	workspaces := []swayipc.Workspace{{Name: "2:web", Output: "DP-1", Visible: true}}
	tree := &swayipc.Node{Type: "root", Nodes: []*swayipc.Node{
		{Type: "output", Name: "DP-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "1:hidden", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 1},
			}},
			{Type: "workspace", Name: "2:web", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 0},
			}},
		}},
	}}
	if allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("un fullscreen en un workspace no visible no debe pausar el fondo")
	}
}

func TestGlobalFullscreenCoversEveryActiveOutput(t *testing.T) {
	outputs := []swayipc.Output{
		{Name: "DP-1", Active: true},
		{Name: "HDMI-A-1", Active: true},
	}
	workspaces := []swayipc.Workspace{{Name: "1:web", Output: "DP-1", Visible: true}}
	tree := &swayipc.Node{Type: "root", Nodes: []*swayipc.Node{
		{Type: "output", Name: "DP-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "1:web", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 2},
			}},
		}},
	}}
	if !allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("fullscreen global debe cubrir todas las salidas")
	}
}

func TestRecognizesCommonScreenLockers(t *testing.T) {
	for _, name := range []string{"swaylock", "/usr/bin/swaylock-effects", "swaylock-effect", "gtklock", "waylock", "hyprlock", "i3lock", "i3lock-color"} {
		if !isScreenLockerProcess(name) {
			t.Errorf("no se reconoció el bloqueador %q", name)
		}
	}
	for _, name := range []string{"sway", "chromium", "swaydesk"} {
		if isScreenLockerProcess(name) {
			t.Errorf("se reconoció erróneamente %q como bloqueador", name)
		}
	}
}
