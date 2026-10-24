package osd

import (
	"image"
	"image/color"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func testMediaRenderer() *MediaRenderer {
	return NewMedia(Options{
		Width:   320,
		Padding: 12,
		Radius:  16,
		Opacity: 0.95,
		Face:    font.Builtin(),
		Theme:   ui.PaletteByName("sakura"),
		Scale:   2,
		Shadow:  true,
	})
}

func TestMediaRendererSizeAndFade(t *testing.T) {
	r := testMediaRenderer()
	state := MediaState{Title: "Track", Artist: "Artist"}
	w, h := r.Size(state)
	canvas := r.Render(state, 1)
	if canvas.W != w || canvas.H != h || w != 320 || h <= 64 {
		t.Fatalf("tamaño OSD media = %dx%d, se esperaba 320xalgo mayor que 64", canvas.W, canvas.H)
	}
	transparent := r.Render(state, 0)
	for i := 3; i < len(transparent.Pix); i += 4 {
		if transparent.Pix[i] != 0 {
			t.Fatalf("píxel %d con alfa %d en un fundido a cero", i/4, transparent.Pix[i])
		}
	}
}

func TestMediaRendererDrawsArtwork(t *testing.T) {
	r := testMediaRenderer()
	cover := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < cover.Bounds().Dy(); y++ {
		for x := 0; x < cover.Bounds().Dx(); x++ {
			cover.SetRGBA(x, y, color.RGBA{R: 230, G: 30, B: 70, A: 255})
		}
	}
	canvas := r.Render(MediaState{Title: "Song", Artist: "Artist", Artwork: cover}, 1)
	x := r.Opt.Padding + r.coverSize()/2
	y := canvas.H / 2
	i := (y*canvas.W + x) * 4
	if canvas.Pix[i] < 220 || canvas.Pix[i+1] > 45 || canvas.Pix[i+2] < 60 || canvas.Pix[i+3] == 0 {
		t.Fatalf("la carátula no se dibujó en el centro: %v", canvas.Pix[i:i+4])
	}
}
