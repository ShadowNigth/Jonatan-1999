package osd

import (
	"bytes"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func testRenderer(scale int) *Renderer {
	return New(Options{
		Width:    260,
		Padding:  16,
		Radius:   18,
		Opacity:  0.95,
		Face:     font.Builtin(),
		Theme:    ui.PaletteByName("sakura"),
		Scale:    scale,
		Shadow:   true,
		Segments: 20,
	})
}

func TestRendererDefaultsAreCompact(t *testing.T) {
	r := New(Options{Face: font.Builtin(), Theme: ui.PaletteByName("sakura")})
	if r.Opt.Width != 260 || r.Opt.Padding != 12 || r.Scale() != 1 {
		t.Fatalf("valores compactos del OSD: %+v, escala %d", r.Opt, r.Scale())
	}
}

func TestStatePercentAndFill(t *testing.T) {
	casos := []struct {
		valor  int
		pct    int
		fill   float64
		overdr bool
	}{
		{-10, 0, 0, false},
		{0, 0, 0, false},
		{45, 45, 0.45, false},
		{100, 100, 1, false},
		{130, 130, 1, true},
		{400, MaxValue, 1, true},
	}
	for _, c := range casos {
		s := Volume(c.valor, false, "")
		if got := s.Percent(); got != c.pct {
			t.Fatalf("Percent(%d) = %d, se esperaba %d", c.valor, got, c.pct)
		}
		if got := s.Filled(); got < c.fill-0.001 || got > c.fill+0.001 {
			t.Fatalf("Filled(%d) = %v, se esperaba %v", c.valor, got, c.fill)
		}
		if got := s.Overdrive(); got != c.overdr {
			t.Fatalf("Overdrive(%d) = %v", c.valor, got)
		}
	}

	// Silenciado: la barra queda vacía aunque el volumen tenga valor.
	if got := Volume(70, true, "").Filled(); got != 0 {
		t.Fatalf("silenciado con relleno %v", got)
	}
}

func TestStateTexts(t *testing.T) {
	s := Volume(45, false, "Auriculares")
	if s.Label() != "volumen" || s.ValueText() != "45%" || s.Detail() != "Auriculares" {
		t.Fatalf("textos del aviso: %q %q %q", s.Label(), s.ValueText(), s.Detail())
	}
	m := Volume(45, true, "  ")
	if m.ValueText() != "Silencio" {
		t.Fatalf("estado silenciado = %q", m.ValueText())
	}
	if m.Detail() != "" {
		t.Fatalf("un dispositivo en blanco no debe mostrarse: %q", m.Detail())
	}
	if Volume(45, false, "A").Key() == Volume(46, false, "A").Key() {
		t.Fatal("Key no distingue valores distintos")
	}
}

func TestRenderSizeAndAlpha(t *testing.T) {
	for _, esc := range []int{1, 2, 3} {
		r := testRenderer(esc)
		s := Volume(50, false, "Altavoces")
		c := r.Render(s, 1)
		w, h := r.Size(s)
		if c.W != w || c.H != h {
			t.Fatalf("escala %d: lienzo %dx%d, tamaño %dx%d", esc, c.W, c.H, w, h)
		}
		if h < 40 || w != 260 {
			t.Fatalf("escala %d: tamaño poco razonable %dx%d", esc, w, h)
		}
		// El texto crece con la escala.
		r1, r3 := testRenderer(1), testRenderer(3)
		ejemplo := Volume(50, false, "Altavoces")
		_, h1 := r1.Size(ejemplo)
		_, h3 := r3.Size(ejemplo)
		if h3 <= h1 {
			t.Fatalf("la escala 3 no es más alta que la 1: %d vs %d", h3, h1)
		}
	}
}

func TestMutedLayoutStacksTitleAndValueWhenNeeded(t *testing.T) {
	r := New(Options{
		Width:   150,
		Padding: 12,
		Face:    font.Builtin(),
		Theme:   ui.PaletteByName("sakura"),
	})
	active := Volume(50, false, "")
	muted := Volume(50, true, "")

	if !r.textFitsInline(active) {
		t.Fatal("un porcentaje corto debería compartir fila con el título")
	}
	if r.textFitsInline(muted) {
		t.Fatal("«Silencio» debe pasar a otra fila si no cabe junto al título")
	}
	_, activeHeight := r.Size(active)
	_, mutedHeight := r.Size(muted)
	wantHeight := activeHeight + r.lineH(r.sc) + 2*r.sc
	if mutedHeight != wantHeight {
		t.Fatalf("alto del OSD silenciado = %d; se esperaba %d", mutedHeight, wantHeight)
	}
	if canvas := r.Render(muted, 1); canvas.H != mutedHeight {
		t.Fatalf("el lienzo silenciado mide %d px de alto, se esperaban %d", canvas.H, mutedHeight)
	}
}

func TestMutedValueScaleKeepsStateTextWhole(t *testing.T) {
	r := New(Options{
		Width:   180,
		Padding: 12,
		Scale:   2,
		Face:    font.Builtin(),
		Theme:   ui.PaletteByName("sakura"),
	})
	muted := Volume(50, true, "")
	scale := r.valueScaleFor(muted)
	if scale >= r.valueScale() {
		t.Fatalf("el estado largo no redujo su escala: %d", scale)
	}
	inner := r.Opt.Width - 2*r.Opt.Padding
	if width := bar.TextWidth(r.face, muted.ValueText(), scale, scale); width > inner {
		t.Fatalf("«%s» mide %d px y se recortaría dentro de %d px", muted.ValueText(), width, inner)
	}
	canvas := r.Render(muted, 1)
	_, wantHeight := r.Size(muted)
	if canvas.H != wantHeight {
		t.Fatalf("alto del lienzo %d; se esperaba %d", canvas.H, wantHeight)
	}
}

func TestRenderOmitsTechnicalAudioDevice(t *testing.T) {
	r := testRenderer(1)
	withoutDevice := Volume(45, false, "")
	withTechnicalDevice := Volume(45, false, "alsa_output.pci-0000_00_1f.3.analog-stereo")

	w1, h1 := r.Size(withTechnicalDevice)
	w2, h2 := r.Size(withoutDevice)
	if w1 != w2 || h1 != h2 {
		t.Fatalf("el identificador técnico no debe cambiar el tamaño del OSD: %dx%d != %dx%d", w1, h1, w2, h2)
	}
	if a, b := r.Render(withTechnicalDevice, 1), r.Render(withoutDevice, 1); !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("el OSD no debe dibujar el identificador técnico de la salida de audio")
	}
}

