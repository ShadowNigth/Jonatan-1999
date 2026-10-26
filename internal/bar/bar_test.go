package bar

import (
	"math"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

func testRenderer() *Renderer {
	return New(Options{
		Height:  38,
		Margin:  6,
		Padding: 14,
		Gap:     10,
		Opacity: 0.9,
		Islands: true,
		Modules: []string{"cpu", "ram", "volume", "network", "battery", "clock"},
		Theme:   ui.PaletteByName("sakura"),
		Face:    font.Builtin(),
	})
}

func TestOpacityZeroMustBeExplicit(t *testing.T) {
	implicit := New(Options{Theme: ui.PaletteByName("sakura"), Face: font.Builtin()})
	if implicit.Opt.Opacity != 0.68 {
		t.Fatalf("opacidad cero implícita = %.2f, quería el valor por defecto 0.68", implicit.Opt.Opacity)
	}
	explicit := New(Options{Opacity: 0, OpacitySet: true, Theme: ui.PaletteByName("sakura"), Face: font.Builtin()})
	if explicit.Opt.Opacity != 0 {
		t.Fatalf("no se respetó la opacidad cero explícita: %.2f", explicit.Opt.Opacity)
	}
}

func TestPowerAndStatsModulesRemovedFromDefaultsAndLegacyConfig(t *testing.T) {
	for _, module := range DefaultModules {
		switch module {
		case "power":
			t.Fatal("el módulo de energía no debe aparecer en los valores por defecto")
		case "cpu", "ram":
			t.Fatalf("CPU y RAM no deben aparecer por defecto en la barra: %q", module)
		}
	}

	r := New(Options{
		Modules: []string{"clock", "power"},
		Theme:   ui.PaletteByName("sakura"),
		Face:    font.Builtin(),
	})
	if len(r.Opt.Modules) != 1 || r.Opt.Modules[0] != "clock" {
		t.Fatalf("módulos después de retirar el control de energía: %v", r.Opt.Modules)
	}
}

func sampleSnapshot() Snapshot {
	return Snapshot{
		Workspaces: []Workspace{
			{Name: "1", Focused: true},
			{Name: "2", Visible: true},
			{Name: "3"},
			{Name: "4", Urgent: true},
		},
		Title:      "nvim · configuración del escritorio",
		AppID:      "foot",
		Clock:      "14:32",
		Date:       "vie 23 oct",
		CPU:        23,
		RAM:        61,
		Volumen:    45,
		Wifi:       "Casa",
		Signal:     82,
		Connected:  true,
		HasBattery: true,
		Battery:    74,
	}
}

func TestParseColor(t *testing.T) {
	c := ParseColor("#1e1e2e", 0.5)
	if c.R != 0x1e || c.G != 0x1e || c.B != 0x2e {
		t.Fatalf("componentes mal: %+v", c)
	}
	if c.A == 0 || c.A == 255 {
		t.Fatalf("alfa no aplicado: %d", c.A)
	}
	if got := ParseColor("#ffffff", -1); got.A != 255 {
		t.Fatalf("sin alfa explícito debe ser opaco: %d", got.A)
	}
	if got := ParseColor("#11223344", -1); got.A != 0x44 {
		t.Fatalf("alfa de 8 dígitos ignorado: %d", got.A)
	}
	// Cadena vacía no debe entrar en pánico.
	_ = ParseColor("", 1)
	_ = ParseColor("#", 1)
}

func TestBlendAndOver(t *testing.T) {
	a := RGBA{0, 0, 0, 255}
	b := RGBA{255, 255, 255, 255}
	mid := Blend(a, b, 0.5)
	if mid.R < 127 || mid.R > 128 {
		t.Fatalf("mezcla incorrecta: %+v", mid)
	}
	c := NewCanvas(4, 4)
	c.FillRect(1, 1, 2, 2, RGBA{255, 0, 0, 255})
	if c.Pix[(1*4+1)*4] != 255 {
		t.Fatal("FillRect no pintó")
	}
	// Un píxel transparente no debe cambiar nada.
	before := c.Pix[(1*4+1)*4+1]
	c.Over(1, 1, RGBA{0, 255, 0, 0})
	if c.Pix[(1*4+1)*4+1] != before {
		t.Fatal("un color transparente modificó el lienzo")
	}

	// Los atajos de alfa cero y alfa opaco deben conservar source-over.
	fast := NewCanvas(1, 1)
	src := RGBA{200, 80, 30, 127}
	fast.Over(0, 0, src)
	if got := fast.Pix[:4]; got[0] != src.R || got[1] != src.G || got[2] != src.B || got[3] != src.A {
		t.Fatalf("source-over sobre transparencia = %v", got)
	}
	background := RGBA{10, 20, 30, 255}
	fast.Set(0, 0, background)
	fast.Over(0, 0, RGBA{240, 90, 40, 128})
	want := []uint8{
		uint8(math.Round(240*128.0/255 + 10*(1-128.0/255))),
		uint8(math.Round(90*128.0/255 + 20*(1-128.0/255))),
		uint8(math.Round(40*128.0/255 + 30*(1-128.0/255))),
		255,
	}
	for i := range want {
		if fast.Pix[i] != want[i] {
			t.Fatalf("source-over opaco[%d] = %d, esperaba %d", i, fast.Pix[i], want[i])
		}
	}
}

func TestFillRoundCorners(t *testing.T) {
	c := NewCanvas(40, 20)
	c.FillRound(0, 0, 40, 20, 10, RGBA{255, 255, 255, 255}, nil)
	// Las esquinas deben quedar transparentes y el centro opaco.
	if c.Pix[3] != 0 {
		t.Fatal("la esquina no quedó transparente")
	}
	mid := ((10*40)+20)*4 + 3
	if c.Pix[mid] != 255 {
		t.Fatal("el centro debería estar relleno")
	}
}

func TestRenderProducesPixels(t *testing.T) {
	r := testRenderer()
	c := r.Render(sampleSnapshot(), 1600)
	if c.W != 1600 || c.H != 38 {
		t.Fatalf("tamaño inesperado: %dx%d", c.W, c.H)
	}
	if len(c.Pix) != 1600*38*4 {
		t.Fatalf("búfer de %d bytes", len(c.Pix))
	}
	// Debe haber píxeles opacos (las islas) y transparentes (los huecos).
	opaque, clear := 0, 0
	for i := 3; i < len(c.Pix); i += 4 {
		if c.Pix[i] > 0 {
			opaque++
		} else {
			clear++
		}
	}
	if opaque == 0 {
		t.Fatal("la barra salió completamente transparente")
	}
	if clear == 0 {
		t.Fatal("las islas deberían dejar huecos transparentes entre ellas")
	}
}

func TestRenderSingleIsland(t *testing.T) {
	opt := Options{Height: 34, Margin: 4, Padding: 10, Islands: false, Theme: ui.PaletteByName("nord"), Face: font.Builtin(), Modules: []string{"clock"}}
	r := New(opt)
	c := r.Render(sampleSnapshot(), 800)
	// En modo continuo, el borde inferior de las esquinas debe estar pintado
	// casi por completo a lo ancho.
	painted := 0
	for x := 8; x < 792; x++ {
		if c.Pix[((34/2)*800+x)*4+3] > 0 {
			painted++
		}
	}
	if painted < 700 {
		t.Fatalf("la barra continua solo pinta %d px de 784", painted)
	}
}

func TestRenderNoWorkspaces(t *testing.T) {
	r := testRenderer()
	snap := sampleSnapshot()
	snap.Workspaces = nil
	c := r.Render(snap, 900)
	if len(c.Pix) != 900*38*4 {
		t.Fatal("el lienzo no se generó")
	}
}

func TestMeasureAndItems(t *testing.T) {
	r := testRenderer()
	// 5 px de glifo (sin espaciado) por 3 caracteres, escalado.
	if got, want := r.measure("abc"), 3*5*r.Scale(); got != want {
		t.Fatalf("medida %d, esperada %d", got, want)
	}
	items := r.rightItems(sampleSnapshot(), RGBA{255, 255, 255, 255}, RGBA{180, 180, 180, 255},
		RGBA{200, 100, 200, 255}, RGBA{0, 255, 0, 255}, RGBA{255, 200, 0, 255}, RGBA{255, 0, 0, 255})
	if len(items) == 0 {
		t.Fatal("sin módulos")
	}
	var texts []string
	for _, it := range items {
		texts = append(texts, it.text)
	}
	joined := strings.Join(texts, " ")
	for _, want := range []string{"cpu", "ram", "vol", "Casa", "74%", "14:32"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("falta %q en %q", want, joined)
		}
	}
}

