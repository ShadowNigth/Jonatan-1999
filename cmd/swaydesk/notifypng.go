package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// writeNotifyPNG dibuja una lista de notificaciones y la guarda como PNG sobre
// un fondo del tema, para poder revisar el diseño sin sesión gráfica.
func writeNotifyPNG(cfg *config.Config, items []notify.Notification, path string) error {
	return writeNotifyPNGWith(cfg, items, path, true)
}

// writeNotifyPNGWith permite generar el PNG con fondo transparente, para
// componerlo sobre una captura del escritorio.
func writeNotifyPNGWith(cfg *config.Config, items []notify.Notification, path string, backdrop bool) error {
	now := time.Now()
	r := notify.New(notify.RenderOptions{
		Width:    cfg.NotifyWidth,
		Padding:  cfg.NotifyPadding,
		Gap:      cfg.NotifyGap,
		Radius:   cfg.NotifyRadius,
		Opacity:  cfg.NotifyOpacity,
		Face:     font.Best(cfg.NotifyFont),
		Theme:    ui.PaletteByName(cfg.Theme),
		Scale:    2,
		MaxLines: cfg.NotifyMaxLines,
	})
	canvas, _ := r.RenderGroups(notify.Views(items, nil), now)

	const mx, my = 24, 20
	w, h := canvas.W+2*mx, canvas.H+2*my
	pal := ui.PaletteByName(cfg.Theme)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	top, bottom := parseHexRGBA(string(pal.BgAlt)), parseHexRGBA(string(pal.Bg))
	for y := 0; backdrop && y < h; y++ {
		t := 0.0
		if h > 1 {
			t = float64(y) / float64(h-1)
		}
		row := color.RGBA{
			R: uint8(float64(top.R) + (float64(bottom.R)-float64(top.R))*t),
			G: uint8(float64(top.G) + (float64(bottom.G)-float64(top.G))*t),
			B: uint8(float64(top.B) + (float64(bottom.B)-float64(top.B))*t),
			A: 255,
		}
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, row)
		}
	}
	for y := 0; y < canvas.H; y++ {
		for x := 0; x < canvas.W; x++ {
			i := (y*canvas.W + x) * 4
			a := float64(canvas.Pix[i+3]) / 255
			if a == 0 {
				continue
			}
			dst := img.RGBAAt(x+mx, y+my)
			img.SetRGBA(x+mx, y+my, color.RGBA{
				R: uint8(float64(canvas.Pix[i])*a + float64(dst.R)*(1-a)),
				G: uint8(float64(canvas.Pix[i+1])*a + float64(dst.G)*(1-a)),
				B: uint8(float64(canvas.Pix[i+2])*a + float64(dst.B)*(1-a)),
				A: 255,
			})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// parseHexRGBA convierte "#rrggbb" en color.RGBA opaco.
func parseHexRGBA(s string) color.RGBA {
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	dig := func(i int) uint8 {
		if i >= len(s) {
			return 0
		}
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			return c - '0'
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10
		case c >= 'A' && c <= 'F':
			return c - 'A' + 10
		}
		return 0
	}
	if len(s) < 6 {
		return color.RGBA{A: 255}
	}
	return color.RGBA{R: dig(0)<<4 | dig(1), G: dig(2)<<4 | dig(3), B: dig(4)<<4 | dig(5), A: 255}
}
