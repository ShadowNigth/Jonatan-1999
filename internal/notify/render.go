package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Options de aspecto de las tarjetas.
type RenderOptions struct {
	Width    int     // ancho lógico de cada tarjeta
	Padding  int     // relleno interior
	Gap      int     // separación entre tarjetas
	Radius   int     // radio de las esquinas
	Opacity  float64 // opacidad del fondo
	FontPath string
	Face     *font.Face
	Theme    ui.Palette
	Scale    int // escala de la fuente (1 o 2)
	Shadow   bool
	MaxLines int // líneas de cuerpo como máximo
}

// Rect es un área de la superficie.
type Rect struct{ X, Y, W, H int }

// Button es un botón dibujado dentro de una tarjeta: sus acciones normales o
// el campo de respuesta rápida (clave InlineReplyAction).
type Button struct {
	Key   string
	Label string
	Rect  Rect
}

// Card es una tarjeta dibujada, con su notificación y su área.
type Card struct {
	Notification Notification
	Rect         Rect
	Buttons      []Button

	// GroupKey identifica el grupo de la tarjeta ("" si no pertenece a uno).
	GroupKey string
	// GroupCount es cuántas notificaciones tiene el grupo.
	GroupCount int
	// Collapsed marca la tarjeta que resume un grupo plegado.
	Collapsed bool
	// Header marca la franja de un grupo desplegado.
	Header bool
	// Toggle indica que un clic en la tarjeta pliega o despliega el grupo.
	Toggle bool
	// IDs son las notificaciones que representa la tarjeta (el grupo entero en
	// la cabecera y en la pila plegada).
	IDs []uint32
}

// GroupExpandAction es la clave del botón que despliega una pila.
const GroupExpandAction = "group-expand"

// GroupCollapseAction es la clave del botón que pliega un grupo abierto.
const GroupCollapseAction = "group-collapse"

// Renderer dibuja notificaciones como tarjetas apiladas.
type Renderer struct {
	Opt  RenderOptions
	face *font.Face
	sc   int
}

// New crea un renderizador; si no se indica fuente, usa la mejor disponible.
func New(opt RenderOptions) *Renderer {
	if opt.Width <= 0 {
		opt.Width = 300
	}
	if opt.Padding <= 0 {
		opt.Padding = 10
	}
	if opt.Gap <= 0 {
		opt.Gap = 6
	}
	if opt.Radius < 0 {
		opt.Radius = 0
	}
	if opt.Opacity <= 0 {
		opt.Opacity = 0.94
	}
	if opt.MaxLines <= 0 {
		opt.MaxLines = 2
	}
	if opt.Theme.Name == "" {
		opt.Theme = ui.PaletteByName("catppuccin")
	}
	r := &Renderer{Opt: opt}
	r.face = opt.Face
	if r.face == nil {
		r.face = font.Best(opt.FontPath)
	}
	r.sc = opt.Scale
	if r.sc < 1 {
		r.sc = 1
	}
	return r
}

// Face devuelve la fuente en uso.
func (r *Renderer) Face() *font.Face { return r.face }

// Scale devuelve la escala de la fuente.
func (r *Renderer) Scale() int { return r.sc }

// advance es el ancho de avance de un carácter en la escala dada. El espaciado
// entre letras es igual a la escala (como en Canvas.Text), así que la celda es
// (ancho + espaciado) * escala.
func (r *Renderer) advance(scale int) int { return (r.face.W + scale) * scale }

func (r *Renderer) rows(scale int) int { return r.face.Rows(scale) }

// lead es el aire vertical entre líneas. Sin él las líneas de una fuente de
// mapa de bits quedan pegadas (y a escala 2 se tocan).
func (r *Renderer) lead(scale int) int {
	l := 2 * scale
	if l < 3 {
		l = 3
	}
	return l
}

// lineH es el alto de una línea de texto con su interlineado.
func (r *Renderer) lineH(scale int) int { return r.rows(scale) + r.lead(scale) }

// titleScale destaca los resúmenes solo en tarjetas anchas; con el ancho
// compacto predeterminado evita duplicar el tamaño del título.
func (r *Renderer) titleScale() int {
	if r.sc == 1 && r.Opt.Width >= 360 {
		return 2
	}
	return r.sc
}

