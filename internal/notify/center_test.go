package notify

import (
	"strings"
	"testing"
)

func sampleCenter() *Center {
	live := []Notification{
		{ID: 3, App: "spotify", Summary: "Lofi Girl", Body: "Reproduciendo", Progress: -1},
		{ID: 4, App: "Telegram", Summary: "Nuevo mensaje", Body: "¿Vienes?", InlineReply: true,
			ReplyPlaceholder: "Responder", Progress: -1},
	}
	hist := []Record{
		{Notification: Notification{ID: 1, App: "pacman", Summary: "Actualización pendiente", Progress: -1},
			ClosedAt: at(1), Reason: "dismissed"},
		{Notification: Notification{ID: 2, App: "wf-recorder", Summary: "Grabación terminada",
			Body: "El vídeo se guardó", Urgency: Critical, Progress: -1},
			ClosedAt: at(2), Reason: "expired"},
	}
	return NewCenter(live, hist, false)
}

func TestCenterOrderAndCounts(t *testing.T) {
	c := sampleCenter()
	if c.Len() != 4 {
		t.Fatalf("entradas: %d", c.Len())
	}
	live, hist := c.Counts()
	if live != 2 || hist != 2 {
		t.Fatalf("vivas=%d historial=%d", live, hist)
	}
	f := c.Filtered()
	if !f[0].Live || !f[1].Live {
		t.Error("las vivas deben ir primero")
	}
	// Dentro de las vivas, lo más reciente arriba.
	if f[0].ID != 4 || f[1].ID != 3 {
		t.Errorf("orden de vivas: %d, %d", f[0].ID, f[1].ID)
	}
	// En el historial, la más reciente arriba.
	if f[2].ID != 2 || f[3].ID != 1 {
		t.Errorf("orden del historial: %d, %d", f[2].ID, f[3].ID)
	}
}

func TestCenterSkipsLiveInHistory(t *testing.T) {
	live := []Notification{{ID: 7, App: "thunar", Summary: "Copiando", Progress: -1}}
	hist := []Record{
		{Notification: Notification{ID: 7, App: "thunar", Summary: "Copiando", Progress: -1},
			ClosedAt: at(1), Reason: "replaced"},
	}
	c := NewCenter(live, hist, false)
	if c.Len() != 1 {
		t.Fatalf("no debe duplicarse: %d", c.Len())
	}
	if !c.Filtered()[0].Live {
		t.Error("debe quedarse la versión viva")
	}
}

func TestCenterFilterIgnoresCaseAndAccents(t *testing.T) {
	c := sampleCenter()
	c.SetQuery("GRABACION")
	f := c.Filtered()
	if len(f) != 1 || f[0].ID != 2 {
		t.Fatalf("filtro con acentos: %+v", f)
	}
	c.SetQuery("telegram")
	if f := c.Filtered(); len(f) != 1 || f[0].ID != 4 {
		t.Fatalf("filtro por aplicación: %+v", f)
	}
	// Varias palabras: deben aparecer todas.
	c.SetQuery("nuevo mensaje")
	if f := c.Filtered(); len(f) != 1 {
		t.Fatalf("filtro de dos palabras: %+v", f)
	}
	c.SetQuery("zzz")
	if f := c.Filtered(); len(f) != 0 {
		t.Fatalf("sin resultados: %+v", f)
	}
	if _, ok := c.Selected(); ok {
		t.Error("sin resultados no hay selección")
	}
	c.Backspace()
	c.Backspace()
	c.Backspace()
	if c.Query() != "" {
		t.Errorf("backspace: %q", c.Query())
	}
}

func TestCenterSelectionClamps(t *testing.T) {
	c := sampleCenter()
	c.Move(10)
	if idx := c.Index(); idx != 3 {
		t.Errorf("selección al final: %d", idx)
	}
	c.Move(1)
	if idx := c.Index(); idx != 3 {
		t.Errorf("no debe pasarse: %d", idx)
	}
	c.Move(-100)
	if idx := c.Index(); idx != 0 {
		t.Errorf("no debe salirse por abajo: %d", idx)
	}
	e, ok := c.Selected()
	if !ok || e.ID != 4 {
		t.Fatalf("seleccionada: %+v", e)
	}
	// Al cambiar el filtro la selección vuelve al principio.
	c.Move(2)
	c.SetQuery("lofi")
	if c.Index() != 0 {
		t.Errorf("tras filtrar la selección debe reiniciarse: %d", c.Index())
	}
	// Si el filtro deja la lista más corta, la selección se recorta.
	c.SetQuery("")
	c.Select(3)
	c.SetQuery("lofi")
	if idx := c.Index(); idx != 0 {
		t.Errorf("selección fuera de rango: %d", idx)
	}
}

func TestCenterBadges(t *testing.T) {
	c := sampleCenter()
	cases := map[uint32]string{
		4: "responder",
		2: "urgente",
		1: "cerrada",
	}
	for id, want := range cases {
		var got string
		for _, e := range c.Filtered() {
			if e.ID == id {
				got = strings.Join(e.Badges(), ",")
			}
		}
		if !strings.Contains(got, want) {
			t.Errorf("entrada %d: insignias %q, se esperaba %q", id, got, want)
		}
	}
}

func TestCenterSummary(t *testing.T) {
	c := sampleCenter()
	if got := c.Summary(); !strings.Contains(got, "2 en pantalla") || !strings.Contains(got, "2 en el historial") {
		t.Errorf("resumen: %q", got)
	}
	c.SetDND(true)
	if got := c.Summary(); !strings.Contains(got, "no molestar") {
		t.Errorf("resumen con DND: %q", got)
	}
	c.SetQuery("lofi")
	if got := c.Summary(); !strings.Contains(got, "1 resultado") {
		t.Errorf("resumen filtrado: %q", got)
	}
	if !c.DND() {
		t.Error("DND debería seguir activo")
	}
}

func TestCenterEmpty(t *testing.T) {
	c := NewCenter(nil, nil, false)
	if c.Len() != 0 {
		t.Fatalf("entradas: %d", c.Len())
	}
	c.Move(1)
	c.Select(5)
	if _, ok := c.Selected(); ok {
		t.Error("un centro vacío no tiene selección")
	}
	if got := c.Summary(); !strings.Contains(got, "0 en pantalla") {
		t.Errorf("resumen vacío: %q", got)
	}
}

func TestCenterEntryTitle(t *testing.T) {
	c := sampleCenter()
	e := c.Filtered()[0]
	if got := e.Title(); !strings.Contains(got, "Telegram") || !strings.Contains(got, "Nuevo mensaje") {
		t.Errorf("título: %q", got)
	}
	solo := Entry{Notification: Notification{Summary: "sin aplicación", Progress: -1}}
	if got := solo.Title(); !strings.HasPrefix(got, "sistema ·") {
		t.Errorf("título sin app: %q", got)
	}
}

func TestSanitizeReply(t *testing.T) {
	cases := map[string]string{
		"  hola   mundo ": "hola mundo",
		"linea1\nlinea2":  "linea1 linea2",
		"con\ttab":        "con tab",
		"control\x07aquí": "control aquí",
		"acentos ñ á é":   "acentos ñ á é",
	}
	for in, want := range cases {
		if got := SanitizeReply(in); got != want {
			t.Errorf("SanitizeReply(%q) = %q; esperado %q", in, got, want)
		}
	}
	long := strings.Repeat("x", MaxReplyLen+50)
	if got := SanitizeReply(long); len([]rune(got)) != MaxReplyLen {
		t.Errorf("longitud: %d", len([]rune(got)))
	}
}
