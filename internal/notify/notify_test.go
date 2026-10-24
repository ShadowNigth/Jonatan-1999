package notify

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func at(sec float64) time.Time {
	return time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC).Add(time.Duration(sec * float64(time.Second)))
}

func newTestStore() *Store {
	s := NewStore(Options{Timeout: 5 * time.Second, MaxVisible: 3, HistorySize: 5})
	s.SetNow(func() time.Time { return at(0) })
	return s
}

func TestAddAssignsIDsAndExpiry(t *testing.T) {
	s := newTestStore()
	a := s.Add(Notification{App: "foot", Summary: "uno"})
	b := s.Add(Notification{App: "foot", Summary: "dos", Urgency: Critical})
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids: %d %d", a.ID, b.ID)
	}
	if a.Expires.IsZero() {
		t.Error("la notificación normal debería caducar")
	}
	if !b.Expires.IsZero() {
		t.Error("la crítica no debería caducar por defecto")
	}
	if got := len(s.Visible()); got != 2 {
		t.Fatalf("visibles: %d", got)
	}
}

func TestTimeoutZeroUsesDefaultAndNegativeNever(t *testing.T) {
	s := newTestStore()
	d := s.Add(Notification{Summary: "defecto", Timeout: 0})
	if d.Timeout != 5000 {
		t.Errorf("timeout por defecto: %d", d.Timeout)
	}
	never := s.Add(Notification{Summary: "nunca", Timeout: -1})
	if !never.Expires.IsZero() || never.Timeout != 0 {
		t.Errorf("timeout -1 debería ser permanente: %+v", never)
	}
	crit := s.Add(Notification{Summary: "crítica con tiempo", Urgency: Critical, Timeout: 1500})
	if crit.Timeout != 1500 {
		t.Errorf("la crítica con tiempo explícito lo respeta: %d", crit.Timeout)
	}
}

func TestReplaceKeepsSlot(t *testing.T) {
	s := newTestStore()
	a := s.Add(Notification{Summary: "descarga 10%"})
	_ = s.Add(Notification{Summary: "otra"})
	b := s.Add(Notification{Summary: "descarga 90%", ReplacesID: a.ID})
	if b.ID != a.ID {
		t.Fatalf("el reemplazo debe conservar el id: %d != %d", b.ID, a.ID)
	}
	vis := s.Visible()
	if len(vis) != 2 || vis[0].Summary != "descarga 90%" {
		t.Fatalf("visibles tras reemplazar: %+v", vis)
	}
	h := s.History()
	if len(h) != 1 || h[0].Reason != "replaced" {
		t.Fatalf("historial: %+v", h)
	}
}

func TestExpireAndDismiss(t *testing.T) {
	s := newTestStore()
	a := s.Add(Notification{Summary: "caduca", Timeout: 2000})
	b := s.Add(Notification{Summary: "sigue"})
	s.SetNow(func() time.Time { return at(3) })
	ids := s.Expire()
	if len(ids) != 1 || ids[0] != a.ID {
		t.Fatalf("caducadas: %v", ids)
	}
	if !s.Dismiss(b.ID, "dismissed") {
		t.Fatal("se esperaba poder cerrar la segunda")
	}
	if s.Dismiss(b.ID, "dismissed") {
		t.Fatal("cerrar dos veces no debe funcionar")
	}
	h := s.History()
	if len(h) != 2 || h[0].Reason != "expired" || h[1].Reason != "dismissed" {
		t.Fatalf("historial: %+v", h)
	}
}

func TestDNDHidesButKeepsCritical(t *testing.T) {
	s := NewStore(Options{Timeout: time.Second, MaxVisible: 3, DNDBypass: true})
	s.SetNow(func() time.Time { return at(0) })
	s.SetDND(true)
	s.Add(Notification{Summary: "normal"})
	crit := s.Add(Notification{Summary: "importante", Urgency: Critical})
	vis := s.Visible()
	if len(vis) != 1 || vis[0].ID != crit.ID {
		t.Fatalf("con no molestar solo pasa la crítica: %+v", vis)
	}
	if s.Pending() != 2 {
		t.Errorf("pendientes: %d", s.Pending())
	}
	if s.ToggleDND() {
		t.Error("ToggleDND desde activo debería desactivarlo")
	}
	if s.DND() {
		t.Error("DND debería estar desactivado")
	}
	if len(s.Visible()) != 2 {
		t.Error("al desactivar no molestar deben verse las dos")
	}
}

