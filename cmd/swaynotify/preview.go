package main

import (
	"fmt"
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

// demoNotes son las notificaciones de ejemplo de la vista previa.
func demoNotes(now time.Time) []notify.Notification {
	return []notify.Notification{
		{
			ID: 1, App: "swaydesk", Summary: "Tema aplicado",
			Body:    "Sakura · 31 paletas disponibles, cambia con <b>Super+t</b>",
			Urgency: notify.Normal, Created: now, Expires: now.Add(6 * time.Second),
			Timeout: 6000, Progress: -1,
		},
		{
			ID: 2, App: "pacman", Summary: "12 paquetes por actualizar",
			Body:    "Entre ellos el kernel y Mesa. Reinicia la sesión al terminar.",
			Urgency: notify.Critical, Created: now.Add(-2 * time.Second), Progress: 42,
			Actions: []notify.Action{{Key: "default", Label: "Actualizar"}, {Key: "later", Label: "Luego"}},
		},
		{
			ID: 3, App: "spotify", Summary: "Lofi Girl · Midnight Study",
			Body:    "Reproduciendo la lista «estudio»",
			Urgency: notify.Low, Created: now.Add(-4 * time.Second), Timeout: -1, Progress: -1,
		},
	}
}

// renderPreview compone las tarjetas sobre un fondo del tema y guarda el PNG.
func renderPreview(cfg *config.Config, path string) error {
	return renderPreviewWith(cfg, path, true)
}

// renderPreviewWith compone las tarjetas; con backdrop=false el PNG sale con
// fondo transparente para superponerlo a una captura.
func renderPreviewWith(cfg *config.Config, path string, backdrop bool) error {
	now := time.Now()
	items := demoNotes(now)
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
		Shadow:   true,
	})
	canvas, _ := r.Render(items, now)

	// Fondo simulado: degradado del tema con margen alrededor.
	const marginX, marginY = 24, 20
	w, h := canvas.W+2*marginX, canvas.H+2*marginY
	pal := ui.PaletteByName(cfg.Theme)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	top := color.RGBAModel.Convert(hexColor(string(pal.BgAlt))).(color.RGBA)
	bottom := color.RGBAModel.Convert(hexColor(string(pal.Bg))).(color.RGBA)
	for y := 0; backdrop && y < h; y++ {
		t := float64(y) / float64(h-1)
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
	// Composición alfa de las tarjetas.
	for y := 0; y < canvas.H; y++ {
		for x := 0; x < canvas.W; x++ {
			i := (y*canvas.W + x) * 4
			a := float64(canvas.Pix[i+3]) / 255
			if a == 0 {
				continue
			}
			if !backdrop {
				img.SetRGBA(x+marginX, y+marginY, color.RGBA{
					R: canvas.Pix[i], G: canvas.Pix[i+1], B: canvas.Pix[i+2], A: canvas.Pix[i+3],
				})
				continue
			}
			dst := img.RGBAAt(x+marginX, y+marginY)
			img.SetRGBA(x+marginX, y+marginY, color.RGBA{
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
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("guardando el PNG: %w", err)
	}
	return nil
}

// hexColor convierte "#rrggbb" en color.RGBA.
func hexColor(s string) color.RGBA {
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	var v [6]uint8
	for i := 0; i < 6 && i < len(s); i++ {
		d := s[i]
		switch {
		case d >= '0' && d <= '9':
			v[i] = d - '0'
		case d >= 'a' && d <= 'f':
			v[i] = d - 'a' + 10
		case d >= 'A' && d <= 'F':
			v[i] = d - 'A' + 10
		}
	}
	return color.RGBA{
		R: v[0]<<4 | v[1],
		G: v[2]<<4 | v[3],
		B: v[4]<<4 | v[5],
		A: 255,
	}
}
