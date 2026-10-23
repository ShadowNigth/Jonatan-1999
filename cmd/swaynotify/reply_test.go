package main

import (
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func TestReplyFrameFits(t *testing.T) {
	theme := ui.NewTheme("sakura", true)
	n := notify.Notification{
		ID: 7, App: "Chat", Sender: "Marta", Summary: "Marta",
		Body:        "¿Comemos a las dos? Traigo el postre y algo de fruta para después de comer.",
		InlineReply: true, ReplyPlaceholder: "Responder a Marta…",
	}
	for _, size := range [][2]int{{60, 12}, {40, 10}, {24, 8}} {
		out := replyFrame(theme, size[0], size[1], n, "vale, nos vemos allí", "", false)
		lines := strings.Split(out, "\n")
		for i, ln := range lines {
			if got := ui.Width(ln); got > size[0] {
				t.Fatalf("%dx%d línea %d mide %d: %q", size[0], size[1], i, got, ln)
			}
		}
		if !strings.Contains(out, "Responder") && !strings.Contains(out, "responder") {
			t.Fatalf("el título del diálogo no aparece: %q", out)
		}
	}
}

func TestReplyFrameShowsPlaceholderAndCounter(t *testing.T) {
	theme := ui.NewTheme("sakura", true)
	n := notify.Notification{ID: 2, App: "Chat", Summary: "Marta", InlineReply: true}
	out := replyFrame(theme, 56, 8, n, "", "Responder a Marta…", false)
	if !strings.Contains(out, "Responder a Marta") {
		t.Fatalf("falta el texto de ayuda: %q", out)
	}
	if !strings.Contains(out, "0/512") {
		t.Fatalf("falta el contador: %q", out)
	}
	out = replyFrame(theme, 56, 8, n, "hola", "", false)
	if !strings.Contains(out, "hola") || !strings.Contains(out, "4/512") {
		t.Fatalf("no se ve el texto escrito: %q", out)
	}
}
