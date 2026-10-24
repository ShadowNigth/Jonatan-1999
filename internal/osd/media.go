package osd

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// MediaState son los metadatos de reproducción que se presentan en la tarjeta.
type MediaState struct {
	Title          string
	Artist         string
	Album          string
	PlaybackStatus string
	Artwork        image.Image
}

// MediaRenderer dibuja la tarjeta de pista usando la misma paleta y geometría
// que el OSD de volumen.
type MediaRenderer struct {
	Opt  Options
	face *font.Face
	sc   int
}

// NewMedia crea el renderizador del OSD de reproducción.
func NewMedia(opt Options) *MediaRenderer {
	base := New(opt)
	return &MediaRenderer{Opt: base.Opt, face: base.face, sc: base.sc}
}

func (r *MediaRenderer) coverSize() int {
	size := 64 + (r.sc-2)*8
	if size < 52 {
		size = 52
	}
	if size > 88 {
		size = 88
	}
	return size
}

func (r *MediaRenderer) lineH(scale int) int {
	h := r.face.Rows(scale) + 2*scale
	if h < 1 {
		h = 1
	}
	return h
}

// Size devuelve las dimensiones de la tarjeta.
func (r *MediaRenderer) Size(s MediaState) (int, int) {
	cover := r.coverSize()
	small := r.sc - 1
	if small < 1 {
		small = 1
	}
	content := r.lineH(small) + 2*r.lineH(r.sc)
	if strings.TrimSpace(s.Album) != "" {
		content += r.lineH(small)
	}
	if content < cover {
		content = cover
	}
	return r.Opt.Width, content + 2*r.Opt.Padding
}

// Render dibuja el OSD de reproducción. alpha (0..1) se usa en los fundidos.
func (r *MediaRenderer) Render(s MediaState, alpha float64) *bar.Canvas {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	w, h := r.Size(s)
	c := bar.NewCanvas(w, h)
	if alpha == 0 {
		return c
	}

	p := r.Opt.Theme
	bg := bar.ParseColor(string(p.Bg), r.Opt.Opacity*alpha)
	border := bar.ParseColor(string(p.Border), math.Min(1, (r.Opt.Opacity+0.12)*alpha))
	muted := bar.ParseColor(string(p.FgDim), alpha)
	fg := bar.ParseColor(string(p.Fg), alpha)
	accent := bar.ParseColor(string(p.Accent), alpha)
	if r.Opt.Shadow {
		c.FillRound(0, 3, w, h-3, r.Opt.Radius, bar.RGBA{R: 0, G: 0, B: 0, A: uint8(60 * alpha)}, nil)
	}
	c.FillRound(0, 0, w, h, r.Opt.Radius, bg, &border)

	pad := r.Opt.Padding
	cover := r.coverSize()
	coverX := pad
	coverY := (h - cover) / 2
	coverRadius := cover / 6
	c.FillRound(coverX, coverY, cover, cover, coverRadius, bar.ParseColor(string(p.BgAlt), alpha), &border)
	if s.Artwork != nil {
		drawCover(c, s.Artwork, coverX+1, coverY+1, cover-2, maxInt(0, coverRadius-1), alpha)
	} else {
		drawEqualizer(c, coverX, coverY, cover, accent)
	}

	textX := coverX + cover + 14
	textW := w - pad - textX
	small := r.sc - 1
	if small < 1 {
		small = 1
	}
	textH := r.lineH(small) + 2*r.lineH(r.sc)
	if strings.TrimSpace(s.Album) != "" {
		textH += r.lineH(small)
	}
	textY := (h - textH) / 2

	heading := "REPRODUCTOR"
	switch strings.ToLower(strings.TrimSpace(s.PlaybackStatus)) {
	case "playing":
		heading = "REPRODUCIENDO"
	case "paused":
		heading = "EN PAUSA"
	}
	c.Text(r.face, textX, textY, truncateMediaText(r.face, heading, textW, small, small), small, small, muted)
	textY += r.lineH(small)

	title := strings.TrimSpace(s.Title)
	if title == "" {
		title = "Sin título"
	}
	c.Text(r.face, textX, textY, truncateMediaText(r.face, title, textW, r.sc, r.sc), r.sc, r.sc, accent)
	textY += r.lineH(r.sc)

	artist := strings.TrimSpace(s.Artist)
	if artist == "" {
		artist = "Artista desconocido"
	}
	c.Text(r.face, textX, textY, truncateMediaText(r.face, artist, textW, r.sc, r.sc), r.sc, r.sc, fg)
	textY += r.lineH(r.sc)
	if album := strings.TrimSpace(s.Album); album != "" {
		c.Text(r.face, textX, textY, truncateMediaText(r.face, album, textW, small, small), small, small, muted)
	}
	return c
}

func truncateMediaText(face *font.Face, text string, width, scale, spacing int) string {
	cell := (face.W + spacing) * scale
	if cell <= 0 || width <= 0 {
		return ""
	}
	maxCells := width / cell
	for maxCells > 0 {
		out := ui.Truncate(text, maxCells)
		if face.Measure(out, scale, spacing) <= width {
			return out
		}
		maxCells--
	}
	return ""
}

func drawCover(c *bar.Canvas, img image.Image, x, y, size, radius int, alpha float64) {
	if img == nil || size <= 0 {
		return
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return
	}
	crop := sw
	if sh < crop {
		crop = sh
	}
	offX := (sw - crop) / 2
	offY := (sh - crop) / 2
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			if !insideRoundedRect(dx, dy, size, size, radius) {
				continue
			}
			sx := b.Min.X + offX + int((float64(dx)+0.5)*float64(crop)/float64(size))
			sy := b.Min.Y + offY + int((float64(dy)+0.5)*float64(crop)/float64(size))
			col := color.NRGBAModel.Convert(img.At(sx, sy)).(color.NRGBA)
			col.A = uint8(float64(col.A) * alpha)
			c.Over(x+dx, y+dy, bar.RGBA{R: col.R, G: col.G, B: col.B, A: col.A})
		}
	}
}

func insideRoundedRect(x, y, w, h, radius int) bool {
	if radius <= 0 {
		return true
	}
	dx, dy := 0, 0
	switch {
	case x < radius && y < radius:
		dx, dy = radius-x-1, radius-y-1
	case x >= w-radius && y < radius:
		dx, dy = x-(w-radius), radius-y-1
	case x < radius && y >= h-radius:
		dx, dy = radius-x-1, y-(h-radius)
	case x >= w-radius && y >= h-radius:
		dx, dy = x-(w-radius), y-(h-radius)
	}
	return dx == 0 && dy == 0 || dx*dx+dy*dy <= radius*radius
}

func drawEqualizer(c *bar.Canvas, x, y, size int, col bar.RGBA) {
	barW := maxInt(3, size/12)
	gap := maxInt(3, size/10)
	heights := []int{size / 3, size / 2, size * 2 / 3, size / 2, size / 3}
	totalW := len(heights)*barW + (len(heights)-1)*gap
	startX := x + (size-totalW)/2
	midY := y + size/2
	for i, height := range heights {
		c.FillRound(startX+i*(barW+gap), midY-height/2, barW, height, barW/2, col, nil)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
