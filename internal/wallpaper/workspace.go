package wallpaper

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ─── Fondos por workspace ───────────────────────────────────────────────────

// WorkspaceEntry es una regla «workspace → fondo».
type WorkspaceEntry struct {
	// Key es la clave tal como la escribió el usuario (nombre, número o
	// sufijo tras ':').
	Key string
	// Source es el fondo en forma textual: scene:petals, image:~/x.png…
	Source string
}

// WorkspaceMap resuelve qué fondo corresponde a cada workspace.
//
// Se compara, por este orden: el nombre completo (sin distinguir mayúsculas),
// el sufijo tras ':' (así «web» vale para «3:web») y el número. Si nada
// coincide se usa el fondo base.
type WorkspaceMap struct {
	fallback string
	entries  map[string]string
}

// NewWorkspaceMap crea el mapa con el fondo de reserva y las reglas dadas.
func NewWorkspaceMap(fallback string, rules map[string]string) *WorkspaceMap {
	m := &WorkspaceMap{fallback: strings.TrimSpace(fallback), entries: map[string]string{}}
	for k, v := range rules {
		if err := m.Set(k, v); err != nil {
			continue
		}
	}
	return m
}

// normalizeWorkspaceKey prepara una clave para comparar.
func normalizeWorkspaceKey(k string) string {
	return strings.ToLower(strings.TrimSpace(k))
}

// Set añade o reemplaza una regla.
func (m *WorkspaceMap) Set(key, source string) error {
	k := normalizeWorkspaceKey(key)
	src := strings.TrimSpace(source)
	if k == "" {
		return fmt.Errorf("el workspace no puede estar vacío")
	}
	if src == "" {
		return fmt.Errorf("el fondo no puede estar vacío")
	}
	if m.entries == nil {
		m.entries = map[string]string{}
	}
	m.entries[k] = src
	return nil
}

// Unset elimina una regla; devuelve true si existía.
func (m *WorkspaceMap) Unset(key string) bool {
	k := normalizeWorkspaceKey(key)
	if _, ok := m.entries[k]; !ok {
		return false
	}
	delete(m.entries, k)
	return true
}

// Clear borra todas las reglas.
func (m *WorkspaceMap) Clear() { m.entries = map[string]string{} }

// Len devuelve cuántas reglas hay.
func (m *WorkspaceMap) Len() int { return len(m.entries) }

// Fallback es el fondo que se usa cuando ningún workspace coincide.
func (m *WorkspaceMap) Fallback() string { return m.fallback }

// Raw devuelve una copia de las reglas, para guardarla en la configuración.
func (m *WorkspaceMap) Raw() map[string]string {
	out := make(map[string]string, len(m.entries))
	for k, v := range m.entries {
		out[k] = v
	}
	return out
}