func TestMaxVisibleKeepsNewest(t *testing.T) {
	s := NewStore(Options{Timeout: time.Hour, MaxVisible: 2})
	s.SetNow(func() time.Time { return at(0) })
	for _, txt := range []string{"uno", "dos", "tres"} {
		s.Add(Notification{Summary: txt})
	}
	vis := s.Visible()
	if len(vis) != 2 || vis[0].Summary != "dos" || vis[1].Summary != "tres" {
		t.Fatalf("visibles: %+v", vis)
	}
}

func TestClearReturnsIDs(t *testing.T) {
	s := newTestStore()
	s.Add(Notification{Summary: "a"})
	s.Add(Notification{Summary: "b"})
	ids := s.Clear()
	if len(ids) != 2 || len(s.Visible()) != 0 {
		t.Fatalf("ids=%v visibles=%d", ids, len(s.Visible()))
	}
}

func TestSaveAndLoadHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notifications.json")
	s := newTestStore()
	a := s.Add(Notification{App: "foot", Summary: "guardada", Body: "cuerpo"})
	s.Dismiss(a.ID, "dismissed")

	if err := s.SaveHistory(path); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	s2 := newTestStore()
	if err := s2.LoadHistory(path); err != nil {
		t.Fatalf("cargar: %v", err)
	}
	h := s2.History()
	if len(h) != 1 || h[0].Summary != "guardada" || h[0].App != "foot" {
		t.Fatalf("historial cargado: %+v", h)
	}
	// Los IDs nuevos no chocan con los del historial.
	if n := s2.Add(Notification{Summary: "nueva"}); n.ID != a.ID+1 {
		t.Errorf("id nuevo: %d (esperado %d)", n.ID, a.ID+1)
	}
	if err := s2.LoadHistory(filepath.Join(dir, "no-existe.json")); err != nil {
		t.Errorf("un historial ausente no es un error: %v", err)
	}
}

func TestStripMarkup(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hola <b>mundo</b>", "hola mundo"},
		{"a &amp; b &lt;c&gt;", "a & b <c>"},
		{"<i>uno</i> <u>dos</u>", "uno dos"},
		{`<a href="https://x">enlace</a>`, "enlace"},
		{"2 < 3 y 5 > 4", "2 < 3 y 5 > 4"},
		{"<img src='x'/>texto", "texto"},
		{"sin etiquetas", "sin etiquetas"},
	}
	for _, c := range cases {
		if got := StripMarkup(c.in); got != c.want {
			t.Errorf("StripMarkup(%q) = %q; esperado %q", c.in, got, c.want)
		}
	}
}

func TestNotificationTextCleansControlsAndInvalidUTF8(t *testing.T) {
	got := Sanitize("uno\x01dos\u0085tres\u200bcuatro", 0)
	if got != "uno dos tres cuatro" {
		t.Fatalf("los controles deberían separar palabras sin dibujar símbolos: %q", got)
	}
	if got := Sanitize("cafe\u0301", 0); got != "cafe" {
		t.Fatalf("la marca combinante no debería convertirse en un glifo extraño: %q", got)
	}
	malformed := string([]byte{'n', 'i', 0xff, 0xc3, 0xb1})
	if got := StripMarkup(malformed); got != "niñ" {
		t.Fatalf("el UTF-8 inválido dejó artefactos: %q", got)
	}
}

