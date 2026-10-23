package wallpaper

import (
	"testing"
	"time"
)

func TestWorkspaceMapLookup(t *testing.T) {
	m := NewWorkspaceMap("scene:aurora", map[string]string{
		"1":                "scene:petals",
		"web":              "scene:city",
		"3:web":            "scene:grid",
		"Chat de la Tarde": "image:~/fondo.png",
	})

	casos := []struct {
		num  int
		name string
		want string
		key  string
	}{
		{1, "1", "scene:petals", "1"},
		{2, "2:web", "scene:city", "web"},   // sufijo tras ':'
		{3, "3:web", "scene:grid", "3:web"}, // el nombre completo gana
		{4, "4:chat de la tarde", "image:~/fondo.png", "chat de la tarde"},
		{5, "5:otros", "scene:aurora", ""}, // sin coincidencia: base
		{2, "", "scene:aurora", ""},        // sin nombre ni número útil
		{0, "sudo", "scene:aurora", ""},
	}
	for _, c := range casos {
		src, key := m.SourceFor(c.num, c.name)
		if src != c.want || key != c.key {
			t.Fatalf("%d/%q: %q (%q), se esperaba %q (%q)", c.num, c.name, src, key, c.want, c.key)
		}
	}

	// Las mayúsculas y los espacios sobrantes no importan.
	if src, _, ok := m.Lookup(0, "  WEB "); !ok || src != "scene:city" {
		t.Fatalf("normalización de nombre: %q %v", src, ok)
	}

	// Un mapa nil usa la reserva vacía sin entrar en pánico.
	var vacio *WorkspaceMap
	if src, key := vacio.SourceFor(1, "1"); src != "" || key != "" {
		t.Fatalf("mapa nil: %q %q", src, key)
	}
}

func TestWorkspaceMapEdits(t *testing.T) {
	m := NewWorkspaceMap("scene:aurora", nil)
	if err := m.Set("  ", "scene:x"); err == nil {
		t.Fatal("una clave vacía debe fallar")
	}
	if err := m.Set("1", "   "); err == nil {
		t.Fatal("un fondo vacío debe fallar")
	}
	if err := m.Set("1", "scene:petals"); err != nil {
		t.Fatal(err)
	}
	if err := m.Set("2", "scene:grid"); err != nil {
		t.Fatal(err)
	}
	if err := m.Set("10", "scene:city"); err != nil {
		t.Fatal(err)
	}
	if m.Len() != 3 {
		t.Fatalf("Len = %d", m.Len())
	}
	// Se ordenan primero los números y en orden numérico.
	entries := m.Entries()
	want := []string{"1", "2", "10"}
	for i, k := range want {
		if entries[i].Key != k {
			t.Fatalf("orden %v", entries)
		}
	}
	// Raw es una copia: tocarla no altera el mapa.
	raw := m.Raw()
	raw["1"] = "scene:aurora"
	if src, _, _ := m.Lookup(1, "1"); src != "scene:petals" {
		t.Fatalf("Raw debería ser una copia, hay %q", src)
	}
	if m.Unset("2") != true || m.Unset("2") != false {
		t.Fatal("Unset debe informar si existía")
	}
	m.Clear()
	if m.Len() != 0 {
		t.Fatalf("tras Clear quedan %d", m.Len())
	}
	if m.Fallback() != "scene:aurora" {
		t.Fatalf("reserva = %q", m.Fallback())
	}
}

