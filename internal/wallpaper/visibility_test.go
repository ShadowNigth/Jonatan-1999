package wallpaper

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

func TestAnyVisibleFullscreenPausesOnAnyOutput(t *testing.T) {
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
				{Type: "con", FullscreenMode: 0},
			}},
		}},
	}}
	if !anyVisibleFullscreen(outputs, workspaces, tree) {
		t.Fatal("un fullscreen visible debe pausar el fondo, aunque otra salida siga visible")
	}

	tree.Nodes[0].Nodes[0].Nodes[0].FullscreenMode = 0
	if anyVisibleFullscreen(outputs, workspaces, tree) {
		t.Fatal("sin ventanas visibles a pantalla completa no debe pausarse")
	}
}

func TestAnyVisibleFullscreenIgnoresHiddenWorkspace(t *testing.T) {
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
	if anyVisibleFullscreen(outputs, workspaces, tree) {
		t.Fatal("un fullscreen en un workspace no visible no debe pausar el fondo")
	}
}

func TestAnyVisibleGlobalFullscreenPauses(t *testing.T) {
	outputs := []swayipc.Output{{Name: "DP-1", Active: true}}
	workspaces := []swayipc.Workspace{{Name: "1:web", Output: "DP-1", Visible: true}}
	tree := &swayipc.Node{Type: "root", Nodes: []*swayipc.Node{
		{Type: "output", Name: "DP-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "1:web", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 2},
			}},
		}},
	}}
	if !anyVisibleFullscreen(outputs, workspaces, tree) {
		t.Fatal("fullscreen global debe pausar el fondo")
	}
}
