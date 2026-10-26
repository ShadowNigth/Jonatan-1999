// Package bar dibuja una barra de estado propia en píxeles, lista para subirla
// a una superficie de Wayland (wlr-layer-shell) sin usar GTK, Qt ni Cairo.
package bar

import (
	"image"
	"math"

	"github.com/ShadowNigth/swaydesk/internal/font"
)

// RGBA es un color con alfa no premultiplicado.
type RGBA struct{ R, G, B, A uint8 }

// ParseColor convierte "#rrggbb" o "#rrggbbaa" en un color con alfa.
func ParseColor(hex string, alpha float64) RGBA {
	c := RGBA{A: 255}
	s := hex
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	val := func(i int) uint8 {
		if i+1 >= len(s) {
			return 0
		}
		d := func(ch byte) int {
			switch {
			case ch >= '0' && ch <= '9':
				return int(ch - '0')
			case ch >= 'a' && ch <= 'f':
				return int(ch-'a') + 10
			case ch >= 'A' && ch <= 'F':
				return int(ch-'A') + 10
			}
			return 0
		}
		return uint8(d(s[i])<<4 | d(s[i+1]))
	}
	if len(s) >= 6 {
		c.R, c.G, c.B = val(0), val(2), val(4)
	}
	if len(s) == 8 {
		c.A = val(6)
	}
	if alpha >= 0 {
		c.A = uint8(math.Round(math.Max(0, math.Min(1, alpha)) * 255))
	}
	return c
}

// WithAlpha devuelve el mismo color con otro alfa.
func (c RGBA) WithAlpha(a float64) RGBA {
	if a < 0 {
		a = 0
	}
	if a > 1 {
		a = 1
	}
	c.A = uint8(math.Round(a * 255))
	return c
}

// Uint8 devuelve los componentes RGBA.
func (c RGBA) Uint8() (uint8, uint8, uint8, uint8) { return c.R, c.G, c.B, c.A }

// Blend mezcla dos colores (t = 0 → a, t = 1 → b), conservando el alfa de a.
func Blend(a, b RGBA, t float64) RGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		b.A = a.A
		return b
	}
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), a.A}
}

// Canvas es un lienzo RGBA con alfa (fondo transparente por defecto).
type Canvas struct {
	W, H int
	Pix  []uint8 // 4 bytes por píxel: R, G, B, A
}

// NewCanvas crea un lienzo completamente transparente.
func NewCanvas(w, h int) *Canvas {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return &Canvas{W: w, H: h, Pix: make([]uint8, w*h*4)}
}

// Set escribe un píxel sin mezclar.
func (c *Canvas) Set(x, y int, col RGBA) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	i := (y*c.W + x) * 4
	c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = col.R, col.G, col.B, col.A
}

// Over mezcla un color sobre el píxel existente (source-over).
func (c *Canvas) Over(x, y int, col RGBA) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	if col.A == 0 {
		return
	}
	i := (y*c.W + x) * 4
	if col.A == 255 {
		c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = col.R, col.G, col.B, 255
		return
	}
	if c.Pix[i+3] == 0 {
		c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = col.R, col.G, col.B, col.A
		return
	}
	if c.Pix[i+3] == 255 {
		sa := uint32(col.A)
		inv := uint32(255 - col.A)
		c.Pix[i] = uint8((uint32(col.R)*sa + uint32(c.Pix[i])*inv + 127) / 255)
		c.Pix[i+1] = uint8((uint32(col.G)*sa + uint32(c.Pix[i+1])*inv + 127) / 255)
		c.Pix[i+2] = uint8((uint32(col.B)*sa + uint32(c.Pix[i+2])*inv + 127) / 255)
		return
	}
	sa := float64(col.A) / 255
	da := float64(c.Pix[i+3]) / 255
	outA := sa + da*(1-sa)
	if outA <= 0 {
		c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = 0, 0, 0, 0
		return
	}
	mix := func(s, d uint8) uint8 {
		v := (float64(s)*sa + float64(d)*da*(1-sa)) / outA
		return uint8(math.Round(math.Max(0, math.Min(255, v))))
	}
	c.Pix[i] = mix(col.R, c.Pix[i])
	c.Pix[i+1] = mix(col.G, c.Pix[i+1])
	c.Pix[i+2] = mix(col.B, c.Pix[i+2])
	c.Pix[i+3] = uint8(math.Round(outA * 255))
}

// FillRect rellena un rectángulo.
func (c *Canvas) FillRect(x, y, w, h int, col RGBA) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			c.Over(xx, yy, col)
		}
	}
}

