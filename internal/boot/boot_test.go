package boot

import (
	"strings"
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func testTheme() *ui.Theme { return ui.NewTheme("sakura", false) }

func TestStyles(t *testing.T) {
	if len(Styles()) < 4 {
		t.Fatalf("se esperaban al menos 4 estilos, hay %d", len(Styles()))
	}
	for _, s := range Styles() {
		if !ValidStyle(s) {
			t.Fatalf("%q debería ser válido", s)
		}
		if !ValidStyle(strings.ToUpper(s)) {
			t.Fatalf("%q en mayúsculas debería valer", s)
		}
	}
	if ValidStyle("inexistente") {
		t.Fatal("un estilo inventado no debería ser válido")
	}
}

func TestLogoIsRectangular(t *testing.T) {
	// El perfil de color se fija para que Render emita secuencias ANSI.
	prev := ui.GetProfile()
	ui.SetProfile(ui.ProfileTrueColor)
	defer ui.SetProfile(prev)

	rows := logoRows()
	w := ui.Width(rows[0])
	for i, r := range rows {
		if got := ui.Width(r); got != w {
			t.Fatalf("fila %d mide %d, se esperaba %d", i, got, w)
		}
	}
	painted := paintLogo(testTheme(), 0.4)
	for i, r := range painted {
		if ui.Width(r) != w {
			t.Fatalf("fila pintada %d mide %d, se esperaba %d", i, ui.Width(r), w)
		}
		if !strings.Contains(r, "38;2;") {
			t.Fatalf("fila %d sin color", i)
		}
	}
}

func TestParticlesPerStyle(t *testing.T) {
	for _, style := range Styles() {
		parts := makeParticles(style, 60, 100, 30, 5)
		if style == "minimal" {
			if len(parts) != 0 {
				t.Fatalf("minimal no debería tener partículas")
			}
			continue
		}
		if len(parts) != 60 {
			t.Fatalf("%s: %d partículas", style, len(parts))
		}
		for _, p := range parts {
			if p.x < 0 || p.x > 100 || p.y < 0 || p.y > 30 {
				t.Fatalf("%s: partícula fuera de rango: %+v", style, p)
			}
			if p.ch == 0 {
				t.Fatalf("%s: partícula sin carácter", style)
			}
		}
	}
}

func TestRenderFrame(t *testing.T) {
	opts := Options{
		Theme:    testTheme(),
		Style:    "sakura",
		Duration: 2 * time.Second,
		Version:  "1.0.0",
		Intro:    true,
		Steps: []Step{
			{Label: "Compositor", Detail: "sway", OK: true},
			{Label: "Barra", Detail: "waybar", OK: true},
			{Label: "Fondo", Detail: "petals", OK: true},
		},
	}
	for _, prog := range []float64{0, 0.3, 0.6, 1.0} {
		frame := render(opts.Theme, opts, time.Duration(prog*float64(opts.Duration)), 100, 30)
		lines := strings.Split(frame, "\n")
		if len(lines) != 30 {
			t.Fatalf("progreso %.1f: %d líneas, se esperaban 30", prog, len(lines))
		}
		plain := ui.Sanitize(frame)
		if !strings.Contains(plain, "swaydesk") {
			t.Fatalf("progreso %.1f: falta el logotipo", prog)
		}
		if !strings.Contains(plain, "Compositor") || !strings.Contains(plain, "waybar") {
			t.Fatalf("progreso %.1f: faltan los pasos", prog)
		}
	}
}

func TestRenderWithoutIntro(t *testing.T) {
	opts := Options{Theme: testTheme(), Style: "minimal", Duration: time.Second, Steps: []Step{{Label: "Tema", Detail: "x"}}}
	frame := render(opts.Theme, opts, time.Second, 80, 24)
	if strings.Contains(ui.Sanitize(frame), "█") {
		t.Fatal("sin intro no debería aparecer el logotipo grande")
	}
	if n := len(strings.Split(frame, "\n")); n != 24 {
		t.Fatalf("%d líneas, se esperaban 24", n)
	}
}

func TestProgressBar(t *testing.T) {
	th := testTheme()
	fade := func(c ui.Color) ui.Color { return c }
	for _, prog := range []float64{0, 0.5, 1} {
		bar := progressBar(th, 40, prog, fade)
		plain := ui.Sanitize(bar)
		if !strings.Contains(plain, "0 %") && !strings.Contains(plain, "100 %") && !strings.Contains(plain, "50 %") {
			t.Fatalf("progreso %.1f: porcentaje ausente en %q", prog, plain)
		}
	}
}

func TestDefaultSteps(t *testing.T) {
	cfg := config.Default()
	steps := DefaultSteps(cfg)
	if len(steps) < 5 {
		t.Fatalf("se esperaban al menos 5 pasos, hay %d", len(steps))
	}
	for _, s := range steps {
		if s.Label == "" || s.Detail == "" {
			t.Fatalf("paso incompleto: %+v", s)
		}
	}
}