func TestTransitionParsing(t *testing.T) {
	casos := map[string]string{
		"fade":     TransFade,
		"FUNDIDO":  TransFade,
		"slide":    TransSlide,
		"deslizar": TransSlide,
		"wipe":     TransWipe,
		"zoom":     TransZoom,
		"none":     TransNone,
		"ninguna":  TransNone,
	}
	for in, want := range casos {
		if !ValidTransition(in) {
			t.Fatalf("%q debería ser válido", in)
		}
		if got := NewTransition(in, 300).Kind; got != want {
			t.Fatalf("%q → %q, se esperaba %q", in, got, want)
		}
	}
	if ValidTransition("arcoiris") {
		t.Fatal("un tipo inventado no es válido")
	}
	if tr := NewTransition("fade", 0); tr.Active() {
		t.Fatal("0 ms no debe animar")
	}
	if tr := NewTransition("fade", -5); tr.Active() {
		t.Fatal("una duración negativa no debe animar")
	}
	if tr := NewTransition("arcoiris", 300); tr.Active() || tr.Kind != TransNone {
		t.Fatalf("tipo inválido → %+v", tr)
	}
	tr := NewTransition("slide", 400)
	if !tr.Active() || tr.Duration != 400*time.Millisecond {
		t.Fatalf("transición = %+v", tr)
	}
	if tr.Progress(200*time.Millisecond) != 0.5 {
		t.Fatalf("progreso = %v", tr.Progress(200*time.Millisecond))
	}
	if tr.Progress(time.Second) != 1 || tr.Progress(-time.Second) != 0 {
		t.Fatal("el progreso debe quedar acotado a 0..1")
	}
	if EaseInOut(0) != 0 || EaseInOut(1) != 1 {
		t.Fatal("la curva debe empezar en 0 y acabar en 1")
	}
	if EaseInOut(0.5) < 0.49 || EaseInOut(0.5) > 0.51 {
		t.Fatalf("la curva debe ser simétrica: %v", EaseInOut(0.5))
	}
	if NewTransition("fade", 450).String() != "fade · 450 ms" {
		t.Fatalf("descripción = %q", NewTransition("fade", 450).String())
	}
}

// canvasSolid crea un lienzo de un color.
func canvasSolid(w, h int, r, g, b uint8) *Canvas {
	c := &Canvas{W: w, H: h, Pix: make([]uint8, w*h*3)}
	for i := 0; i < len(c.Pix); i += 3 {
		c.Pix[i], c.Pix[i+1], c.Pix[i+2] = r, g, b
	}
	return c
}

func TestBlendTransitions(t *testing.T) {
	rojo := canvasSolid(40, 20, 255, 0, 0)
	azul := canvasSolid(40, 20, 0, 0, 255)

	// Extremos: 0 y 1 devuelven el original tal cual.
	if Blend(rojo, azul, 0, TransFade) != rojo {
		t.Fatal("p=0 debe devolver el fondo anterior")
	}
	if Blend(rojo, azul, 1, TransFade) != azul {
		t.Fatal("p=1 debe devolver el fondo nuevo")
	}
	// Nada que mezclar.
	if Blend(nil, azul, 0.5, TransFade) != azul {
		t.Fatal("sin fondo anterior debe verse el nuevo")
	}
	if Blend(rojo, nil, 0.5, TransFade) != rojo {
		t.Fatal("sin fondo nuevo debe verse el anterior")
	}

	// Fundido: en el punto medio ambos colores están presentes.
	mid := Blend(rojo, azul, 0.5, TransFade)
	if mid.W != 40 || mid.H != 20 {
		t.Fatalf("tamaño = %dx%d", mid.W, mid.H)
	}
	r, _, b := mid.Pix[0], mid.Pix[1], mid.Pix[2]
	if r < 120 || r > 135 || b < 120 || b > 135 {
		t.Fatalf("fundido al 50%% = %d,%d", r, b)
	}

	// Todos los tipos respetan el tamaño y no dejan píxeles a cero.
	for _, kind := range TransitionNames() {
		out := Blend(rojo, azul, 0.5, kind)
		if out.W != 40 || out.H != 20 {
			t.Fatalf("%s: tamaño %dx%d", kind, out.W, out.H)
		}
		if len(out.Pix) != 40*20*3 {
			t.Fatalf("%s: búfer de %d", kind, len(out.Pix))
		}
		for i := 3; i < len(out.Pix); i += 3 {
			if out.Pix[i] == 0 && out.Pix[i+1] == 0 && out.Pix[i+2] == 0 {
				t.Fatalf("%s: píxel negro inesperado en %d", kind, i)
			}
		}
	}

	// Deslizamiento: a mitad, la mitad izquierda es el fondo viejo y la derecha
	// el nuevo (el anterior se desplaza hacia la izquierda).
	sl := Blend(rojo, azul, 0.5, TransSlide)
	if r, _, b := sl.Pix[0], sl.Pix[1], sl.Pix[2]; r != 255 || b != 0 {
		t.Fatalf("slide izquierda = %d,%d", r, b)
	}
	last := (20-1)*40*3 + (40-1)*3
	if r, _, b := sl.Pix[last], sl.Pix[last+1], sl.Pix[last+2]; b != 255 || r != 0 {
		t.Fatalf("slide derecha = %d,%d", r, b)
	}

	// Barrido: el píxel 0 ya es el fondo nuevo y el último sigue siendo el viejo.
	wp := Blend(rojo, azul, 0.5, TransWipe)
	if r, _, b := wp.Pix[0], wp.Pix[1], wp.Pix[2]; b != 255 {
		t.Fatalf("wipe inicio = %d,%d", r, b)
	}
	if r, _, b := wp.Pix[last], wp.Pix[last+1], wp.Pix[last+2]; r != 255 {
		t.Fatalf("wipe final = %d,%d", r, b)
	}

	// Tamaños distintos: se reescala el fondo anterior en vez de fallar.
	pequeno := canvasSolid(20, 10, 255, 0, 0)
	out := Blend(pequeno, azul, 0.5, TransFade)
	if out.W != 40 || out.H != 20 {
		t.Fatalf("no se reescaló: %dx%d", out.W, out.H)
	}
}

