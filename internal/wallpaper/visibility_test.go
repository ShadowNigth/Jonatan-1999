package wallpaper

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

func TestAllActiveOutputsNeedFullscreenWindows(t *testing.T) {
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
	if allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("no debe pausar si otra salida aún muestra el fondo")
	}

	tree.Nodes[1].Nodes[0].Nodes[0].FullscreenMode = 1
	if !allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("debe pausar cuando todas las salidas activas están cubiertas")
	}

	tree.Nodes[0].Nodes[0].Nodes[0].FullscreenMode = 0
	if allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("no debe pausar si vuelve a quedar visible una salida")
	}
}

func TestAllActiveOutputsIgnoresHiddenWorkspace(t *testing.T) {
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

func TestFullscreenRequiresLeafWindow(t *testing.T) {
	outputs := []swayipc.Output{{Name: "DP-1", Active: true}}
	workspaces := []swayipc.Workspace{{Name: "1:web", Output: "DP-1", Visible: true}}
	tree := &swayipc.Node{Type: "root", Nodes: []*swayipc.Node{
		{Type: "output", Name: "DP-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "1:web", FullscreenMode: 1, Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 1, Nodes: []*swayipc.Node{
					{Type: "con", FullscreenMode: 0},
				}},
			}},
		}},
	}}
	if allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("un estado fullscreen en el workspace o en un contenedor no debe pausar el fondo")
	}
}

func TestAllActiveOutputsIgnoresInvisibleWindow(t *testing.T) {
	outputs := []swayipc.Output{{Name: "DP-1", Active: true}}
	workspaces := []swayipc.Workspace{{Name: "1:web", Output: "DP-1", Visible: true}}
	visible := false
	tree := &swayipc.Node{Type: "root", Nodes: []*swayipc.Node{
		{Type: "output", Name: "DP-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "1:web", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 1, Visible: &visible},
			}},
		}},
	}}
	if allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("una ventana fullscreen que no es visible no debe pausar el fondo")
	}
}

func TestGlobalFullscreenCoversAllActiveOutputs(t *testing.T) {
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
				{Type: "con", FullscreenMode: 2},
			}},
		}},
		{Type: "output", Name: "HDMI-A-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "2:chat", Nodes: []*swayipc.Node{
				{Type: "con", FullscreenMode: 0},
			}},
		}},
	}}
	if !allActiveOutputsFullscreen(outputs, workspaces, tree) {
		t.Fatal("fullscreen global debe pausar aunque solo tenga una ventana en el árbol")
	}
}
