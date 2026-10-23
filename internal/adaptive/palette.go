// Package adaptive genera paletas de color a partir de un fondo de pantalla.
//
// La idea es que el escritorio acompañe a la imagen: se cuantizan sus píxeles,
// se elige un color dominante para el fondo, el más saturado para el acento y
// se derivan el resto de roles manteniendo el contraste legible.
package adaptive

import (
	"fmt"
	"image"
	"image/gif"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// swatch es un color agrupado con su peso.
type swatch struct {
	r, g, b float64
	count   float64
}

// OpenImage abre cualquier imagen soportada por Go (PNG, JPEG, GIF).
func OpenImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return img, nil
}

// FirstFrame abre una imagen animada (GIF) y devuelve su primer fotograma, o la
// imagen completa si no es animada.
func FirstFrame(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if format != "gif" {
		if _, err := f.Seek(0, 0); err != nil {
			return nil, err
		}
		img, _, err := image.Decode(f)
		return img, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, err
	}
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("gif sin fotogramas: %s", path)
	}
	_ = cfg
	return g.Image[0], nil
}

// quantize agrupa los píxeles en cubos de color y devuelve los más frecuentes.
func quantize(img image.Image, buckets int) []swatch {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}
	// Muestreo adaptativo: como máximo ~20 000 píxeles.
	stepX := 1
	stepY := 1
	for (w/stepX)*(h/stepY) > 20000 {
		stepX++
		stepY++
	}
	type key struct{ r, g, b uint8 }
	acc := map[key]*swatch{}
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bl, _ := img.At(x, y).RGBA()
			k := key{uint8(r >> 12), uint8(g >> 12), uint8(bl >> 12)} // 4 bits por canal
			s, ok := acc[k]
			if !ok {
				s = &swatch{}
				acc[k] = s
			}
			// Se conserva la media real dentro del cubo.
			s.r += float64(r >> 8)
			s.g += float64(g >> 8)
			s.b += float64(bl >> 8)
			s.count++
		}
	}
	list := make([]swatch, 0, len(acc))
	for _, s := range acc {
		list = append(list, swatch{r: s.r / s.count, g: s.g / s.count, b: s.b / s.count, count: s.count})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].count > list[j].count })
	if len(list) > buckets {
		list = list[:buckets]
	}
	return list
}

// ─── Utilidades de color ────────────────────────────────────────────────────

type rgb struct{ r, g, b float64 }

