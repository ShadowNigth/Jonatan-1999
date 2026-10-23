package main

import (
	"strings"
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func testCenter() *notifyCenter {
	now := time.Date(2026, 10, 23, 14, 5, 0, 0, time.Local)
	live := []notify.Notification{
		{ID: 1, App: "Chat", Sender: "Marta", Summary: "Marta", Body: "¿comemos?",
			Urgency: notify.Critical, InlineReply: true, Progress: -1, Created: now},
		{ID: 2, App: "pacman", Summary: "12 paquetes para actualizar", Body: "sway 1.10",
			Progress: -1, Created: now},
	}
	hist := []notify.Record{
		{Notification: notify.Notification{ID: 3, App: "mpd", Summary: "Reproduciendo",
			Body: "Lofi Girl", Progress: -1, Created: now.Add(-time.Hour)},
			ClosedAt: now.Add(-50 * time.Minute), Reason: "expired"},
	}
	c := &notifyCenter{theme: ui.NewTheme("sakura", true)}
	c.model = notify.NewCenter(live, hist, false)
	return c
}

// Cada línea debe medir exactamente el ancho del marco: ni un carácter de más
// (rompería el borde derecho) ni de menos.
func TestNotifyCenterGroupedRows(t *testing.T) {
	c := testCenter()
	// Tres mensajes del mismo chat (con etiqueta de pila) y un aviso suelto.
	now := time.Date(2026, 10, 23, 14, 0, 0, 0, time.Local)
	live := []notify.Notification{
		{ID: 10, App: "Telegram", GroupID: "marta", Summary: "Marta", Body: "¿Quedamos?",
			Progress: -1, Created: now.Add(-3 * time.Minute)},
		{ID: 11, App: "Telegram", GroupID: "marta", Summary: "Marta", Body: "Traigo el postre",
			Progress: -1, Created: now.Add(-2 * time.Minute)},
		{ID: 12, App: "Telegram", GroupID: "marta", Summary: "Marta", Body: "¿Comemos?",
			Progress: -1, Created: now.Add(-time.Minute)},
		{ID: 13, App: "pacman", Summary: "12 paquetes", Body: "sway 1.10",
			Progress: -1, Created: now},
	}
	c.model = notify.NewCenter(live, nil, false)
	out := c.frame(100, 24)
	if !strings.Contains(out, "Telegram · 3 avisos") {
		t.Fatalf("no se ve la pila plegada: %q", out)
	}
	if !strings.Contains(out, "agrupado") {
		t.Fatalf("la cabecera debe indicar el modo agrupado: %q", out)
	}
	if c.model.Rows()[1].Count != 3 {
		t.Fatalf("la segunda fila debería ser el grupo de 3: %+v", c.model.Rows()[1])
	}

	// g alterna el modo: sin agrupar se ven los cuatro avisos.
	c.handle(ui.Key{Rune: 'g'})
	if c.model.Grouped() {
		t.Fatal("g debería desagrupar")
	}
	if out = c.frame(100, 24); strings.Contains(out, "3 avisos") || strings.Contains(out, "agrupado") {
		t.Fatalf("sin agrupar no debe haber pilas: %q", out)
	}
	if got := len(c.model.Rows()); got != 4 {
		t.Fatalf("filas sin agrupar = %d", got)
	}
	c.handle(ui.Key{Rune: 'g'})
	if !c.model.Grouped() {
		t.Fatal("g debería volver a agrupar")
	}

	// Tab sobre la pila la despliega y sangra sus mensajes.
	c.handle(ui.Key{Name: "down"})
	c.handle(ui.Key{Name: "tab"})
	out = c.frame(100, 24)
	if !strings.Contains(out, "Telegram · 3 avisos") {
		t.Fatalf("la pila desplegada no muestra su franja: %q", out)
	}
	if got := len(c.model.Rows()); got != 5 {
		t.Fatalf("filas tras desplegar = %d, se esperaban 5", got)
	}
}

func TestNotifyCenterFrameFits(t *testing.T) {
	c := testCenter()
	for _, size := range [][2]int{{100, 24}, {80, 20}, {60, 14}, {40, 10}} {
		out := c.frame(size[0], size[1])
		lines := strings.Split(out, "\n")
		for i, ln := range lines {
			if got := ui.Width(ln); got != size[0] {
				t.Fatalf("%dx%d línea %d mide %d: %q", size[0], size[1], i, got, ln)
			}
		}
	}
}

func TestNotifyCenterShowsEntriesAndFilter(t *testing.T) {
	c := testCenter()
	out := c.frame(100, 24)
	for _, want := range []string{"centro de notificaciones", "Marta", "pacman", "mpd", "urgente"} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q en la pantalla", want)
		}
	}

	// El filtro se aplica al escribir.
	c.model.Append('m')
	c.model.Append('p')
	out = c.frame(100, 24)
	if !strings.Contains(out, "mpd") || !strings.Contains(out, "1/3") {
		t.Fatalf("el filtro no se refleja: %q", out)
	}
	if strings.Contains(out, "pacman · 12") {
		t.Fatalf("el filtro no ocultó la entrada que no coincide")
	}

	// Sin coincidencias se avisa en vez de dejar la lista en blanco.
	c.model.SetQuery("zzz")
	if out = c.frame(100, 24); !strings.Contains(out, "nada coincide") {
		t.Fatalf("sin mensaje de lista vacía: %q", out)
	}
}

func TestNotifyCenterKeys(t *testing.T) {
	c := testCenter()
	if c.handle(ui.Key{Name: "down"}) {
		t.Fatal("la flecha abajo no debe salir")
	}
	if c.model.Index() != 1 {
		t.Fatalf("la selección no bajó: %d", c.model.Index())
	}
	// Esc borra el filtro antes de salir.
	c.model.Append('m')
	if c.handle(ui.Key{Name: "esc"}) {
		t.Fatal("Esc con filtro debe limpiar el filtro, no salir")
	}
	if c.model.Query() != "" {
		t.Fatalf("el filtro sigue: %q", c.model.Query())
	}
	if !c.handle(ui.Key{Name: "esc"}) {
		t.Fatal("Esc sin filtro debe salir")
	}
	// Una letra normal filtra; para salir está Esc (y Ctrl+c).
	if c.handle(ui.Key{Rune: 'q'}) {
		t.Fatal("una letra no debe salir")
	}
	if c.model.Query() != "q" {
		t.Fatalf("la letra no llegó al filtro: %q", c.model.Query())
	}
	if !c.handle(ui.Key{Rune: 'c', Ctrl: true}) {
		t.Fatal("Ctrl+c debe salir")
	}
}