// colors devuelve la paleta resuelta.
func (r *Renderer) colors() (bg, border, fg, muted, accent, accent2, warn, err bar.RGBA) {
	p := r.Opt.Theme
	bg = bar.ParseColor(string(p.Bg), r.Opt.Opacity)
	border = bar.ParseColor(string(p.Border), minF(1, r.Opt.Opacity+0.12))
	fg = bar.ParseColor(string(p.Fg), 1)
	muted = bar.ParseColor(string(p.FgDim), 1)
	accent = bar.ParseColor(string(p.Accent), 1)
	accent2 = bar.ParseColor(string(p.Accent2), 1)
	warn = bar.ParseColor(string(p.Warn), 1)
	err = bar.ParseColor(string(p.Err), 1)
	return
}

// urgencyColor elige el color del punto según la urgencia.
func urgencyColor(u Urgency, muted, accent, err, warn bar.RGBA) bar.RGBA {
	switch u {
	case Critical:
		return err
	case Low:
		return muted
	}
	return accent
}

// Measure devuelve el alto total que ocuparían las tarjetas dadas.
func (r *Renderer) Measure(items []Notification, now time.Time) int {
	views := make([]GroupView, 0, len(items))
	for _, n := range items {
		views = append(views, GroupView{Key: GroupKey(n), Items: []Notification{n}})
	}
	return r.MeasureGroups(views, now)
}

// GroupView es un grupo tal como se quiere dibujar: sus notificaciones (la más
// reciente primero) y si está desplegado.
type GroupView struct {
	Key      string
	Items    []Notification
	Expanded bool
}

// Views construye las vistas a partir de la lista plana de notificaciones y
// del estado de plegado. Con stack nil todos los grupos van plegados.
func Views(items []Notification, stack *Stack) []GroupView {
	groups := GroupAll(items)
	out := make([]GroupView, 0, len(groups))
	for _, g := range groups {
		out = append(out, GroupView{Key: g.Key, Items: g.Items, Expanded: stack.Expanded(g.Key)})
	}
	return out
}

// layoutEntry es una tarjeta en la lista de dibujo.
type layoutEntry struct {
	n      Notification
	group  Card // metadatos de agrupación (sin rect)
	layers int  // capas de pila detrás de la tarjeta
	header bool // franja de un grupo desplegado
	app    string
}

// entry cuenta las notificaciones del grupo (para las insignias).
func (r *Renderer) build(views []GroupView) []layoutEntry {
	var out []layoutEntry
	for _, v := range views {
		if len(v.Items) == 0 {
			continue
		}
		key := v.Key
		if key == "" {
			key = GroupKey(v.Items[0])
		}
		ids := make([]uint32, 0, len(v.Items))
		for _, n := range v.Items {
			ids = append(ids, n.ID)
		}
		if len(v.Items) == 1 {
			n := v.Items[0]
			out = append(out, layoutEntry{
				n:     n,
				app:   appLabel(n),
				group: Card{GroupKey: key, GroupCount: 1, IDs: ids},
			})
			continue
		}
		if v.Expanded {
			out = append(out, layoutEntry{
				header: true,
				group:  Card{GroupKey: key, GroupCount: len(v.Items), Header: true, Toggle: true, IDs: ids},
				app:    groupApp(v.Items),
			})
			for _, n := range v.Items {
				out = append(out, layoutEntry{
					n:     n,
					app:   appLabel(n),
					group: Card{GroupKey: key, GroupCount: len(v.Items), IDs: []uint32{n.ID}},
				})
			}
			continue
		}
		// Grupo plegado: se resume en una tarjeta con la notificación más
		// reciente y una pila dibujada detrás.
		latest := v.Items[0]
		summary := latest
		summary.Summary = groupPreview(v.Items)
		if len(v.Items) > 1 {
			summary.Body = ""
		}
		out = append(out, layoutEntry{
			n:      summary,
			layers: 2,
			app:    groupApp(v.Items),
			group: Card{GroupKey: key, GroupCount: len(v.Items), Collapsed: true,
				Toggle: true, IDs: ids},
		})
	}
	return out
}

// appLabel es el nombre de la aplicación para la cabecera.
func appLabel(n Notification) string {
	if a := Sanitize(n.App, 26); a != "" {
		return a
	}
	return "sistema"
}

// groupApp es el nombre común de un grupo.
func groupApp(items []Notification) string {
	for _, n := range items {
		if a := Sanitize(n.App, 26); a != "" {
			return a
		}
	}
	return "sistema"
}