func TestScaleCanvas(t *testing.T) {
	c := canvasSolid(4, 2, 10, 20, 30)
	if ScaleCanvas(c, 4, 2) != c {
		t.Fatal("mismo tamaño debe devolver el mismo lienzo")
	}
	up := ScaleCanvas(c, 8, 4)
	if up.W != 8 || up.H != 4 || len(up.Pix) != 8*4*3 {
		t.Fatalf("escalado = %+v", up)
	}
	if up.Pix[0] != 10 || up.Pix[1] != 20 || up.Pix[2] != 30 {
		t.Fatalf("colores perdidos: %v", up.Pix[:3])
	}
	down := ScaleCanvas(c, 1, 1)
	if down.W != 1 || down.H != 1 || down.Pix[0] != 10 {
		t.Fatalf("reducción = %+v", down)
	}
	if ScaleCanvas(nil, 4, 4) != nil {
		t.Fatal("nil debe seguir siendo nil")
	}
	if zero := ScaleCanvas(c, 0, 0); zero.W != 1 || zero.H != 1 {
		t.Fatalf("tamaño mínimo = %dx%d", zero.W, zero.H)
	}
}

func TestSpecSourceRoundTrip(t *testing.T) {
	casos := []string{"scene:petals", "image:~/fondo.png", "gif:/tmp/a.gif", "video:/tmp/v.mp4", "none"}
	for _, src := range casos {
		spec, err := ParseSpec(src, testPal)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		got := spec.Source()
		if src == "none" {
			if got != "none" {
				t.Fatalf("none → %q", got)
			}
			continue
		}
		// Algunos formatos normalizan la ruta (~ no se expande aquí).
		spec2, err := ParseSpec(got, testPal)
		if err != nil {
			t.Fatalf("%q → %q no se puede volver a leer: %v", src, got, err)
		}
		if spec2.Kind != spec.Kind {
			t.Fatalf("%q → %q cambió el tipo: %s ≠ %s", src, got, spec2.Kind, spec.Kind)
		}
	}
	// Un fondo vacío no inventa una escena silenciosa: devuelve none.
	if got := (Spec{}).Source(); got != "none" {
		t.Fatalf("spec vacía → %q", got)
	}
}

// La escena de una spec se conserva al pasar por Source.
func TestSpecSourceScene(t *testing.T) {
	s := Spec{Kind: KindScene, Scene: "grid", Pal: testPal}
	if s.Source() != "scene:grid" {
		t.Fatalf("Source = %q", s.Source())
	}
}