func TestSanitizeAndShorten(t *testing.T) {
	if got := Sanitize("  hola   <b>mundo</b>\n", 0); got != "hola mundo" {
		t.Errorf("Sanitize: %q", got)
	}
	if got := Sanitize("abcdefghij", 5); got != "abcd…" {
		t.Errorf("Sanitize con límite: %q", got)
	}
	if got := Shorten("abcdef", 4); got != "abc…" {
		t.Errorf("Shorten: %q", got)
	}
	if got := Shorten("ab", 4); got != "ab" {
		t.Errorf("Shorten sin recorte: %q", got)
	}
}

func TestWrap(t *testing.T) {
	// 6 px por carácter.
	lines := Wrap("uno dos tres cuatro cinco", 18*6, 6, 3)
	if len(lines) > 3 {
		t.Fatalf("demasiadas líneas: %#v", lines)
	}
	for _, l := range lines {
		if len([]rune(l)) > 18 {
			t.Errorf("línea demasiado larga: %q", l)
		}
	}
	// Respeta los saltos explícitos.
	if got := Wrap("uno\ndos", 100, 6, 4); len(got) != 2 || got[1] != "dos" {
		t.Errorf("saltos: %#v", got)
	}
	// Palabras larguísimas se parten.
	long := strings.Repeat("x", 50)
	got := Wrap(long, 10*6, 6, 10)
	if len(got) < 5 {
		t.Errorf("se esperaba partir la palabra: %#v", got)
	}
	// Recorte con puntos suspensivos al superar maxLines.
	many := Wrap(strings.Repeat("palabra ", 40), 12*6, 6, 2)
	if len(many) != 2 || !strings.HasSuffix(many[1], "…") {
		t.Errorf("recorte: %#v", many)
	}
}

func sampleNotes() []Notification {
	return []Notification{
		{
			ID: 1, App: "spotify", Summary: "Lofi Girl · Midnight Study",
			Body: "Nueva canción de la lista «estudio»", Urgency: Normal,
			Created: at(0), Expires: at(20), Timeout: 20000, Progress: -1,
		},
		{
			ID: 2, App: "pacman", Summary: "Actualización importante",
			Body:    "Se han actualizado <b>12 paquetes</b> del sistema. Reinicia la sesión para aplicar los cambios.",
			Urgency: Critical, Created: at(2), Progress: 65,
			Actions: []Action{{Key: "default", Label: "Abrir"}, {Key: "later", Label: "Más tarde"}},
		},
		{
			ID: 3, App: "wf-recorder", Summary: "Grabación detenida",
			Body:    "El vídeo se guardó en ~/Vídeos/grabacion.mp4",
			Urgency: Low, Created: at(4), Timeout: -1, Progress: -1,
		},
	}
}

func testRenderer(width int) *Renderer {
	return New(RenderOptions{
		Width: width, Padding: 14, Gap: 8, Radius: 14, Opacity: 0.94,
		Face: font.Builtin(), Theme: ui.PaletteByName("sakura"), Scale: 1,
	})
}

func TestRenderLayout(t *testing.T) {
	items := sampleNotes()
	r := testRenderer(360)
	now := at(6)
	c, cards := r.Render(items, now)
	if c.W != 360 {
		t.Fatalf("ancho: %d", c.W)
	}
	h := r.Measure(items, now)
	if c.H < h {
		t.Fatalf("alto del lienzo %d < medido %d", c.H, h)
	}
	if len(cards) != 3 {
		t.Fatalf("tarjetas: %d", len(cards))
	}
	for i, card := range cards {
		if card.Rect.W != 360 || card.Rect.H <= 0 {
			t.Errorf("tarjeta %d: %+v", i, card.Rect)
		}
		if i > 0 {
			prev := cards[i-1].Rect
			if card.Rect.Y < prev.Y+prev.H {
				t.Errorf("solapamiento entre %d y %d: %+v %+v", i-1, i, prev, card.Rect)
			}
			if card.Rect.Y-prev.Y-prev.H > r.Opt.Gap+8 {
				t.Errorf("hueco demasiado grande entre %d y %d", i-1, i)
			}
		}
		if card.Rect.Y+card.Rect.H > c.H {
			t.Errorf("la tarjeta %d se sale del lienzo", i)
		}
		for _, b := range card.Buttons {
			if b.Rect.Y+b.Rect.H > card.Rect.Y+card.Rect.H || b.Rect.X+b.Rect.W > 360 {
				t.Errorf("botón fuera de la tarjeta: %+v en %+v", b, card.Rect)
			}
		}
	}
	// El fondo de la tarjeta debe ser opaco en el centro.
	px := (cards[0].Rect.Y+cards[0].Rect.H/2)*c.W + 20
	if c.Pix[px*4+3] < 200 {
		t.Errorf("alfa del fondo demasiado bajo: %d", c.Pix[px*4+3])
	}
	// Fuera de las tarjetas (hueco) debe quedar transparente.
	gapY := cards[0].Rect.Y + cards[0].Rect.H + 2
	if gapY < c.H {
		p := gapY*c.W + 20
		if c.Pix[p*4+3] > 10 {
			t.Errorf("el hueco entre tarjetas no es transparente: %d", c.Pix[p*4+3])
		}
	}
	if cards[1].Notification.Urgency != Critical {
		t.Errorf("urgencia perdida: %v", cards[1].Notification.Urgency)
	}
	if len(cards[1].Buttons) != 2 {
		t.Errorf("acciones dibujadas: %d", len(cards[1].Buttons))
	}
}