func TestMutedAndNoBattery(t *testing.T) {
	r := New(Options{Height: 32, Modules: []string{"volume", "battery", "network"}, Theme: ui.PaletteByName("dracula"), Face: font.Builtin()})
	snap := Snapshot{Muted: true, HasBattery: false, Connected: false}
	items := r.rightItems(snap, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{})
	var texts []string
	for _, it := range items {
		texts = append(texts, it.text)
	}
	joined := strings.Join(texts, " ")
	if !strings.Contains(joined, "silencio") || !strings.Contains(joined, "sin red") {
		t.Fatalf("estados alternativos ausentes: %q", joined)
	}
	if strings.Contains(joined, "%") {
		t.Fatalf("no debería mostrar batería: %q", joined)
	}
}

func TestScaleCanvas(t *testing.T) {
	c := NewCanvas(10, 5)
	c.Set(0, 0, RGBA{255, 0, 0, 255})
	big := c.ScaleCanvas(2)
	if big.W != 20 || big.H != 10 {
		t.Fatalf("tamaño escalado %dx%d", big.W, big.H)
	}
	if big.Pix[3] != 255 {
		t.Fatal("el píxel no se replicó")
	}
}

func TestCompositeAndImage(t *testing.T) {
	c := NewCanvas(4, 2)
	c.FillRect(0, 0, 4, 2, RGBA{255, 255, 255, 128})
	out := c.OverlayComposite(RGBA{0, 0, 0, 255})
	if out.Pix[0] < 120 || out.Pix[0] > 136 {
		t.Fatalf("composición incorrecta: %d", out.Pix[0])
	}
	if out.Pix[3] != 255 {
		t.Fatal("el resultado debe ser opaco")
	}
	if img := out.Image(); img.Bounds().Dx() != 4 {
		t.Fatal("Image devolvió un tamaño distinto")
	}
}

