package notify

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func TestCompactRendererDefaultsAndTitleScale(t *testing.T) {
	compact := New(RenderOptions{Face: font.Builtin(), Theme: ui.PaletteByName("sakura")})
	if compact.Opt.Width != 300 || compact.Opt.Padding != 10 || compact.Opt.Gap != 6 || compact.Opt.MaxLines != 2 {
		t.Fatalf("valores compactos del renderizador: %+v", compact.Opt)
	}
	if got := compact.titleScale(); got != 1 {
		t.Fatalf("el resumen compacto usa escala %d, quería 1", got)
	}

	wide := New(RenderOptions{
		Width: 360, Scale: 1, Face: font.Builtin(), Theme: ui.PaletteByName("sakura"),
	})
	if got := wide.titleScale(); got != 2 {
		t.Fatalf("una tarjeta ancha debe conservar el resumen destacado, escala %d", got)
	}
}

func TestNotificationActionLabelsUseSpanish(t *testing.T) {
	for _, test := range []struct {
		label, key, want string
	}{
		{"Open", "default", "Abrir"},
		{"Open link", "open-link", "Abrir enlace"},
		{"View details", "view_details", "Ver detalles"},
		{"Reply", "reply", "Responder"},
		{"Reply all", "reply-all", "Responder a todos"},
		{"Dismiss", "close", "Cerrar"},
		{"Later", "later", "Más tarde"},
		{"Mark as read", "mark_read", "Marcar como leído"},
		{"Retry", "retry", "Reintentar"},
		{"", "default", "Abrir"},
		{"Abrir chat", "custom", "Abrir chat"},
	} {
		if got := localizedActionLabel(test.label, test.key); got != test.want {
			t.Errorf("localizedActionLabel(%q, %q) = %q, quería %q", test.label, test.key, got, test.want)
		}
	}
}