func TestRenderEmpty(t *testing.T) {
	r := testRenderer(300)
	c, cards := r.Render(nil, at(0))
	if len(cards) != 0 || c.W != 300 || c.H < 1 {
		t.Fatalf("lienzo vacío: %dx%d, %d tarjetas", c.W, c.H, len(cards))
	}
}

func TestRenderNarrowHidesLongActions(t *testing.T) {
	n := Notification{
		ID: 9, App: "app", Summary: "Resumen", Created: at(0), Progress: -1,
		Actions: []Action{{Key: "a", Label: "Una acción muy larga"}},
	}
	r := testRenderer(120)
	_, cards := r.Render([]Notification{n}, at(1))
	if len(cards) != 1 {
		t.Fatalf("tarjetas: %d", len(cards))
	}
	if len(cards[0].Buttons) != 0 {
		t.Errorf("una acción que no cabe no debe dibujarse: %+v", cards[0].Buttons)
	}
}

func TestCardHeightGrowsWithBody(t *testing.T) {
	r := testRenderer(360)
	short := Notification{Summary: "hola", Body: "una línea", Created: at(0), Progress: -1}
	long := Notification{Summary: "hola", Body: strings.Repeat("palabra ", 60), Created: at(0), Progress: -1}
	if r.cardHeight(&long, at(1)) <= r.cardHeight(&short, at(1)) {
		t.Error("un cuerpo largo debería ocupar más")
	}
}

func TestRenderInlineReplyButton(t *testing.T) {
	n := Notification{
		ID: 5, App: "Telegram", Summary: "Nuevo mensaje", Body: "¿Vienes?",
		Created: at(0), Progress: -1, InlineReply: true, ReplyPlaceholder: "Escribe…",
	}
	r := testRenderer(360)
	_, cards := r.Render([]Notification{n}, at(1))
	if len(cards) != 1 {
		t.Fatalf("tarjetas: %d", len(cards))
	}
	if len(cards[0].Buttons) != 1 {
		t.Fatalf("botones: %+v", cards[0].Buttons)
	}
	b := cards[0].Buttons[0]
	if b.Key != InlineReplyAction {
		t.Errorf("clave del botón: %q", b.Key)
	}
	if b.Label != "Escribe…" {
		t.Errorf("etiqueta: %q", b.Label)
	}
	if b.Rect.W <= 0 || b.Rect.H <= 0 {
		t.Errorf("rectángulo vacío: %+v", b.Rect)
	}
	// Sin texto de ayuda se usa «Responder».
	n2 := n
	n2.ReplyPlaceholder = ""
	_, cards2 := r.Render([]Notification{n2}, at(1))
	if got := cards2[0].Buttons[0].Label; got != "Responder" {
		t.Errorf("etiqueta por defecto: %q", got)
	}
}