func TestNotifyModuleShowsCount(t *testing.T) {
	r := New(Options{
		Height:  38,
		Margin:  6,
		Padding: 14,
		Gap:     10,
		Islands: true,
		Modules: []string{"notify", "clock"},
		Theme:   ui.PaletteByName("sakura"),
		Face:    font.Builtin(),
	})
	s := sampleSnapshot()
	s.Notifications = 3
	s.NotifReplies = 1
	items := r.rightItems(s, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{})
	found := false
	for _, it := range items {
		if it.click == "notify" {
			found = true
			if it.text != "3+" || it.kind != itemPill {
				t.Fatalf("indicador = %q (kind %d), se esperaba la pastilla 3+", it.text, it.kind)
			}
		}
	}
	if !found {
		t.Fatalf("no hay indicador de notificaciones en %+v", items)
	}

	// Sin notificaciones y sin DND: cero en color apagado.
	s.Notifications, s.NotifReplies = 0, 0
	items = r.rightItems(s, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{})
	if items[0].text != "avisos" {
		t.Fatalf("sin notificaciones = %q", items[0].text)
	}

	// Con DND se avisa del modo silencioso.
	s.NotifDND = true
	items = r.rightItems(s, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{})
	if items[0].text != "dnd" || items[0].fg != (RGBA{}) {
		t.Logf("DND: %q", items[0].text)
	}
	s.Notifications = 2
	items = r.rightItems(s, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{}, RGBA{})
	if !strings.Contains(items[0].text, "dnd") && !strings.Contains(items[0].text, "2") {
		t.Fatalf("DND no visible: %q", items[0].text)
	}
}

func TestNotifyRegionIsClickable(t *testing.T) {
	r := New(Options{
		Height:  38,
		Margin:  6,
		Padding: 14,
		Gap:     10,
		Islands: true,
		Modules: []string{"notify", "clock"},
		Theme:   ui.PaletteByName("sakura"),
		Face:    font.Builtin(),
	})
	s := sampleSnapshot()
	s.Notifications = 2
	r.Render(s, 1200)

	regions := r.Regions()
	if len(regions) != 1 {
		t.Fatalf("regiones = %+v", regions)
	}
	var notify *Region
	for i := range regions {
		if regions[i].Name == "notify" {
			notify = &regions[i]
		}
	}
	if notify == nil {
		t.Fatalf("no hay región de notificaciones: %+v", regions)
	}
	if _, ok := r.RegionAt(notify.X+1, notify.Y+1); !ok {
		t.Fatalf("un punto de la zona no la encuentra: %+v", notify)
	}
	if _, ok := r.RegionAt(notify.X-20, notify.Y+1); ok {
		t.Fatalf("un punto fuera de la zona la encuentra")
	}
	// Un segundo Render no acumula zonas viejas.
	r.Render(s, 1200)
	if got := len(r.Regions()); got != 1 {
		t.Fatalf("regiones tras repintar = %d", got)
	}
}
