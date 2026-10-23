package osd

import (
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Options describe el aspecto del aviso en pantalla.
type Options struct {
	Width    int     // ancho lógico de la tarjeta
	Padding  int     // relleno interior
	Radius   int     // radio de las esquinas
	Opacity  float64 // opacidad del fondo (0..1)
	FontPath string  // fuente PSF (vacío = automática)
	Face     *font.Face
	Theme    ui.Palette
	Scale    int // escala de la fuente: 1, 2 o 3
	Shadow   bool
	Segments int // marcas de la barra de nivel
}

// Renderer dibuja los avisos en pantalla.
type Renderer struct {
	Opt  Options
	face *font.Face
	sc   int
}

// New crea un renderizador con valores razonables para lo que falte.
func New(opt Options) *Renderer {
	if opt.Width <= 0 {
		opt.Width = 260
	}
	if opt.Padding <= 0 {
		opt.Padding = 16
	}
	if opt.Radius < 0 {
		opt.Radius = 0
	}
	if opt.Opacity <= 0 || opt.Opacity > 1 {
		opt.Opacity = 0.95
	}
	if opt.Segments <= 0 {
		opt.Segments = 24
	}
	if opt.Theme.Name == "" {
		opt.Theme = ui.PaletteByName("catppuccin")
	}
	r := &Renderer{Opt: opt, face: opt.Face}
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

// Scale devuelve la escala en uso.
func (r *Renderer) Scale() int { return r.sc }

// rows es el alto de una línea sin interlineado.
func (r *Renderer) rows(scale int) int { return r.face.Rows(scale) }

// lineH es el alto de una línea con su interlineado.
func (r *Renderer) lineH(scale int) int {
	lead := 2 * scale
	if lead < 3 {
		lead = 3
	}
	return r.rows(scale) + lead
}

// valueScale es la escala del número grande.
func (r *Renderer) valueScale() int {
	if r.sc < 3 {
		return r.sc + 1
	}
	return r.sc
}

// barH es el alto de la barra de nivel.
func (r *Renderer) barH() int {
	h := 8 * r.sc
	if h < 6 {
		h = 6
	}
	return h
}

// Size devuelve el tamaño de la tarjeta, que depende de la fuente y de si hay
// que mostrar el nombre del dispositivo.
func (r *Renderer) Size(s State) (int, int) {
	h := 2 * r.Opt.Padding
	h += r.lineH(r.valueScale())
	h += 4 * r.sc
	h += r.barH()
	if s.Detail() != "" {
		h += 3*r.sc + r.lineH(r.sc)
	}
	return r.Opt.Width, h
}

// Render dibuja el aviso. alpha (0..1) multiplica la opacidad de todo para las
// animaciones de entrada y salida.
func (r *Renderer) Render(s State, alpha float64) *bar.Canvas {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	w, h := r.Size(s)
	c := bar.NewCanvas(w, h)
	if alpha <= 0 {
		return c
	}

	bg, border, muted, accent, warn, errc := r.colors(alpha)
	if r.Opt.Shadow {
		c.FillRound(0, 3, w, h-3, r.Opt.Radius, bar.RGBA{R: 0, G: 0, B: 0, A: uint8(60 * alpha)}, nil)
	}
	c.FillRound(0, 0, w, h, r.Opt.Radius, bg, &border)

	pad := r.Opt.Padding
	x := pad
	inner := w - 2*pad
	y := pad

	// Primera fila: qué es a la izquierda y el valor en grande a la derecha.
	// La etiqueta se alinea abajo para compartir la línea base con el número.
	label := strings.ToUpper(s.Label())
	c.Text(r.face, x, y+r.lineH(r.valueScale())-r.lineH(r.sc), label, r.sc, r.sc, muted)

	col := accent
	switch {
	case s.Muted:
		col = muted
	case s.Overdrive():
		col = warn
	}
	valor := s.ValueText()
	vw := bar.TextWidth(r.face, valor, r.valueScale(), r.valueScale())
	if vw > inner {
		valor = ui.Truncate(valor, inner/((r.face.W+r.valueScale())*r.valueScale()))
		vw = bar.TextWidth(r.face, valor, r.valueScale(), r.valueScale())
	}
	c.Text(r.face, x+inner-vw, y, valor, r.valueScale(), r.valueScale(), col)
	y += r.lineH(r.valueScale()) + 4*r.sc

	// Barra de nivel segmentada.
	track := border
	if track.A < 90 {
		track.A = 90
	}
	bh := r.barH()
	c.FillRound(x, y, inner, bh, bh/2, track, nil)
	fill := int(float64(inner) * s.Filled())
	if fill > 1 {
		fc := accent
		if s.Muted {
			fc = muted
		} else if s.Overdrive() {
			fc = warn
		}
		c.FillRound(x, y, fill, bh, bh/2, fc, nil)
	}
	// Marcas: separan los tramos para que el nivel se lea de un vistazo.
	seg := inner / r.Opt.Segments
	if seg >= 4 {
		for i := 1; i < r.Opt.Segments; i++ {
			sx := x + i*seg - 1
			if sx <= x+2 || sx >= x+fill-1 || sx >= x+inner-2 {
				continue
			}
			c.FillRect(sx, y+bh/4, 2, bh/2, bg)
		}
	}
	// Silenciado: una raya cruza la barra.
	if s.Muted {
		c.FillRect(x, y+bh/2-1, inner, 2, errc)
	}
	y += bh

	// Dispositivo de salida, si se conoce.
	if d := s.Detail(); d != "" {
		y += 3 * r.sc
		d = ui.Truncate(d, inner/((r.face.W+r.sc)*r.sc))
		c.Text(r.face, x, y, d, r.sc, r.sc, muted)
	}
	return c
}

// colors resuelve la paleta aplicando la opacidad y el factor de fundido.
func (r *Renderer) colors(alpha float64) (bg, border, muted, accent, warn, err bar.RGBA) {
	p := r.Opt.Theme
	bg = bar.ParseColor(string(p.Bg), r.Opt.Opacity*alpha)
	border = bar.ParseColor(string(p.Border), minF(1, (r.Opt.Opacity+0.12)*alpha))
	muted = bar.ParseColor(string(p.FgDim), alpha)
	accent = bar.ParseColor(string(p.Accent), alpha)
	warn = bar.ParseColor(string(p.Warn), alpha)
	err = bar.ParseColor(string(p.Err), alpha)
	return
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