// Entries devuelve las reglas ordenadas por clave.
func (m *WorkspaceMap) Entries() []WorkspaceEntry {
	out := make([]WorkspaceEntry, 0, len(m.entries))
	for k, v := range m.entries {
		out = append(out, WorkspaceEntry{Key: k, Source: v})
	}
	sort.Slice(out, func(i, j int) bool {
		// Los números primero y en orden numérico; luego el resto alfabético.
		ni, ei := strconv.Atoi(out[i].Key)
		nj, ej := strconv.Atoi(out[j].Key)
		if ei == nil && ej == nil {
			return ni < nj
		}
		if ei == nil {
			return true
		}
		if ej == nil {
			return false
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Lookup busca la regla que corresponde a un workspace. Devuelve el fondo, la
// clave que coincidió y si hubo coincidencia.
func (m *WorkspaceMap) Lookup(num int, name string) (string, string, bool) {
	if m == nil {
		return "", "", false
	}
	name = strings.TrimSpace(name)
	candidates := make([]string, 0, 3)
	if name != "" {
		candidates = append(candidates, name)
		// «3:web» también responde a «web».
		if i := strings.LastIndex(name, ":"); i >= 0 && i+1 < len(name) {
			candidates = append(candidates, strings.TrimSpace(name[i+1:]))
		}
	}
	if num > 0 {
		candidates = append(candidates, strconv.Itoa(num))
	}
	for _, cand := range candidates {
		k := normalizeWorkspaceKey(cand)
		if k == "" {
			continue
		}
		if src, ok := m.entries[k]; ok {
			return src, k, true
		}
	}
	return "", "", false
}

// SourceFor devuelve el fondo que toca dibujar en un workspace, con el fondo
// base como reserva. La segunda devolución es la clave que coincidió ("" si se
// usó la reserva).
func (m *WorkspaceMap) SourceFor(num int, name string) (string, string) {
	if src, key, ok := m.Lookup(num, name); ok {
		return src, key
	}
	if m == nil {
		return "", ""
	}
	return m.fallback, ""
}

// ─── Transiciones ───────────────────────────────────────────────────────────

// Tipos de transición entre fondos.
const (
	TransFade  = "fade"
	TransSlide = "slide"
	TransWipe  = "wipe"
	TransZoom  = "zoom"
	TransNone  = "none"
)

// Transition describe cómo se pasa de un fondo a otro.
type Transition struct {
	Kind     string
	Duration time.Duration
}

// NewTransition valida el tipo y construye la transición. Un tipo desconocido
// o una duración no positiva se traducen en «sin transición».
func NewTransition(kind string, ms int) Transition {
	k := normalizeTransition(kind)
	if k == "" || ms <= 0 {
		return Transition{Kind: TransNone}
	}
	return Transition{Kind: k, Duration: time.Duration(ms) * time.Millisecond}
}

// normalizeTransition acepta los nombres en español e inglés y devuelve ""
// para lo desconocido.
func normalizeTransition(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", TransNone, "ninguna", "corte", "cut":
		return TransNone
	case TransFade, "fundido", "crossfade":
		return TransFade
	case TransSlide, "deslizar", "deslizamiento":
		return TransSlide
	case TransWipe, "barrido", "cortina":
		return TransWipe
	case TransZoom, "acercar", "zoom-in":
		return TransZoom
	}
	return ""
}

// ValidTransition indica si el nombre es un tipo conocido.
func ValidTransition(kind string) bool {
	return normalizeTransition(kind) != ""
}

// TransitionNames enumera los tipos disponibles.
func TransitionNames() []string {
	return []string{TransFade, TransSlide, TransWipe, TransZoom, TransNone}
}

// Active indica si hay animación que dibujar.
func (t Transition) Active() bool {
	return t.Kind != "" && t.Kind != TransNone && t.Duration > 0
}

// String describe la transición para la interfaz.
func (t Transition) String() string {
	if !t.Active() {
		return "sin transición"
	}
	return fmt.Sprintf("%s · %d ms", t.Kind, t.Duration.Milliseconds())
}

// Progress convierte el tiempo transcurrido en un progreso 0..1.
func (t Transition) Progress(elapsed time.Duration) float64 {
	if !t.Active() || elapsed <= 0 {
		return 0
	}
	p := float64(elapsed) / float64(t.Duration)
	if p > 1 {
		p = 1
	}
	return p
}

// EaseInOut suaviza el progreso con una curva cúbica simétrica.
func EaseInOut(p float64) float64 {
	if p <= 0 {
		return 0
	}
	if p >= 1 {
		return 1
	}
	if p < 0.5 {
		return 4 * p * p * p
	}
	f := -2*p + 2
	return 1 - f*f*f/2
}

// Blend compone dos lienzos según el tipo de transición y un progreso 0..1.
// Si los tamaños no coinciden, el primero se reescala al del segundo.
func Blend(from, to *Canvas, p float64, kind string) *Canvas {
	if to == nil {
		return from
	}
	if from == nil {
		return to
	}
	if p <= 0 {
		return from
	}
	if p >= 1 {
		return to
	}
	a := from
	if a.W != to.W || a.H != to.H {
		a = ScaleCanvas(a, to.W, to.H)
	}
	w, h := to.W, to.H
	if w < 1 || h < 1 {
		return to
	}
	out := &Canvas{W: w, H: h, Pix: make([]uint8, w*h*3)}
	switch normalizeTransition(kind) {
	case TransSlide:
		blendSlide(out, a, to, p)
	case TransWipe:
		blendWipe(out, a, to, p)
	case TransZoom:
		blendZoom(out, a, to, p)
	default:
		blendFade(out, a, to, p)
	}
	return out
}

func setPix(c *Canvas, i int, r, g, b float64) {
	c.Pix[i] = clamp8(r)
	c.Pix[i+1] = clamp8(g)
	c.Pix[i+2] = clamp8(b)
}

func blendFade(out, a, b *Canvas, p float64) {
	q := 1 - p
	for i := 0; i < len(out.Pix); i += 3 {
		setPix(out, i,
			float64(a.Pix[i])*q+float64(b.Pix[i])*p,
			float64(a.Pix[i+1])*q+float64(b.Pix[i+1])*p,
			float64(a.Pix[i+2])*q+float64(b.Pix[i+2])*p)
	}
}

// blendSlide desplaza el fondo viejo a la izquierda mientras el nuevo entra
// por la derecha.
func blendSlide(out, a, b *Canvas, p float64) {
	w, h := out.W, out.H
	shift := int(p * float64(w))
	if shift > w {
		shift = w
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			if x+shift < w {
				j := (y*w + x + shift) * 3
				setPix(out, i, float64(a.Pix[j]), float64(a.Pix[j+1]), float64(a.Pix[j+2]))
				continue
			}
			j := (y*w + x + shift - w) * 3
			setPix(out, i, float64(b.Pix[j]), float64(b.Pix[j+1]), float64(b.Pix[j+2]))
		}
	}
}

// blendWipe descubre el fondo nuevo de izquierda a derecha con un borde suave.
func blendWipe(out, a, b *Canvas, p float64) {
	w, h := out.W, out.H
	edge := p * float64(w)
	soft := float64(w) / 14
	if soft < 2 {
		soft = 2
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			mix := (edge + soft - float64(x)) / soft
			if mix <= 0 {
				setPix(out, i, float64(a.Pix[i]), float64(a.Pix[i+1]), float64(a.Pix[i+2]))
				continue
			}
			if mix > 1 {
				setPix(out, i, float64(b.Pix[i]), float64(b.Pix[i+1]), float64(b.Pix[i+2]))
				continue
			}
			q := 1 - mix
			setPix(out, i,
				float64(a.Pix[i])*q+float64(b.Pix[i])*mix,
				float64(a.Pix[i+1])*q+float64(b.Pix[i+1])*mix,
				float64(a.Pix[i+2])*q+float64(b.Pix[i+2])*mix)
		}
	}
}

// blendZoom acerca el fondo nuevo desde el centro y lo funde con el viejo.
func blendZoom(out, a, b *Canvas, p float64) {
	w, h := out.W, out.H
	scale := 1 + 0.10*(1-p)
	cx, cy := float64(w-1)/2, float64(h-1)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			sx := int(cx + (float64(x)-cx)/scale)
			sy := int(cy + (float64(y)-cy)/scale)
			sx = clampInt(sx, 0, w-1)
			sy = clampInt(sy, 0, h-1)
			j := (sy*w + sx) * 3
			q := 1 - p
			setPix(out, i,
				float64(a.Pix[i])*q+float64(b.Pix[j])*p,
				float64(a.Pix[i+1])*q+float64(b.Pix[j+1])*p,
				float64(a.Pix[i+2])*q+float64(b.Pix[j+2])*p)
		}
	}
}

