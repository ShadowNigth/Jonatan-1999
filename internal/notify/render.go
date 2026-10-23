package notify

import (
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
}

// Renderer dibuja notificaciones como tarjetas apiladas.
type Renderer struct {
	Opt  RenderOptions
	face *font.Face
	sc   int
}

// New crea un renderizador; si no se indica fuente, usa la mejor disponible.
func New(opt RenderOptions) *Renderer {
	if opt.Width <= 0 {
		opt.Width = 360
	}
	if opt.Padding <= 0 {
		opt.Padding = 14
	}
	if opt.Gap <= 0 {
		opt.Gap = 8
	}
	if opt.Radius < 0 {
		opt.Radius = 0
	}
	if opt.Opacity <= 0 {
		opt.Opacity = 0.94
	}
	if opt.MaxLines <= 0 {
		opt.MaxLines = 3
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
	h := 0
	for i := range items {
		h += r.cardHeight(&items[i], now)
		if i > 0 {
			h += r.Opt.Gap
		}
	}
	return h
}

// cardHeight calcula el alto de una tarjeta.
func (r *Renderer) cardHeight(n *Notification, now time.Time) int {
	pad := r.Opt.Padding
	h := 2 * pad
	inner := r.Opt.Width - 2*pad

	h += r.rows(r.sc) // fila de la app
	h += 4

	titleScale := r.sc
	if r.sc == 1 && r.Opt.Width >= 300 {
		titleScale = 2
	}
	titleLines := Wrap(Sanitize(n.Summary, 0), inner, r.advance(titleScale), 2)
	h += len(titleLines) * r.lineH(titleScale)
	if body := strings.TrimSpace(StripMarkup(n.Body)); body != "" {
		h += 4
		bodyLines := Wrap(body, inner, r.advance(r.sc), r.Opt.MaxLines)
		h += len(bodyLines) * r.lineH(r.sc)
	}
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

// Render dibuja las tarjetas y devuelve el lienzo junto con su distribución.
// El lienzo tiene ancho Width y el alto necesario (al menos 1 px).
func (r *Renderer) Render(items []Notification, now time.Time) (*bar.Canvas, []Card) {
	w := r.Opt.Width
	h := r.Measure(items, now)
	if h < 1 {
		h = 1
	}
	c := bar.NewCanvas(w, h)
	bg, border, fg, muted, accent, accent2, warn, err := r.colors()
	// El fondo se compone sobre el escritorio: se refuerza un poco para que el
	// texto se lea sobre cualquier fondo animado.
	if bg.A < 255 {
		bg.A = uint8(minF(255, float64(bg.A)+10))
	}

	cards := make([]Card, 0, len(items))
	y := 0
	for i := range items {
		n := &items[i]
		ch := r.cardHeight(n, now)
		rect := Rect{X: 0, Y: y, W: w, H: ch}
		card := Card{Notification: *n, Rect: rect}

		// Fondo y borde: la urgencia crítica se marca con borde de error.
		borderCol := border
		if n.Urgency == Critical {
			borderCol = err.WithAlpha(minF(1, float64(err.A)/255))
		}
		c.FillRound(rect.X, rect.Y, rect.W, rect.H, r.Opt.Radius, bg, &borderCol)

		pad := r.Opt.Padding
		x := rect.X + pad
		ty := rect.Y + pad
		inner := rect.W - 2*pad

		// Punto de urgencia, app y hora.
		dot := urgencyColor(n.Urgency, muted, accent, err, warn)
		cx, cy := x+3*r.sc, ty+r.rows(r.sc)/2+1
		c.FillRound(cx-3*r.sc, cy-3*r.sc, 6*r.sc, 6*r.sc, 3*r.sc, dot, nil)

		appX := x + 10*r.sc
		app := Sanitize(n.App, 26)
		if app == "" {
			app = "sistema"
		}
		c.Text(r.face, appX, ty, app, r.sc, r.sc, muted)

		stamp := n.Created.Format("15:04")
		sw := bar.TextWidth(r.face, stamp, r.sc, r.sc)
		c.Text(r.face, rect.X+rect.W-pad-sw, ty, stamp, r.sc, r.sc, muted)
		ty += r.rows(r.sc) + 4

		// Resumen.
		titleScale := r.sc
		if r.sc == 1 && r.Opt.Width >= 300 {
			titleScale = 2
		}
		for _, line := range Wrap(Sanitize(n.Summary, 0), inner, r.advance(titleScale), 2) {
			c.Text(r.face, x, ty, line, titleScale, titleScale, fg)
			ty += r.lineH(titleScale)
		}

		// Cuerpo.
		if body := strings.TrimSpace(StripMarkup(n.Body)); body != "" {
			ty += 4
			for _, line := range Wrap(body, inner, r.advance(r.sc), r.Opt.MaxLines) {
				c.Text(r.face, x, ty, line, r.sc, r.sc, muted)
				ty += r.lineH(r.sc)
			}
		}

		// Acciones y respuesta rápida: etiqueta entre corchetes.
		type boton struct {
			key, label string
			principal  bool
		}
		var botones []boton
		for _, a := range n.Actions {
			label := a.Label
			if label == "" {
				label = a.Key
			}
			botones = append(botones, boton{a.Key, label, a.Key == "default"})
		}
		if n.InlineReply {
			label := n.ReplyPlaceholder
			if label == "" {
				label = "Responder"
			}
			botones = append(botones, boton{InlineReplyAction, label, true})
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
				card.Buttons = append(card.Buttons, Button{
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

		cards = append(cards, card)
		y += ch + r.Opt.Gap
	}
	return c, cards
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