func (c rgb) toHSL() (h, s, l float64) {
	r, g, b := c.r/255, c.g/255, c.b/255
	maxv := math.Max(r, math.Max(g, b))
	minv := math.Min(r, math.Min(g, b))
	l = (maxv + minv) / 2
	if maxv == minv {
		return 0, 0, l
	}
	d := maxv - minv
	if l > 0.5 {
		s = d / (2 - maxv - minv)
	} else {
		s = d / (maxv + minv)
	}
	switch maxv {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h /= 6
	return
}

func hsl(h, s, l float64) rgb {
	if s <= 0 {
		v := l * 255
		return rgb{v, v, v}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 1.0/2:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return rgb{hue(h+1.0/3) * 255, hue(h) * 255, hue(h-1.0/3) * 255}
}

func (c rgb) hex() string {
	cl := func(v float64) int {
		n := int(math.Round(v))
		if n < 0 {
			return 0
		}
		if n > 255 {
			return 255
		}
		return n
	}
	return fmt.Sprintf("#%02x%02x%02x", cl(c.r), cl(c.g), cl(c.b))
}

func (c rgb) luminance() float64 {
	f := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c.r) + 0.7152*f(c.g) + 0.0722*f(c.b)
}

func (c rgb) saturation() float64 {
	_, s, _ := c.toHSL()
	return s
}

func (c rgb) mix(o rgb, t float64) rgb {
	return rgb{c.r + (o.r-c.r)*t, c.g + (o.g-c.g)*t, c.b + (o.b-c.b)*t}
}

func (c rgb) darken(t float64) rgb  { return c.mix(rgb{0, 0, 0}, t) }
func (c rgb) lighten(t float64) rgb { return c.mix(rgb{255, 255, 255}, t) }

// ensureContrast ajusta el color hasta que el contraste frente a bg sea >= ratio.
func ensureContrast(c, bg rgb, ratio float64) rgb {
	contrast := func(a, b rgb) float64 {
		la, lb := a.luminance(), b.luminance()
		if la < lb {
			la, lb = lb, la
		}
		return (la + 0.05) / (lb + 0.05)
	}
	if contrast(c, bg) >= ratio {
		return c
	}
	darker := bg.luminance() < 0.35
	for i := 0; i < 20 && contrast(c, bg) < ratio; i++ {
		if darker {
			c = c.lighten(0.08)
		} else {
			c = c.darken(0.08)
		}
	}
	return c
}

func shiftHue(c rgb, delta float64) rgb {
	h, s, l := c.toHSL()
	h = math.Mod(h+delta+1, 1)
	return hsl(h, s, l)
}

// FromImage construye una paleta completa a partir de una imagen.
//
// name y key identifican la paleta resultante. light fuerza una variante clara;
// si es false se elige automáticamente según el brillo dominante de la imagen.
func FromImage(img image.Image, name string, light bool) ui.Palette {
	sw := quantize(img, 24)
	if len(sw) == 0 {
		return ui.PaletteByName("catppuccin")
	}

	// Fondo: los colores más oscuros (o más claros) con peso suficiente.
	var dark, lightC []swatch
	for _, s := range sw {
		c := rgb{s.r, s.g, s.b}
		if c.luminance() < 0.25 {
			dark = append(dark, s)
		}
		if c.luminance() > 0.55 {
			lightC = append(lightC, s)
		}
	}
	avgLight := 0.0
	for _, s := range sw {
		avgLight += rgb{s.r, s.g, s.b}.luminance() * s.count
	}
	total := 0.0
	for _, s := range sw {
		total += s.count
	}
	if total > 0 {
		avgLight /= total
	}
	if !light && avgLight > 0.62 {
		light = true
	}

	var bgSw swatch
	if light {
		if len(lightC) > 0 {
			bgSw = lightC[0]
		} else {
			bgSw = sw[0]
		}
	} else {
		if len(dark) > 0 {
			bgSw = dark[0]
		} else {
			bgSw = sw[len(sw)-1]
		}
	}
	bgBase := rgb{bgSw.r, bgSw.g, bgSw.b}
	var bg, bgAlt, bgSel rgb
	if light {
		bg = bgBase.mix(rgb{255, 255, 255}, 0.82)
		bgAlt = bgBase.mix(rgb{255, 255, 255}, 0.90)
		bgSel = bgBase.mix(rgb{255, 255, 255}, 0.62)
	} else {
		bg = bgBase.darken(0.55)
		bgAlt = bg.darken(0.25)
		bgSel = bg.lighten(0.14)
	}

	// Acento: el color más saturado con peso apreciable, evitando grises.
	var accents []swatch
	for _, s := range sw {
		c := rgb{s.r, s.g, s.b}
		if c.saturation() > 0.25 && c.luminance() > 0.08 {
			accents = append(accents, s)
		}
	}
	if len(accents) == 0 {
		accents = sw
	}
	sort.SliceStable(accents, func(i, j int) bool {
		ci := rgb{accents[i].r, accents[i].g, accents[i].b}
		cj := rgb{accents[j].r, accents[j].g, accents[j].b}
		return ci.saturation()*math.Sqrt(accents[i].count) > cj.saturation()*math.Sqrt(accents[j].count)
	})
	accent := rgb{accents[0].r, accents[0].g, accents[0].b}
	if accent.saturation() < 0.2 {
		accent = shiftHue(accent, 0.5)
	}
	accent = ensureContrast(accent, bg, 3.2)
	accent2 := shiftHue(accent, 0.12)
	if len(accents) > 1 {
		c2 := rgb{accents[1].r, accents[1].g, accents[1].b}
		if math.Abs(hueDistance(c2, accent)) > 0.08 {
			accent2 = ensureContrast(c2, bg, 3.0)
		}
	}
	accent2 = ensureContrast(accent2, bg, 3.0)

	var fg, fgDim, muted rgb
	if light {
		fg = bg.darken(0.85)
		fgDim = bg.darken(0.66)
		muted = bg.darken(0.45)
	} else {
		fg = bg.lighten(0.86)
		fgDim = bg.lighten(0.64)
		muted = bg.lighten(0.42)
	}

	okC := ensureContrast(hsl(0.36, 0.62, 0.62), bg, 3.0)
	if light {
		okC = ensureContrast(hsl(0.36, 0.65, 0.34), bg, 3.0)
	}
	warnC := ensureContrast(hsl(0.11, 0.80, 0.62), bg, 3.0)
	if light {
		warnC = ensureContrast(hsl(0.10, 0.85, 0.36), bg, 3.0)
	}
	errC := ensureContrast(hsl(0.98, 0.72, 0.62), bg, 3.0)
	if light {
		errC = ensureContrast(hsl(0.98, 0.70, 0.46), bg, 3.0)
	}
	border := bg.lighten(0.12)
	if light {
		border = bg.darken(0.12)
	}

	p := ui.Palette{
		Name:     name,
		Bg:       ui.Color(bg.hex()),
		BgAlt:    ui.Color(bgAlt.hex()),
		BgSel:    ui.Color(bgSel.hex()),
		Fg:       ui.Color(fg.hex()),
		FgDim:    ui.Color(fgDim.hex()),
		Muted:    ui.Color(muted.hex()),
		Accent:   ui.Color(accent.hex()),
		Accent2:  ui.Color(accent2.hex()),
		Ok:       ui.Color(okC.hex()),
		Warn:     ui.Color(warnC.hex()),
		Err:      ui.Color(errC.hex()),
		Border:   ui.Color(border.hex()),
		Gradient: [2]ui.Color{ui.Color(accent.hex()), ui.Color(accent2.hex())},
		Category: "adaptativo",
		Light:    light,
		Adaptive: true,
	}
	return p
}

func hueDistance(a, b rgb) float64 {
	ha, _, _ := a.toHSL()
	hb, _, _ := b.toHSL()
	d := math.Abs(ha - hb)
	if d > 0.5 {
		d = 1 - d
	}
	return d
}

// FromFile genera una paleta a partir de un archivo de imagen.
func FromFile(path, name string) (ui.Palette, error) {
	img, err := FirstFrame(path)
	if err != nil {
		return ui.Palette{}, err
	}
	if name == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name = "Adaptativo · " + base
	}
	return FromImage(img, name, false), nil
}

