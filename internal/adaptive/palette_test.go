package adaptive

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// testImage crea una imagen con tres zonas de color bien diferenciadas.
func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 60, 40))
	for x := 0; x < 60; x++ {
		for y := 0; y < 40; y++ {
			var c color.RGBA
			switch {
			case x < 20:
				c = color.RGBA{230, 60, 120, 255}
			case x < 40:
				c = color.RGBA{40, 90, 220, 255}
			default:
				c = color.RGBA{20, 25, 40, 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func TestFromImageProducesUsablePalette(t *testing.T) {
	p := FromImage(testImage(), "Prueba", false)
	if p.Name != "Prueba" {
		t.Fatalf("nombre = %q", p.Name)
	}
	if p.Category != "adaptativo" || !p.Adaptive {
		t.Fatal("la paleta no se marca como adaptativa")
	}
	colors := []ui.Color{p.Bg, p.BgAlt, p.Fg, p.FgDim, p.Accent, p.Accent2, p.Ok, p.Warn, p.Err, p.Border}
	for _, c := range colors {
		s := string(c)
		if len(s) != 7 || !strings.HasPrefix(s, "#") {
			t.Fatalf("color inválido %q", s)
		}
	}
	if p.Accent == p.Accent2 {
		t.Fatal("los dos acentos no deberían coincidir")
	}
	g := p.GradientOr()
	if g[0] == (ui.Color("")) || g[1] == (ui.Color("")) {
		t.Fatal("gradiente vacío")
	}
}

func TestFromFileAndSave(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "fondo.png")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, testImage()); err != nil {
		t.Fatal(err)
	}
	f.Close()
	p, err := FromFile(src, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Name, "fondo") {
		t.Fatalf("nombre derivado = %q", p.Name)
	}
	out := filepath.Join(dir, "themes", "adaptativo.toml")
	if err := Save(p, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"[palette]", "bg =", "accent =", "category = \"adaptativo\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q en la paleta guardada:\n%s", want, text)
		}
	}
}

func TestFromFileMissing(t *testing.T) {
	if _, err := FromFile(filepath.Join(t.TempDir(), "no-existe.png"), ""); err == nil {
		t.Fatal("se esperaba error con un archivo inexistente")
	}
}