func TestRenderFadeIsTransparentAtZero(t *testing.T) {
	r := testRenderer(2)
	c := r.Render(Volume(30, false, ""), 0)
	for i := 3; i < len(c.Pix); i += 4 {
		if c.Pix[i] != 0 {
			t.Fatalf("píxel %d con alfa %d en un fundido a cero", i/4, c.Pix[i])
		}
	}
	// Y opaco en el centro con alfa 1 (las esquinas son redondeadas).
	c = r.Render(Volume(30, false, ""), 1)
	centro := ((c.H/2)*c.W + c.W/2) * 4
	if c.Pix[centro+3] == 0 {
		t.Fatal("el fondo debería ser visible con alfa 1")
	}
	// Con un fundido intermedio, menos opaco que a plena intensidad.
	medio := r.Render(Volume(30, false, ""), 0.5)
	if medio.Pix[centro+3] >= c.Pix[centro+3] {
		t.Fatalf("el fundido intermedio no es más transparente: %d >= %d",
			medio.Pix[centro+3], c.Pix[centro+3])
	}
}

func TestRenderBarGrowsWithVolume(t *testing.T) {
	r := testRenderer(1)
	// Cuenta los píxeles del color de acento en la franja de la barra: cuanta
	// más cobertura, más píxeles pintados.
	cuenta := func(v int) int {
		c := r.Render(Volume(v, false, ""), 1)
		accent := bar.ParseColor(string(r.Opt.Theme.Accent), 1)
		n := 0
		for y := c.H - r.Opt.Padding - r.barH(); y < c.H-r.Opt.Padding; y++ {
			for x := 0; x < c.W; x++ {
				i := (y*c.W + x) * 4
				if c.Pix[i] == accent.R && c.Pix[i+1] == accent.G && c.Pix[i+2] == accent.B {
					n++
				}
			}
		}
		return n
	}
	if cuenta(10) >= cuenta(60) {
		t.Fatal("la barra no crece con el volumen")
	}
	// Silenciado: no hay relleno de acento en la barra.
	if cuenta(0) != 0 {
		t.Fatal("con volumen 0 no debería haber relleno")
	}
}