// groupPreview resume un grupo plegado.
func groupPreview(items []Notification) string {
	if len(items) == 0 {
		return ""
	}
	text := Sanitize(items[0].Summary, 0)
	if text == "" {
		text = Sanitize(StripMarkup(items[0].Body), 0)
	}
	if rest := len(items) - 1; rest > 0 {
		if text == "" {
			return fmt.Sprintf("%d avisos", len(items))
		}
		return fmt.Sprintf("%s · y %d más", text, rest)
	}
	return text
}

// stackStep es cuánto asoma cada capa de la pila.
func (r *Renderer) stackStep() int {
	s := 3 * r.sc
	if s < 3 {
		s = 3
	}
	return s
}

// thumbSize es el lado de la miniatura de una notificación con imagen.
func (r *Renderer) thumbSize() int {
	s := 30 * r.sc
	if s < 24 {
		s = 24
	}
	if s > 64 {
		s = 64
	}
	return s
}

// thumbGap es la separación entre la miniatura y el texto.
func (r *Renderer) thumbGap() int { return 5 * r.sc }

// textCols devuelve el desplazamiento y el ancho disponibles para el texto
// (la miniatura, si la hay, ocupa la izquierda).
func (r *Renderer) textCols(n *Notification, inner int) (int, int) {
	if !n.HasImage() {
		return 0, inner
	}
	used := r.thumbSize() + r.thumbGap()
	if inner-used < 60*r.sc {
		// Sin sitio para la miniatura: mejor el texto completo.
		return 0, inner
	}
	return used, inner - used
}

// cardHeight calcula el alto de una tarjeta según su contenido.
func (r *Renderer) cardHeight(n *Notification, now time.Time) int {
	pad := r.Opt.Padding
	h := 2 * pad
	inner := r.Opt.Width - 2*pad

	h += r.rows(r.sc) // fila de la app
	h += 4
	h += r.contentHeight(n, inner)

	if len(n.Actions) > 0 || n.InlineReply {
		h += 6
		h += r.rows(r.sc)
	}
	if n.Progress >= 0 {
		h += 8
	}
	if _, timed := n.Remaining(now); timed {
		h += 4
	}
	return h
}

// contentHeight es el alto del bloque de título y cuerpo, con la miniatura si
// es más alta que el texto.
func (r *Renderer) contentHeight(n *Notification, inner int) int {
	titleScale := r.titleScale()
	_, cols := r.textCols(n, inner)
	h := len(Wrap(Sanitize(n.Summary, 0), cols, r.advance(titleScale), 2)) * r.lineH(titleScale)
	if body := strings.TrimSpace(StripMarkup(n.Body)); body != "" {
		h += 4
		h += len(Wrap(body, cols, r.advance(r.sc), r.Opt.MaxLines)) * r.lineH(r.sc)
	}
	if n.HasImage() {
		if t := r.thumbSize(); t > h {
			h = t
		}
	}
	return h
}

// headerHeight es el alto de la franja de un grupo desplegado.
func (r *Renderer) headerHeight() int {
	pad := r.Opt.Padding / 2
	if pad < 6 {
		pad = 6
	}
	return 2*pad + r.rows(r.sc)
}

// entryHeight calcula el alto de una entrada del dibujo.
func (r *Renderer) entryHeight(e layoutEntry, now time.Time) int {
	h := 0
	if e.header {
		h = r.headerHeight()
	} else {
		h = r.cardHeight(&e.n, now)
		if e.group.Collapsed && e.group.GroupCount > 1 {
			// Una línea más para el botón de abrir el grupo, salvo que la
			// tarjeta ya tenga una fila de botones.
			if len(e.n.Actions) == 0 && !e.n.InlineReply {
				h += 6 + r.rows(r.sc)
			}
		}
	}
	return h + e.layers*r.stackStep()
}

// MeasureGroups devuelve el alto total de las vistas dadas.
func (r *Renderer) MeasureGroups(views []GroupView, now time.Time) int {
	entries := r.build(views)
	h := 0
	for i, e := range entries {
		if i > 0 {
			h += r.Opt.Gap
		}
		h += r.entryHeight(e, now)
	}
	return h
}