// Save escribe una paleta en formato TOML para que el usuario pueda editarla y
// reutilizarla como cualquier otra.
func Save(p ui.Palette, path string) error {
	var b strings.Builder
	b.WriteString("# Paleta generada por swaydesk a partir de un fondo de pantalla.\n")
	b.WriteString("# Puedes editarla a mano: se recarga al iniciar cualquier herramienta.\n\n")
	b.WriteString("[palette]\n")
	fmt.Fprintf(&b, "name = %q\n", p.Name)
	fmt.Fprintf(&b, "category = %q\n", p.Category)
	fmt.Fprintf(&b, "light = %v\n", p.Light)
	fmt.Fprintf(&b, "bg = %q\nbg_alt = %q\nbg_sel = %q\n", p.Bg, p.BgAlt, p.BgSel)
	fmt.Fprintf(&b, "fg = %q\nfg_dim = %q\nmuted = %q\n", p.Fg, p.FgDim, p.Muted)
	fmt.Fprintf(&b, "accent = %q\naccent2 = %q\n", p.Accent, p.Accent2)
	fmt.Fprintf(&b, "ok = %q\nwarn = %q\nerr = %q\nborder = %q\n", p.Ok, p.Warn, p.Err, p.Border)
	g := p.GradientOr()
	fmt.Fprintf(&b, "gradient = [%q, %q]\n", g[0], g[1])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
