package notify

import (
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func groupNotes(now time.Time) []Notification {
	return []Notification{
		{ID: 1, App: "Telegram", Summary: "Marta", Body: "¿Quedamos?", Created: now.Add(-5 * time.Minute), Progress: -1},
		{ID: 2, App: "Telegram", Summary: "Marta", Body: "Traigo el postre", Created: now.Add(-4 * time.Minute), Progress: -1},
		{ID: 3, App: "Telegram", Summary: "Marta", Body: "¿Comemos?", Created: now.Add(-3 * time.Minute), Progress: -1},
		{ID: 4, App: "pacman", Summary: "12 paquetes", Body: "sway 1.10", Created: now.Add(-2 * time.Minute), Progress: -1},
	}
}

func TestGroupKey(t *testing.T) {
	casos := []struct {
		a, b Notification
		eq   bool
	}{
		{Notification{App: "Telegram"}, Notification{App: "telegram "}, true},
		{Notification{App: "Telegram"}, Notification{App: "Telegram", GroupID: "marta"}, false},
		{Notification{App: "Telegram", GroupID: "Marta"}, Notification{App: "Telegram", GroupID: "marta"}, true},
		{Notification{App: ""}, Notification{App: "sistema"}, true},
		{Notification{App: "Grabación"}, Notification{App: "grabacion"}, true},
	}
	for _, c := range casos {
		a, b := GroupKey(c.a), GroupKey(c.b)
		if (a == b) != c.eq {
			t.Fatalf("GroupKey(%q,%q) = %q y %q, se esperaba iguales=%v",
				c.a.App, c.a.GroupID, a, b, c.eq)
		}
	}
}

func TestGroupAllKeepsOrderAndCounts(t *testing.T) {
	now := time.Now()
	groups := GroupAll(groupNotes(now))
	if len(groups) != 2 {
		t.Fatalf("grupos = %d, se esperaban 2: %+v", len(groups), groups)
	}
	// El grupo más reciente (pacman) va al final, como se apilan las tarjetas.
	if groups[1].Key != GroupKey(Notification{App: "pacman"}) {
		t.Fatalf("el último grupo es %q", groups[1].Key)
	}
	chat := groups[0]
	if chat.Len() != 3 {
		t.Fatalf("el chat tiene %d avisos", chat.Len())
	}
	// Dentro del grupo, la más reciente primero.
	if chat.Items[0].ID != 3 || chat.Items[2].ID != 1 {
		t.Fatalf("orden dentro del grupo: %v", chat.IDs())
	}
	if chat.Label() != "Telegram · 3 avisos" {
		t.Fatalf("etiqueta del grupo = %q", chat.Label())
	}
	if chat.Preview() != "Marta · y 2 avisos más" {
		t.Fatalf("vista previa = %q", chat.Preview())
	}
	// Sin urgencia explícita, la de las notificaciones es baja.
	if chat.Urgency() != Low {
		t.Fatalf("urgencia del grupo = %v", chat.Urgency())
	}
	if got := groups[1].Label(); got != "pacman" {
		t.Fatalf("un grupo de uno no debe numerarse: %q", got)
	}
}

func TestGroupUrgencyAndPreview(t *testing.T) {
	now := time.Now()
	g := Group{Items: []Notification{
		{App: "X", Summary: "aviso", Created: now},
		{App: "X", Summary: "grave", Urgency: Critical, Created: now.Add(-time.Second)},
	}}
	if g.Urgency() != Critical {
		t.Fatalf("la urgencia del grupo debe ser la más alta: %v", g.Urgency())
	}
	if g.Preview() != "aviso · y 1 aviso más" {
		t.Fatalf("vista previa = %q", g.Preview())
	}
	if g.Age(now.Add(time.Minute)) < time.Minute-time.Second {
		t.Fatalf("edad mal calculada: %v", g.Age(now.Add(time.Minute)))
	}
}

func TestTrimToGroups(t *testing.T) {
	now := time.Now()
	items := groupNotes(now)
	// Tres avisos del chat y uno de pacman: con dos grupos caben los cuatro.
	if got := TrimToGroups(items, 2); len(got) != 4 {
		t.Fatalf("con 2 grupos deben quedar 4 avisos, quedan %d", len(got))
	}
	// Con un solo grupo solo cabe el más reciente (pacman), entero.
	got := TrimToGroups(items, 1)
	if len(got) != 1 || got[0].App != "pacman" {
		t.Fatalf("con 1 grupo quedan %d: %+v", len(got), got)
	}
	if got := TrimToGroups(items, 0); got != nil {
		t.Fatalf("con 0 grupos no debe quedar nada: %+v", got)
	}
	// Un grupo reciente con varios avisos entra completo.
	items = append(items, Notification{ID: 9, App: "mpd", Summary: "play", Created: now, Progress: -1})
	got = TrimToGroups(items, 2)
	if len(got) != 2 || got[1].App != "mpd" || got[0].App != "pacman" {
		t.Fatalf("recorte inesperado: %+v", got)
	}
}

func TestStack(t *testing.T) {
	s := NewStack()
	if s.Expanded("x") {
		t.Fatal("un grupo nuevo no debe estar desplegado")
	}
	if !s.Toggle("x") {
		t.Fatal("Toggle debe desplegar")
	}
	if !s.Expanded("x") {
		t.Fatal("el grupo debería estar desplegado")
	}
	if s.Toggle("x") {
		t.Fatal("el segundo Toggle debe plegar")
	}
	s.Expand("a")
	s.Expand("b")
	if got := s.OpenKeys(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("claves abiertas = %v", got)
	}
	// Prune olvida los grupos que ya no existen.
	s.Prune([]Group{{Key: "a"}})
	if got := s.OpenKeys(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("tras Prune = %v", got)
	}
	s.CollapseAll()
	if len(s.OpenKeys()) != 0 {
		t.Fatalf("CollapseAll dejó %v", s.OpenKeys())
	}
	// Un Stack nil se comporta como plegado.
	var nilStack *Stack
	if nilStack.Expanded("x") {
		t.Fatal("un Stack nil no debe desplegar nada")
	}
}

func TestRenderGroupsCollapsedAndExpanded(t *testing.T) {
	now := time.Now()
	items := groupNotes(now)
	r := New(RenderOptions{
		Width: 360, Padding: 14, Gap: 8, Radius: 14,
		Face: font.Builtin(), Theme: ui.PaletteByName("sakura"), Scale: 2, MaxLines: 3,
	})

	// Plegado: dos tarjetas, la del chat con el contador y su capa detrás.
	canvas, cards := r.RenderGroups(Views(items, nil), now)
	if len(cards) != 2 {
		t.Fatalf("tarjetas plegadas = %d, se esperaban 2", len(cards))
	}
	var chat Card
	for _, c := range cards {
		if c.GroupCount == 3 {
			chat = c
		}
	}
	if chat.GroupCount != 3 || !chat.Collapsed || !chat.Toggle {
		t.Fatalf("la tarjeta del grupo no está marcada: %+v", chat)
	}
	if len(chat.IDs) != 3 {
		t.Fatalf("la tarjeta del grupo debe representar 3 avisos: %v", chat.IDs)
	}
	encontrado := false
	for _, b := range chat.Buttons {
		if b.Key == GroupExpandAction {
			encontrado = true
		}
	}
	if !encontrado {
		t.Fatalf("falta el botón de abrir la pila: %+v", chat.Buttons)
	}
	if canvas.H < 20 || canvas.W != 360 {
		t.Fatalf("lienzo raro: %dx%d", canvas.W, canvas.H)
	}

	// Desplegado: una franja por grupo y una tarjeta por aviso.
	stack := NewStack()
	stack.Expand(GroupKey(items[0]))
	expandedCanvas, expandedCards := r.RenderGroups(Views(items, stack), now)
	if len(expandedCards) != 1+3+1 {
		t.Fatalf("tarjetas desplegadas = %d, se esperaban 5", len(expandedCards))
	}
	if !expandedCards[0].Header {
		t.Fatalf("la primera tarjeta debería ser la franja del grupo: %+v", expandedCards[0])
	}
	if len(expandedCards[0].Buttons) == 0 || expandedCards[0].Buttons[0].Key != GroupCollapseAction {
		t.Fatalf("la franja debe traer el botón de plegar: %+v", expandedCards[0].Buttons)
	}
	if expandedCanvas.H <= canvas.H {
		t.Fatalf("desplegado debería ocupar más alto: %d <= %d", expandedCanvas.H, canvas.H)
	}

	// Las tarjetas no se solapan y caben en el lienzo.
	for _, c := range expandedCards {
		if c.Rect.Y+c.Rect.H > expandedCanvas.H {
			t.Fatalf("tarjeta fuera del lienzo: %+v (alto %d)", c.Rect, expandedCanvas.H)
		}
	}
	for i := 1; i < len(expandedCards); i++ {
		if expandedCards[i].Rect.Y < expandedCards[i-1].Rect.Y+expandedCards[i-1].Rect.H {
			t.Fatalf("tarjetas solapadas: %+v y %+v", expandedCards[i-1].Rect, expandedCards[i].Rect)
		}
	}
}

func TestRenderGroupsRespectsMaxVisible(t *testing.T) {
	now := time.Now()
	items := groupNotes(now)
	store := NewStore(Options{MaxVisible: 2, GroupVisible: true, now: func() time.Time { return now }})
	for _, n := range items {
		store.Add(n)
	}
	if got := store.Visible(); len(got) != 4 {
		t.Fatalf("con grupos visibles deben entrar los 4 avisos (2 grupos): %d", len(got))
	}
	store = NewStore(Options{MaxVisible: 2, now: func() time.Time { return now }})
	for _, n := range items {
		store.Add(n)
	}
	if got := store.Visible(); len(got) != 2 {
		t.Fatalf("sin agrupar deben quedar 2: %d", len(got))
	}
}

func TestCenterGroupingRows(t *testing.T) {
	now := time.Now()
	live := groupNotes(now)
	c := NewCenter(live, nil, false)
	c.SetGrouped(true)
	rows := c.Rows()
	if len(rows) != 2 {
		t.Fatalf("filas agrupadas = %d: %+v", len(rows), rows)
	}
	// Lo más reciente (pacman) va primero, como en el listado sin agrupar.
	if rows[0].Group || rows[0].Entry.App != "pacman" {
		t.Fatalf("la primera fila debería ser pacman: %+v", rows[0])
	}
	if !rows[1].Group || rows[1].Count != 3 {
		t.Fatalf("la segunda fila debería ser el grupo de 3: %+v", rows[1])
	}
	if rows[1].Title() != "Telegram · 3 avisos" {
		t.Fatalf("título de la fila = %q", rows[1].Title())
	}
	if ids := rows[1].IDs(); len(ids) != 3 {
		t.Fatalf("IDs de la fila = %v", ids)
	}
	if c.GroupCount() != 2 {
		t.Fatalf("GroupCount = %d", c.GroupCount())
	}

	// Desplegar añade la franja y las tres notificaciones.
	c.Select(1)
	row, ok := c.ToggleSelected()
	if !ok || row.Count != 3 {
		t.Fatalf("no se pudo desplegar el grupo: %+v %v", row, ok)
	}
	rows = c.Rows()
	if len(rows) != 5 {
		t.Fatalf("filas desplegadas = %d: %+v", len(rows), rows)
	}
	if !rows[1].Header {
		t.Fatalf("la segunda fila debe ser la franja: %+v", rows[1])
	}
	// La selección apunta a la franja; Enter la vuelve a plegar.
	row, ok = c.ToggleSelected()
	if !ok || c.Expanded(row.Key) {
		t.Fatalf("la franja debería plegar el grupo")
	}
	if len(c.Rows()) != 2 {
		t.Fatalf("tras plegar quedan %d filas", len(c.Rows()))
	}

	// Una notificación suelta no se pliega.
	c.Select(0)
	if _, ok := c.ToggleSelected(); ok {
		t.Fatal("una notificación suelta no debe plegarse")
	}

	// Sin agrupar hay una fila por notificación y no se conserva lo abierto.
	c.SetGrouped(false)
	if got := len(c.Rows()); got != 4 {
		t.Fatalf("filas sin agrupar = %d", got)
	}
	if len(c.OpenKeys()) != 0 {
		t.Fatalf("al desagrupar quedan claves abiertas: %v", c.OpenKeys())
	}
}

func TestCenterRowsRespectSelection(t *testing.T) {
	now := time.Now()
	c := NewCenter(groupNotes(now), nil, false)
	c.SetGrouped(true)
	c.Select(1 << 20) // al final
	row, ok := c.SelectedRow()
	if !ok || row.Key != GroupKey(Notification{App: "telegram"}) {
		t.Fatalf("la selección final debería ser el chat: %+v", row)
	}
	// Al desplegar el grupo la selección se mantiene en la franja.
	c.ToggleSelected()
	row, _ = c.SelectedRow()
	if !row.Header {
		t.Fatalf("tras desplegar la selección debería seguir en la franja: %+v", row)
	}
	// El filtro también se aplica sobre las filas.
	c.SetQuery("postre")
	rows := c.Rows()
	if len(rows) != 1 || rows[0].Entry.ID != 2 {
		t.Fatalf("el filtro por cuerpo no encuentra el mensaje: %+v", rows)
	}
}