// Render dibuja las tarjetas y devuelve el lienzo junto con su distribución.
func (r *Renderer) Render(items []Notification, now time.Time) (*bar.Canvas, []Card) {
	views := make([]GroupView, 0, len(items))
	for _, n := range items {
		views = append(views, GroupView{Key: GroupKey(n), Items: []Notification{n}})
	}
	return r.RenderGroups(views, now)
}

// RenderGroups dibuja los grupos: las pilas plegadas como una tarjeta con
// capas detrás y los grupos desplegados como una franja y sus tarjetas.
func (r *Renderer) RenderGroups(views []GroupView, now time.Time) (*bar.Canvas, []Card) {
	w := r.Opt.Width
	h := r.MeasureGroups(views, now)
	if h < 1 {
		h = 1
	}
	c := bar.NewCanvas(w, h)
	bg, border, fg, muted, accent, accent2, warn, err := r.colors()
	if bg.A < 255 {
		bg.A = uint8(minF(255, float64(bg.A)+10))
	}

	entries := r.build(views)
	cards := make([]Card, 0, len(entries))
	y := 0
	for i, e := range entries {
		if i > 0 {
			y += r.Opt.Gap
		}
		ch := r.entryHeight(e, now)
		rect := Rect{X: 0, Y: y, W: w, H: ch}
		card := e.group
		card.Notification = e.n
		card.Rect = rect

		// Capas de la pila, de la más al fondo a la de delante.
		step := r.stackStep()
		bodyH := rect.H - e.layers*step
		for layer := e.layers; layer >= 1; layer-- {
			// Cada capa asoma por debajo y por los lados, como una baraja.
			inset := layer * 4 * r.sc
			top := rect.Y + layer*step
			lh := bodyH
			if lh < 4 {
				lh = 4
			}
			col := bg
			col.A = uint8(float64(col.A) * 0.75)
			lb := border
			lb.A = uint8(float64(lb.A) * 0.7)
			c.FillRound(rect.X+inset, top, rect.W-2*inset, lh, r.Opt.Radius, col, &lb)
		}

		if e.header {
			card.Buttons = r.drawHeader(c, e, rect, bg, border, muted, accent)
			cards = append(cards, card)
			y += ch
			continue
		}

		body := Rect{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H - e.layers*step}
		card.Buttons = r.drawCard(c, e, body, appLabelOf(e), now,
			bg, border, fg, muted, accent, accent2, warn, err)
		cards = append(cards, card)
		y += ch
	}
	return c, cards
}

// appLabelOf devuelve la etiqueta de la cabecera de una entrada.
func appLabelOf(e layoutEntry) string {
	if e.app != "" {
		return e.app
	}
	return appLabel(e.n)
}

// drawHeader dibuja la franja que corona un grupo desplegado y devuelve su
// botón de plegar.
func (r *Renderer) drawHeader(c *bar.Canvas, e layoutEntry, rect Rect, bg, border, muted, accent bar.RGBA) []Button {
	pad := r.Opt.Padding
	hbg := bg
	hbg.A = uint8(minF(255, float64(bg.A)+14))
	c.FillRound(rect.X, rect.Y, rect.W, rect.H, r.Opt.Radius/2, hbg, &border)
	y := rect.Y + (rect.H-r.rows(r.sc))/2
	title := fmt.Sprintf("%s · %d avisos", appLabelOf(e), e.group.GroupCount)
	c.Text(r.face, rect.X+pad, y, truncateRunes(title, 40), r.sc, r.sc, accent)

	label := "[plegar]"
	lw := bar.TextWidth(r.face, label, r.sc, r.sc)
	bx := rect.X + rect.W - pad - lw
	c.Text(r.face, bx, y, label, r.sc, r.sc, muted)
	return []Button{{
		Key: GroupCollapseAction, Label: "plegar",
		Rect: Rect{X: bx, Y: y - 2, W: lw, H: r.rows(r.sc) + 4},
	}}
}

// localizedActionLabel traduce acciones genéricas frecuentes al español, sin
// alterar etiquetas específicas que envíe cada aplicación.
func localizedActionLabel(label, key string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = key
	}
	switch strings.ToLower(label) {
	case "default", "open", "open app", "view", "show":
		return "Abrir"
	case "reply", "respond", "answer":
		return "Responder"
	case "dismiss", "close":
		return "Cerrar"
	case "later":
		return "Luego"
	case "snooze":
		return "Posponer"
	case "mark as read":
		return "Marcar leído"
	case "accept":
		return "Aceptar"
	case "decline", "reject":
		return "Rechazar"
	case "delete", "remove":
		return "Eliminar"
	case "play":
		return "Reproducir"
	case "pause":
		return "Pausar"
	case "stop":
		return "Detener"
	case "cancel":
		return "Cancelar"
	default:
		return label
	}
}

