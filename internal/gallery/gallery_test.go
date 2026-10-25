package gallery

import (
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func TestListPaneRendersAllEntriesAcrossGroups(t *testing.T) {
	pane := &listPane{entries: []Entry{
		{Title: "Escena Sakura", Tag: "escenas"},
		{Title: "Escena Nieve", Tag: "escenas"},
		{Title: "video-a.mp4", Tag: "archivos"},
		{Title: "gif-b.gif", Tag: "archivos"},
		{Title: "imagen-c.png", Tag: "archivos"},
	}}

	lines := pane.render(ui.NewTheme("sakura", false), 48, 8, "")
	if len(lines) != 8 {
		t.Fatalf("render devolvió %d filas; se esperaban 8", len(lines))
	}
	content := strings.Join(lines, "\n")
	for _, title := range []string{"Escena Sakura", "Escena Nieve", "video-a.mp4", "gif-b.gif", "imagen-c.png"} {
		if !strings.Contains(content, title) {
			t.Errorf("no se mostró %q en la lista agrupada", title)
		}
	}
}

func TestListPaneKeepsSelectedEntryVisibleWithGroupHeaders(t *testing.T) {
	entries := []Entry{
		{Title: "Escena Sakura", Tag: "escenas"},
		{Title: "Escena Nieve", Tag: "escenas"},
		{Title: "video-a.mp4", Tag: "archivos"},
		{Title: "gif-b.gif", Tag: "archivos"},
		{Title: "imagen-c.png", Tag: "archivos"},
		{Title: "video-d.mkv", Tag: "archivos"},
	}
	pane := &listPane{entries: entries, cursor: 5}

	lines := pane.render(ui.NewTheme("sakura", false), 48, 3, "")
	if len(lines) != 3 {
		t.Fatalf("render devolvió %d filas; se esperaban 3", len(lines))
	}
	if !strings.Contains(strings.Join(lines, "\n"), entries[pane.cursor].Title) {
		t.Fatalf("la selección %q quedó fuera de la lista visible", entries[pane.cursor].Title)
	}
}