// FillRound rellena un rectángulo con esquinas redondeadas y, si border != nil,
// dibuja un borde interior de 1 px.
func (c *Canvas) FillRound(x, y, w, h, radius int, fill RGBA, border *RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	maxR := (minInt(w, h) - 1) / 2
	if radius > maxR {
		radius = maxR
	}
	if radius < 0 {
		radius = 0
	}
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			dx, dy := 0, 0
			// Distancia al centro del círculo de la esquina más cercana.
			switch {
			case xx < radius && yy < radius:
				dx, dy = radius-xx-1, radius-yy-1
			case xx >= w-radius && yy < radius:
				dx, dy = xx-(w-radius), radius-yy-1
			case xx < radius && yy >= h-radius:
				dx, dy = radius-xx-1, yy-(h-radius)
			case xx >= w-radius && yy >= h-radius:
				dx, dy = xx-(w-radius), yy-(h-radius)
			}
			if dx != 0 || dy != 0 {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
			}
			col := fill
			if border != nil && radius > 0 {
				// Anillo exterior: a menos de 1 px del borde del redondeo o de
				// los lados rectos.
				edge := xx == 0 || yy == 0 || xx == w-1 || yy == h-1
				if (dx*dx+dy*dy) > (radius-1)*(radius-1) && (dx != 0 || dy != 0) {
					edge = true
				}
				if edge {
					col = *border
				}
			}
			c.Over(x+xx, y+yy, col)
		}
	}
}

// Text dibuja un texto con la fuente indicada y devuelve el ancho ocupado.
func (c *Canvas) Text(f *font.Face, x, y int, s string, scale, spacing int, col RGBA) int {
	f.Each(s, scale, spacing, func(px, py int) {
		c.Over(x+px, y+py, col)
	})
	return f.Measure(s, scale, spacing)
}

// TextWidth mide un texto sin dibujarlo.
func TextWidth(f *font.Face, s string, scale, spacing int) int {
	return f.Measure(s, scale, spacing)
}

// TextHeight devuelve el alto de una línea con la fuente y escala dadas.
func TextHeight(f *font.Face, scale int) int { return f.Rows(scale) }

// Gradient dibuja un degradado horizontal dentro de un rectángulo redondeado.
func (c *Canvas) Gradient(x, y, w, h, radius int, a, b RGBA) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			t := 0.0
			if w > 1 {
				t = float64(xx-x) / float64(w-1)
			}
			c.Over(xx, yy, Blend(a, b, t))
		}
	}
	_ = radius
}

// Clear deja el lienzo totalmente transparente.
func (c *Canvas) Clear() {
	for i := range c.Pix {
		c.Pix[i] = 0
	}
}

// ScaleCanvas devuelve una copia escalada por un factor entero (para HiDPI).
func (c *Canvas) ScaleCanvas(factor int) *Canvas {
	if factor <= 1 {
		return c
	}
	out := NewCanvas(c.W*factor, c.H*factor)
	srcRowBytes := c.W * 4
	dstRowBytes := out.W * 4
	for y := 0; y < c.H; y++ {
		srcRow := c.Pix[y*srcRowBytes : (y+1)*srcRowBytes]
		firstRowStart := y * factor * dstRowBytes
		firstRow := out.Pix[firstRowStart : firstRowStart+dstRowBytes]
		for x := 0; x < c.W; x++ {
			srcPixel := srcRow[x*4 : x*4+4]
			for dx := 0; dx < factor; dx++ {
				dst := (x*factor + dx) * 4
				copy(firstRow[dst:dst+4], srcPixel)
			}
		}
		for dy := 1; dy < factor; dy++ {
			start := firstRowStart + dy*dstRowBytes
			copy(out.Pix[start:start+dstRowBytes], firstRow)
		}
	}
	return out
}

// Image convierte el lienzo en una imagen de Go (para guardar un PNG).
func (c *Canvas) Image() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	// image.RGBA usa el mismo orden RGBA y stride compacto que Canvas.Pix.
	copy(img.Pix, c.Pix)
	return img
}

// OverlayComposite pega el lienzo sobre un fondo opaco del color indicado, útil
// para las vistas previas (un PNG con alfa no se ve bien en la documentación).
func (c *Canvas) OverlayComposite(bg RGBA) *Canvas {
	out := NewCanvas(c.W, c.H)
	for i := 0; i < len(c.Pix); i += 4 {
		sa := float64(c.Pix[i+3]) / 255
		// El color del lienzo está sin premultiplicar: se mezcla con su alfa.
		out.Pix[i] = uint8(math.Round(float64(c.Pix[i])*sa + float64(bg.R)*(1-sa)))
		out.Pix[i+1] = uint8(math.Round(float64(c.Pix[i+1])*sa + float64(bg.G)*(1-sa)))
		out.Pix[i+2] = uint8(math.Round(float64(c.Pix[i+2])*sa + float64(bg.B)*(1-sa)))
		out.Pix[i+3] = 255
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
