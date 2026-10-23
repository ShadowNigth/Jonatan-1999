package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// demoCover dibuja una carátula de ejemplo (degradado del tema y círculos) y
// la guarda en la caché. Se usa en `swaydesk notifications test` para enseñar
// cómo queda una notificación con imagen.
func demoCover(cache, theme string) string {
	const n = 120
	pal := ui.PaletteByName(theme)
	arriba := parseHexRGBA(string(pal.Accent2))
	abajo := parseHexRGBA(string(pal.Accent))
	centro := parseHexRGBA(string(pal.Bg))
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	cx, cy := float64(n)/2, float64(n)/2
	for y := 0; y < n; y++ {
		t := float64(y) / float64(n-1)
		base := color.RGBA{
			R: uint8(float64(arriba.R) + (float64(abajo.R)-float64(arriba.R))*t),
			G: uint8(float64(arriba.G) + (float64(abajo.G)-float64(arriba.G))*t),
			B: uint8(float64(arriba.B) + (float64(abajo.B)-float64(arriba.B))*t),
			A: 255,
		}
		for x := 0; x < n; x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			c := base
			// Aro exterior y disco central, como una portada minimalista.
			if d > 40 && d < 44 {
				c = color.RGBA{R: centro.R, G: centro.G, B: centro.B, A: 210}
			}
			if d < 17 {
				c = color.RGBA{R: centro.R, G: centro.G, B: centro.B, A: 235}
			}
			img.SetRGBA(x, y, c)
		}
	}
	path := filepath.Join(cache, "demo-caratula.png")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return ""
	}
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return ""
	}
	return path
}