// drawCard dibuja una tarjeta normal (o la que resume un grupo plegado).
func (r *Renderer) drawCard(c *bar.Canvas, e layoutEntry, rect Rect, app string,
	now time.Time, bg, border, fg, muted, accent, accent2, warn, err bar.RGBA) []Button {
	n := &e.n
	var buttons []Button

	borderCol := border
	if n.Urgency == Critical {
		borderCol = err.WithAlpha(minF(1, float64(err.A)/255))
	}
	c.FillRound(rect.X, rect.Y, rect.W, rect.H, r.Opt.Radius, bg, &borderCol)

	pad := r.Opt.Padding
	x := rect.X + pad
	inner := rect.W - 2*pad
	ty := rect.Y + pad

	// Punto de urgencia, aplicación y hora.
	dot := urgencyColor(n.Urgency, muted, accent, err, warn)
	if e.group.Collapsed && e.group.GroupCount > 1 {
		dot = accent2
	}
	cx, cy := x+3*r.sc, ty+r.rows(r.sc)/2+1
	c.FillRound(cx-3*r.sc, cy-3*r.sc, 6*r.sc, 6*r.sc, 3*r.sc, dot, nil)

	appX := x + 10*r.sc
	c.Text(r.face, appX, ty, app, r.sc, r.sc, muted)

	stamp := n.Created.Format("15:04")
	sw := bar.TextWidth(r.face, stamp, r.sc, r.sc)
	c.Text(r.face, rect.X+rect.W-pad-sw, ty, stamp, r.sc, r.sc, muted)
	if e.group.Collapsed && e.group.GroupCount > 1 {
		// Contador del grupo, como una pastilla a la izquierda de la hora.
		count := fmt.Sprintf("%d", e.group.GroupCount)
		cw := bar.TextWidth(r.face, count, r.sc, r.sc)
		chipW, chipH := cw+6*r.sc, r.rows(r.sc)
		chipX := rect.X + rect.W - pad - sw - 6*r.sc - chipW
		c.FillRound(chipX, ty, chipW, chipH, chipH/2, accent2, nil)
		c.Text(r.face, chipX+3*r.sc, ty, count, r.sc, r.sc, bg)
	}
	ty += r.rows(r.sc) + 4

	// Miniatura a la izquierda, si la tarjeta trae imagen.
	indent, cols := r.textCols(n, inner)
	tx := x + indent
	if indent > 0 {
		r.drawThumb(c, n.Image, x, ty, r.thumbSize())
	}

	// Resumen.
	titleScale := r.titleScale()
	for _, line := range Wrap(Sanitize(n.Summary, 0), cols, r.advance(titleScale), 2) {
		c.Text(r.face, tx, ty, line, titleScale, titleScale, fg)
		ty += r.lineH(titleScale)
	}

	// Cuerpo.
	if body := strings.TrimSpace(StripMarkup(n.Body)); body != "" {
		ty += 4
		for _, line := range Wrap(body, cols, r.advance(r.sc), r.Opt.MaxLines) {
			c.Text(r.face, tx, ty, line, r.sc, r.sc, muted)
			ty += r.lineH(r.sc)
		}
	}
	// El bloque de contenido manda: si la miniatura es más alta, los botones
	// y las barras empiezan debajo de ella.
	contentTop := rect.Y + pad + r.rows(r.sc) + 4
	contentBottom := contentTop + r.contentHeight(n, inner)
	if ty < contentBottom {
		ty = contentBottom
	}

	// Acciones, respuesta rápida y el botón de abrir la pila.
	type boton struct {
		key, label string
		principal  bool
	}
	var botones []boton
	for _, a := range n.Actions {
		if a.Key == InlineReplyAction {
			// Se dibuja con el resto de la fila, no dos veces.
			continue
		}
		label := localizedActionLabel(a.Label, a.Key)
		botones = append(botones, boton{a.Key, label, a.Key == "default"})
	}
	if n.InlineReply {
		label := n.ReplyPlaceholder
		// Si el botón de la pila va a compartir fila, se acorta para que
		// quepan los dos.
		if e.group.Collapsed && e.group.GroupCount > 1 {
			label = "Responder"
		}
		if label == "" {
			label = "Responder"
		}
		botones = append(botones, boton{InlineReplyAction, label, true})
	}
	if e.group.Collapsed && e.group.GroupCount > 1 {
		botones = append(botones, boton{GroupExpandAction, fmt.Sprintf("ver %d", e.group.GroupCount), false})
	}
	if len(botones) > 0 {
		ty += 6
		bx := x
		for _, b := range botones {
			txt := "[" + b.label + "]"
			tw := bar.TextWidth(r.face, txt, r.sc, r.sc)
			if bx+tw > rect.X+rect.W-pad {
				break
			}
			col := accent2
			if b.principal {
				col = accent
			}
			c.Text(r.face, bx, ty, txt, r.sc, r.sc, col)
			buttons = append(buttons, Button{
				Key: b.key, Label: b.label,
				Rect: Rect{X: bx, Y: ty - 2, W: tw, H: r.rows(r.sc) + 4},
			})
			bx += tw + 8*r.sc
		}
		ty += r.rows(r.sc)
	}

	// Progreso (hint `progress`).
	if n.Progress >= 0 {
		ty += 4
		ph := 3 * r.sc
		c.FillRound(x, ty, inner, ph, ph/2, border, nil)
		fill := inner * n.Progress / 100
		if fill > 0 {
			c.FillRound(x, ty, fill, ph, ph/2, accent, nil)
		}
		ty += ph + 4
	}

	// Barra de tiempo restante.
	if frac, timed := n.Remaining(now); timed {
		bh := 2 * r.sc
		by := rect.Y + rect.H - bh - 2*r.sc
		if by < ty {
			by = ty
		}
		c.FillRound(x, by, inner, bh, bh/2, border, nil)
		if wpx := int(float64(inner) * frac); wpx > 1 {
			c.FillRound(x, by, wpx, bh, bh/2, accent2, nil)
		}
	}
	return buttons
}

