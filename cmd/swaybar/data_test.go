package main

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

func TestWindowCountsUsesOneTreeAndCountsWorkspaceLeaves(t *testing.T) {
	tree := &swayipc.Node{Type: "root", Nodes: []*swayipc.Node{
		{Type: "output", Name: "DP-1", Nodes: []*swayipc.Node{
			{Type: "workspace", Name: "1:web", Nodes: []*swayipc.Node{
				{Type: "con", Nodes: []*swayipc.Node{
					{Type: "con", AppID: "chromium"},
					{Type: "floating_con", AppID: "thunar", Focused: true},
				}},
			}},
			{Type: "workspace", Name: "2:chat", Nodes: []*swayipc.Node{
				{Type: "con", AppID: "foot"},
			}},
		}},
	}}

	got := windowCounts(tree)
	if got["1:web"] != 2 || got["2:chat"] != 1 {
		t.Fatalf("conteos por workspace = %v; se esperaban 2 y 1", got)
	}
	if len(got) != 2 {
		t.Fatalf("conteos inesperados: %v", got)
	}
	_, focused := summarizeTree(tree)
	if focused == nil || focused.AppID != "thunar" {
		t.Fatalf("ventana enfocada = %v; se esperaba thunar", focused)
	}
}

func TestSnapshotKeyTracksDisplayedWorkspaceAndBatteryState(t *testing.T) {
	base := bar.Snapshot{
		Workspaces: []bar.Workspace{{Name: "1", Focused: true}},
		HasBattery: true,
		Charging:   true,
		Battery:    72,
	}
	key := snapshotKey(1920, base)

	workspaceChanged := base
	workspaceChanged.Workspaces = append([]bar.Workspace(nil), base.Workspaces...)
	workspaceChanged.Workspaces[0].Focused = false
	if snapshotKey(1920, workspaceChanged) == key {
		t.Fatal("el foco de workspace debe invalidar el fotograma")
	}

	batteryChanged := base
	batteryChanged.Charging = false
	if snapshotKey(1920, batteryChanged) == key {
		t.Fatal("el estado de carga debe invalidar el fotograma")
	}
}