// ScaleCanvas reescala un lienzo con muestreo por caja (sin interpolación
// aparatosa: solo importa durante los milisegundos de una transición).
func ScaleCanvas(c *Canvas, w, h int) *Canvas {
	if c == nil {
		return nil
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if c.W == w && c.H == h {
		return c
	}
	out := &Canvas{W: w, H: h, Pix: make([]uint8, w*h*3), bgR: c.bgR, bgG: c.bgG, bgB: c.bgB}
	if c.W < 1 || c.H < 1 {
		return out
	}
	for y := 0; y < h; y++ {
		sy := y * c.H / h
		for x := 0; x < w; x++ {
			sx := x * c.W / w
			i := (y*w + x) * 3
			j := (sy*c.W + sx) * 3
			out.Pix[i], out.Pix[i+1], out.Pix[i+2] = c.Pix[j], c.Pix[j+1], c.Pix[j+2]
		}
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Source devuelve la descripción textual del fondo, la misma que acepta
// ParseSpec.
func (s Spec) Source() string {
	switch s.Kind {
	case KindScene:
		return "scene:" + s.Scene
	case KindImage, KindGIF, KindVideo:
		if s.Path == "" {
			return "none"
		}
		return string(s.Kind) + ":" + s.Path
	case KindNone:
		return "none"
	}
	if s.Scene != "" {
		return "scene:" + s.Scene
	}
	if s.Path != "" {
		return "image:" + s.Path
	}
	return "none"
}