// drawThumb dibuja una imagen cuadrada con las esquinas redondeadas y un
// borde tenue; los píxeles con alfa se mezclan con lo que haya debajo.
func (r *Renderer) drawThumb(c *bar.Canvas, im *Image, x, y, side int) {
	if im.Empty() || side <= 0 {
		return
	}
	thumb := im.Thumbnail(side)
	rad := side / 6
	if rad < 2 {
		rad = 2
	}
	// Máscara: se pinta un rectángulo redondeado opaco y se recortan los
	// píxeles de fuera con una comprobación de esquina.
	for py := 0; py < thumb.H; py++ {
		for px := 0; px < thumb.W; px++ {
			if !insideRound(px, py, thumb.W, thumb.H, rad) {
				continue
			}
			sr, sg, sb, sa := thumb.At(px, py)
			if sa == 0 {
				continue
			}
			c.Over(x+px, y+py, bar.RGBA{R: sr, G: sg, B: sb, A: sa})
		}
	}
	// Marco.
	borderCol := bar.RGBA{R: 255, G: 255, B: 255, A: 40}
	for py := 0; py < thumb.H; py++ {
		for px := 0; px < thumb.W; px++ {
			edge := py == 0 || px == 0 || py == thumb.H-1 || px == thumb.W-1
			if !edge || !insideRound(px, py, thumb.W, thumb.H, rad) {
				continue
			}
			c.Over(x+px, y+py, borderCol)
		}
	}
}

// insideRound indica si un punto cae dentro de un rectángulo de esquinas
// redondeadas de radio rad.
func insideRound(x, y, w, h, rad int) bool {
	if rad <= 0 {
		return true
	}
	cx, cy := x, y
	switch {
	case x < rad && y < rad:
		cx, cy = rad, rad
	case x >= w-rad && y < rad:
		cx, cy = w-rad-1, rad
	case x < rad && y >= h-rad:
		cx, cy = rad, h-rad-1
	case x >= w-rad && y >= h-rad:
		cx, cy = w-rad-1, h-rad-1
	default:
		return true
	}
	dx, dy := float64(x-cx), float64(y-cy)
	return dx*dx+dy*dy <= float64(rad*rad)
}

// truncateRunes recorta por caracteres completos con puntos suspensivos.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
