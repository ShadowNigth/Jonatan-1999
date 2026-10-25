// Package wallpaper implementa fondos de pantalla animados para el terminal:
// escenas procedurales (pétalos, nieve, lluvia de neón, olas, auroras...) y
// reproducción de GIF animados, todo con la biblioteca estándar.
package wallpaper

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// SceneInfo describe una escena de fondo animada.
type SceneInfo struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Intensity   float64  `json:"intensity"` // 0 suave … 1 intensa
	Animation   string   `json:"animation"` // cómo se mueve
	Tags        []string `json:"tags"`
	// Cost recomienda el modo de pintado: "image" (recomendado) o "halfblock".
	Cost string `json:"cost"`
}

// AvailableScenes devuelve las escenas disponibles ordenadas por nombre.
func AvailableScenes() []SceneInfo {
	out := make([]SceneInfo, 0, len(scenes))
	for _, s := range scenes {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// SceneByKey busca una escena por su clave.
func SceneByKey(key string) (SceneInfo, bool) {
	s, ok := scenes[key]
	return s, ok
}

var scenes = map[string]SceneInfo{
	"petals": {
		Key: "petals", Name: "Pétalos de sakura", Description: "Pétalos rosas cayendo con viento suave",
		Intensity: 0.55, Animation: "caída lateral", Tags: []string{"anime", "sakura", "suave", "romántico"}, Cost: "image",
	},
	"snow": {
		Key: "snow", Name: "Nieve", Description: "Copos lentos sobre un cielo frío",
		Intensity: 0.35, Animation: "caída lenta", Tags: []string{"invierno", "yuki", "calma"}, Cost: "image",
	},
	"stars": {
		Key: "stars", Name: "Noche estrellada", Description: "Estrellas titilando y fugaces",
		Intensity: 0.45, Animation: "parpadeo", Tags: []string{"hoshizora", "cielo", "noche"}, Cost: "image",
	},
	"rain": {
		Key: "rain", Name: "Lluvia de neón", Description: "Gotas verticales con reflejos de ciudad",
		Intensity: 0.70, Animation: "caída rápida", Tags: []string{"cyberpunk", "ciudad", "noche"}, Cost: "image",
	},
	"aurora": {
		Key: "aurora", Name: "Aurora", Description: "Cortinas de luz ondulando",
		Intensity: 0.50, Animation: "ondas", Tags: []string{"magia", "cielo", "verde"}, Cost: "image",
	},
	"wave": {
		Key: "wave", Name: "Olas", Description: "Líneas de agua en movimiento",
		Intensity: 0.55, Animation: "ondas", Tags: []string{"mar", "miku", "azul"}, Cost: "image",
	},
	"city": {
		Key: "city", Name: "Ciudad de neón", Description: "Letreros que pulsan sobre edificios oscuros",
		Intensity: 0.65, Animation: "pulso", Tags: []string{"cyberpunk", "tokyo", "edificios"}, Cost: "image",
	},
	"embers": {
		Key: "embers", Name: "Brasas", Description: "Chispas y ascuas subiendo",
		Intensity: 0.60, Animation: "ascenso", Tags: []string{"fuego", "oni", "calor"}, Cost: "image",
	},
	"grid": {
		Key: "grid", Name: "Rejilla synthwave", Description: "Suelo en perspectiva con sol de neón",
		Intensity: 0.75, Animation: "avance", Tags: []string{"vaporwave", "retro", "ochentas"}, Cost: "image",
	},
	"meadow": {
		Key: "meadow", Name: "Pradera", Description: "Hierba meciéndose y polen flotando",
		Intensity: 0.40, Animation: "viento", Tags: []string{"ghibli", "naturaleza", "verde"}, Cost: "image",
	},
	"bubbles": {
		Key: "bubbles", Name: "Burbujas", Description: "Burbujas subiendo con borde de luz",
		Intensity: 0.50, Animation: "ascenso", Tags: []string{"agua", "suave", "kawaii"}, Cost: "image",
	},
	"nebula": {
		Key: "nebula", Name: "Nebulosa", Description: "Nubes de gas, filamentos y polvo luminoso con los colores del tema",
		Intensity: 0.38, Animation: "deriva, pulsos y motas flotantes", Tags: []string{"espacio", "cosmos", "galaxia", "nubes", "suave", "color"}, Cost: "image",
	},
}

// ─── Contexto del lienzo ────────────────────────────────────────────────────

// Canvas es una imagen RGB en memoria con la paleta del tema aplicada.
type Canvas struct {
	W, H int
	Pix  []uint8 // 3 bytes por píxel (r,g,b)
	// profundo cachea el color de fondo como componentes.
	bgR, bgG, bgB uint8
}

// NewCanvas crea un lienzo del tamaño indicado relleno con bg.
func NewCanvas(w, h int, bg ui.Color) *Canvas {
	c := &Canvas{W: w, H: h, Pix: make([]uint8, w*h*3)}
	r, g, b, _ := parseHex(string(bg), 5, 5, 10)
	c.bgR, c.bgG, c.bgB = uint8(r), uint8(g), uint8(b)
	for i := 0; i < len(c.Pix); i += 3 {
		c.Pix[i], c.Pix[i+1], c.Pix[i+2] = c.bgR, c.bgG, c.bgB
	}
	return c
}

func parseHex(s string, dr, dg, db int) (int, int, int, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return dr, dg, db, false
	}
	var r, g, b int
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return dr, dg, db, false
	}
	return r, g, b, true
}

func (c *Canvas) set(x, y int, r, g, b float64) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	i := (y*c.W + x) * 3
	c.Pix[i] = clamp8(r)
	c.Pix[i+1] = clamp8(g)
	c.Pix[i+2] = clamp8(b)
}

// blend mezcla un color sobre el píxel existente con la opacidad dada.
func (c *Canvas) blend(x, y int, r, g, b, a float64) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H || a <= 0 {
		return
	}
	if a > 1 {
		a = 1
	}
	i := (y*c.W + x) * 3
	c.Pix[i] = clamp8(float64(c.Pix[i])*(1-a) + r*a)
	c.Pix[i+1] = clamp8(float64(c.Pix[i+1])*(1-a) + g*a)
	c.Pix[i+2] = clamp8(float64(c.Pix[i+2])*(1-a) + b*a)
}

// glow añade luz con caída suave alrededor de un punto.
func (c *Canvas) glow(cx, cy, radius, r, g, b, strength float64) {
	if radius <= 0 {
		return
	}
	x0, x1 := int(cx-radius), int(cx+radius)
	y0, y1 := int(cy-radius), int(cy+radius)
	for y := y0; y <= y1; y++ {
		if y < 0 || y >= c.H {
			continue
		}
		for x := x0; x <= x1; x++ {
			if x < 0 || x >= c.W {
				continue
			}
			dx, dy := float64(x)-cx, float64(y)-cy
			d := math.Hypot(dx, dy)
			if d > radius {
				continue
			}
			f := 1 - d/radius
			c.blend(x, y, r, g, b, f*f*strength)
		}
	}
}

func (c *Canvas) disc(cx, cy, radius, r, g, b, a float64) {
	x0, x1 := int(cx-radius), int(cx+radius)
	y0, y1 := int(cy-radius), int(cy+radius)
	rr := radius * radius
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if dx*dx+dy*dy <= rr {
				c.blend(x, y, r, g, b, a)
			}
		}
	}
}

func (c *Canvas) cline(x0, y0, x1, y1, r, g, b, a float64) {
	dx := math.Abs(x1 - x0)
	dy := -math.Abs(y1 - y0)
	sx, sy := 1.0, 1.0
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	x, y := x0, y0
	err := dx + dy
	for {
		c.blend(int(x), int(y), r, g, b, a)
		if math.Abs(x-x1) < 0.5 && math.Abs(y-y1) < 0.5 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
	}
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// hexRGB convierte un color de la paleta en componentes float.
func hexRGB(c ui.Color) (float64, float64, float64) {
	r, g, b, _ := parseHex(string(c), 255, 255, 255)
	return float64(r), float64(g), float64(b)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// ─── Motor de escenas ───────────────────────────────────────────────────────

// Renderer dibuja una escena animada fotograma a fotograma.
type Renderer struct {
	scene   string
	pal     ui.Palette
	light   bool
	canvas  *Canvas
	part    []particle
	rng     *rand.Rand
	frame   int
	initW   int
	initH   int
	flashes []flash
}

type particle struct {
	x, y, vx, vy, size, phase, life, maxLife float64
	hue                                      int
}

type flash struct {
	x, y, t float64
}

// NewRenderer crea un renderizador para la escena y la paleta indicadas.
func NewRenderer(scene string, pal ui.Palette) *Renderer {
	if _, ok := scenes[scene]; !ok {
		scene = "stars"
	}
	return &Renderer{
		scene: scene, pal: pal, light: pal.Light,
		rng: rand.New(rand.NewSource(42)),
	}
}

// Resize ajusta el lienzo al tamaño del terminal en píxeles.
func (r *Renderer) Resize(w, h int) {
	if w < 2 {
		w = 2
	}
	if h < 2 {
		h = 2
	}
	if r.canvas != nil && r.initW == w && r.initH == h {
		return
	}
	r.canvas = NewCanvas(w, h, r.pal.Bg)
	r.initW, r.initH = w, h
	r.part = nil
	r.flashes = nil
	r.spawn()
}

// Frame dibuja el siguiente fotograma y devuelve el lienzo.
func (r *Renderer) Frame(w, h int) *Canvas {
	r.Resize(w, h)
	r.frame++
	c := r.canvas
	// Fondo base con un degradado vertical sutil del tema.
	r.paintBackdrop()

	switch r.scene {
	case "petals":
		r.drawPetals(c)
	case "snow":
		r.drawSnow(c)
	case "stars":
		r.drawStars(c)
	case "rain":
		r.drawRain(c)
	case "aurora":
		r.drawAurora(c)
	case "wave":
		r.drawWave(c)
	case "city":
		r.drawCity(c)
	case "embers":
		r.drawEmbers(c)
	case "grid":
		r.drawGrid(c)
	case "meadow":
		r.drawMeadow(c)
	case "bubbles":
		r.drawBubbles(c)
	case "nebula":
		r.drawNebula(c)
	}
	return c
}

// paintBackdrop pinta un degradado vertical entre BgAlt y Bg con un brillo de
// acento en la parte inferior, para que la escena nunca quede plana.
func (r *Renderer) paintBackdrop() {
	c := r.canvas
	bgR, bgG, bgB := hexRGB(r.pal.Bg)
	altR, altG, altB := hexRGB(r.pal.BgAlt)
	acR, acG, acB := hexRGB(r.pal.Accent)
	t := float64(r.frame) / 600.0
	for y := 0; y < c.H; y++ {
		f := float64(y) / float64(c.H)
		// curva suave
		g := f * f
		rr := lerp(altR, bgR, g)
		gg := lerp(altG, bgG, g)
		bb := lerp(altB, bgB, g)
		// halo de acento en el tercio inferior, respirando lentamente
		halo := math.Pow(f, 3) * (0.10 + 0.05*math.Sin(t*2*math.Pi))
		rr = lerp(rr, acR, halo)
		gg = lerp(gg, acG, halo)
		bb = lerp(bb, acB, halo)
		for x := 0; x < c.W; x++ {
			c.set(x, y, rr, gg, bb)
		}
	}
}

// ─── Partículas ─────────────────────────────────────────────────────────────

// spawn crea las partículas iniciales según la escena.
func (r *Renderer) spawn() {
	c := r.canvas
	n := 0
	switch r.scene {
	case "petals":
		n = c.W * c.H / 900
	case "snow":
		n = c.W * c.H / 700
	case "stars":
		n = c.W * c.H / 500
	case "rain":
		n = c.W * c.H / 400
	case "embers":
		n = c.W * c.H / 800
	case "bubbles":
		n = c.W * c.H / 2600
	case "meadow":
		n = c.W * c.H / 400
	case "city":
		n = c.W * 4
	case "nebula":
		n = 0 // El campo de gas se calcula directamente sobre el lienzo.
	}
	if n < 8 && r.scene != "nebula" {
		n = 8
	}
	if n > 900 {
		n = 900
	}
	for i := 0; i < n; i++ {
		r.part = append(r.part, r.newParticle(true))
	}
}

func (r *Renderer) newParticle(scatter bool) particle {
	c := r.canvas
	w, h := float64(c.W), float64(c.H)
	p := particle{phase: r.rng.Float64() * math.Pi * 2, size: 1 + r.rng.Float64()*2}
	place := func() float64 {
		if scatter {
			return r.rng.Float64() * h
		}
		return -r.rng.Float64() * h * 0.2
	}
	switch r.scene {
	case "petals":
		p.x = r.rng.Float64() * w
		p.y = place()
		p.vx = 0.25 + r.rng.Float64()*0.5
		p.vy = 0.5 + r.rng.Float64()*0.9
		p.size = 1.5 + r.rng.Float64()*2.5
	case "snow":
		p.x = r.rng.Float64() * w
		p.y = place()
		p.vx = (r.rng.Float64() - 0.5) * 0.3
		p.vy = 0.2 + r.rng.Float64()*0.6
		p.size = 0.6 + r.rng.Float64()*1.6
	case "stars":
		p.x = r.rng.Float64() * w
		p.y = r.rng.Float64() * h * 0.92
		p.vx, p.vy = 0, 0
		p.size = 0.4 + r.rng.Float64()*1.2
		p.life = r.rng.Float64() * 10
	case "rain":
		p.x = r.rng.Float64() * w
		p.y = place()
		p.vx = 0.35
		p.vy = 6 + r.rng.Float64()*7
		p.size = 1 + r.rng.Float64()*1.5
	case "embers":
		p.x = r.rng.Float64() * w
		p.y = h
		p.vx = (r.rng.Float64() - 0.5) * 0.6
		p.vy = -(0.4 + r.rng.Float64()*1.1)
		p.size = 0.7 + r.rng.Float64()*1.8
	case "bubbles":
		p.x = r.rng.Float64() * w
		p.y = h
		p.vx = (r.rng.Float64() - 0.5) * 0.4
		p.vy = -(0.3 + r.rng.Float64()*0.8)
		p.size = 1.5 + r.rng.Float64()*4
	case "meadow":
		p.x = r.rng.Float64() * w
		p.y = r.rng.Float64() * h
		p.vx = (r.rng.Float64() - 0.5) * 0.25
		p.vy = -(0.1 + r.rng.Float64()*0.35)
		p.size = 0.6 + r.rng.Float64()*1.4
	case "city":
		p.x = r.rng.Float64() * w
		p.y = r.rng.Float64() * h * 0.7
		p.vx = (r.rng.Float64() - 0.5) * 0.15
		p.vy = 0.05 + r.rng.Float64()*0.15
		p.size = 1 + r.rng.Float64()*2
	}
	return p
}

func (r *Renderer) wrap(p *particle) {
	c := r.canvas
	w, h := float64(c.W), float64(c.H)
	down := p.vy > 0 || r.scene == "snow" || r.scene == "petals" || r.scene == "rain" || r.scene == "city"
	if down && p.y > h+2 {
		*p = r.newParticle(false)
		p.y = -2
		p.x = r.rng.Float64() * w
	}
	if !down && p.y < -2 {
		*p = r.newParticle(false)
		p.y = h + 2
		p.x = r.rng.Float64() * w
	}
	if p.x > w+2 {
		p.x = -2
	}
	if p.x < -2 {
		p.x = w + 2
	}
}

// ─── Escenas ────────────────────────────────────────────────────────────────

func (r *Renderer) drawPetals(c *Canvas) {
	p := r.pal
	r1, g1, b1 := hexRGB(p.Accent)
	r2, g2, b2 := hexRGB(p.Accent2)
	fr := float64(r.frame)
	for i := range r.part {
		q := &r.part[i]
		sway := math.Sin(fr*0.03+q.phase) * 0.9
		q.x += q.vx + sway
		q.y += q.vy
		r.wrap(q)
		// Pétalo con forma de rombo suave y giro.
		spin := math.Sin(fr*0.05 + q.phase)
		ax := math.Abs(spin) * q.size
		for dy := -q.size; dy <= q.size; dy++ {
			wdist := (1 - math.Abs(dy)/q.size) * ax
			for dx := -wdist; dx <= wdist; dx++ {
				t := 0.5 + 0.5*math.Sin(fr*0.02+q.phase)
				rr := lerp(r1, r2, t)
				gg := lerp(g1, g2, t)
				bb := lerp(b1, b2, t)
				c.blend(int(q.x+dx), int(q.y+dy), rr, gg, bb, 0.85)
			}
		}
		c.glow(q.x, q.y, q.size*3, r1, g1, b1, 0.10)
	}
}

func (r *Renderer) drawSnow(c *Canvas) {
	p := r.pal
	cr, cg, cb := hexRGB(p.Fg)
	fr := float64(r.frame)
	for i := range r.part {
		q := &r.part[i]
		q.x += q.vx + math.Sin(fr*0.02+q.phase)*0.3
		q.y += q.vy
		r.wrap(q)
		c.disc(q.x, q.y, q.size, cr, cg, cb, 0.9)
		c.glow(q.x, q.y, q.size*2.5, cr, cg, cb, 0.18)
	}
}

func (r *Renderer) drawStars(c *Canvas) {
	p := r.pal
	fr := float64(r.frame)
	sr, sg, sb := hexRGB(p.Fg)
	ar, ag, ab := hexRGB(p.Accent)
	// Vía láctea: banda diagonal de brillo tenue.
	for i := 0; i < 40; i++ {
		t := float64(i) / 40
		c.glow(t*float64(c.W), float64(c.H)*(0.15+t*0.35), 26, ar, ag, ab, 0.05)
	}
	for i := range r.part {
		q := &r.part[i]
		tw := 0.45 + 0.55*math.Sin(fr*0.05+q.phase)
		c.disc(q.x, q.y, q.size, sr, sg, sb, tw)
		if q.size > 1.1 {
			c.glow(q.x, q.y, q.size*3, ar, ag, ab, 0.25*tw)
		}
	}
	// Estrellas fugaces ocasionales.
	if r.rng.Float64() < 0.02 {
		r.flashes = append(r.flashes, flash{x: r.rng.Float64() * float64(c.W), y: r.rng.Float64() * float64(c.H) * 0.4, t: 1})
	}
	for i := len(r.flashes) - 1; i >= 0; i-- {
		f := &r.flashes[i]
		f.t -= 0.04
		if f.t <= 0 {
			r.flashes = append(r.flashes[:i], r.flashes[i+1:]...)
			continue
		}
		length := 40.0 * (1 - f.t)
		for k := 0.0; k < length; k += 0.7 {
			c.blend(int(f.x+k), int(f.y+k*0.45), sr, sg, sb, f.t*(1-k/length))
		}
	}
}

func (r *Renderer) drawRain(c *Canvas) {
	p := r.pal
	cr, cg, cb := hexRGB(p.Accent2)
	acR, acG, acB := hexRGB(p.Accent)
	for i := range r.part {
		q := &r.part[i]
		q.x += q.vx
		q.y += q.vy
		r.wrap(q)
		// Cola de la gota.
		for k := 0.0; k < q.size*5; k += 1 {
			a := 0.35 * (1 - k/(q.size*5))
			c.blend(int(q.x-k*0.05), int(q.y-k), cr, cg, cb, a)
		}
		c.blend(int(q.x), int(q.y), acR, acG, acB, 0.8)
	}
	// Charco reflectante en la base.
	for x := 0; x < c.W; x += 1 {
		f := math.Sin(float64(x)*0.06 + float64(r.frame)*0.05)
		if f > 0.7 {
			c.blend(x, c.H-1-int(r.rng.Intn(3)), acR, acG, acB, 0.18*f)
		}
	}
}

func (r *Renderer) drawAurora(c *Canvas) {
	fr := float64(r.frame) * 0.02
	colors := [3][3]float64{}
	c1r, c1g, c1b := hexRGB(r.pal.Accent)
	c2r, c2g, c2b := hexRGB(r.pal.Accent2)
	c3r, c3g, c3b := hexRGB(r.pal.Ok)
	colors[0] = [3]float64{c1r, c1g, c1b}
	colors[1] = [3]float64{c2r, c2g, c2b}
	colors[2] = [3]float64{c3r, c3g, c3b}

	for band := 0; band < 3; band++ {
		col := colors[band]
		phase := fr + float64(band)*1.7
		baseY := float64(c.H) * (0.28 + 0.10*float64(band))
		height := float64(c.H) * (0.42 - 0.06*float64(band))
		for x := 0; x < c.W; x++ {
			fx := float64(x) / float64(c.W)
			y := baseY + math.Sin(fx*4.5+phase)*height*0.30 + math.Sin(fx*11+phase*1.9)*height*0.10
			thick := height * (0.35 + 0.15*math.Sin(fx*3+phase*0.7))
			for k := 0.0; k < thick; k += 0.6 {
				fade := 1 - k/thick
				if y+k < 0 {
					continue
				}
				c.blend(x, int(y+k), col[0], col[1], col[2], 0.22*fade)
			}
		}
	}
}

func (r *Renderer) drawWave(c *Canvas) {
	fr := float64(r.frame) * 0.03
	waves := []struct {
		amp, freq, speed, alpha, y float64
		col                        [3]float64
	}{
		{0.10, 0.018, 1.0, 0.30, 0.55, [3]float64{}},
		{0.07, 0.026, 1.6, 0.22, 0.68, [3]float64{}},
		{0.05, 0.040, 2.3, 0.16, 0.80, [3]float64{}},
	}
	waves[0].col = mulHex(hexRGB(r.pal.Accent))
	waves[1].col = mulHex(hexRGB(r.pal.Accent2))
	waves[2].col = mulHex(hexRGB(r.pal.Ok))

	for _, w := range waves {
		base := float64(c.H) * w.y
		for x := 0; x < c.W; x++ {
			y := base + math.Sin(float64(x)*w.freq+fr*w.speed)*float64(c.H)*w.amp
			for k := 0.0; k < float64(c.H)*0.04; k += 0.5 {
				c.blend(x, int(y+k), w.col[0], w.col[1], w.col[2], w.alpha*(1-k/(float64(c.H)*0.04)))
			}
		}
	}
}

func mulHex(r, g, b float64) [3]float64 { return [3]float64{r, g, b} }

func (r *Renderer) drawCity(c *Canvas) {
	// Silueta de edificios determinista.
	rng := rand.New(rand.NewSource(7))
	x := 0
	skyline := make([]int, c.W)
	for x < c.W {
		bw := 3 + rng.Intn(7)
		bh := int(float64(c.H) * (0.25 + rng.Float64()*0.45))
		for i := 0; i < bw && x+i < c.W; i++ {
			skyline[x+i] = bh
		}
		x += bw
	}
	bgR, bgG, bgB := hexRGB(r.pal.BgAlt)
	dark := 0.55
	for x := 0; x < c.W; x++ {
		for y := 0; y < skyline[x]; y++ {
			row := c.H - 1 - y
			c.set(x, row, bgR*dark, bgG*dark, bgB*dark)
		}
	}
	// Ventanas encendidas que parpadean.
	ar, ag, ab := hexRGB(r.pal.Accent)
	a2r, a2g, a2b := hexRGB(r.pal.Accent2)
	fr := float64(r.frame)
	for x := 1; x < c.W-1; x += 3 {
		bh := skyline[x]
		if bh == 0 {
			continue
		}
		for y := 2; y < bh-1; y += 3 {
			h := math.Sin(float64(x)*12.9898+float64(y)*78.233) * 43758.5453
			frac := h - math.Floor(h)
			if frac > 0.45 {
				continue
			}
			on := math.Sin(fr*0.05+frac*40) > -0.3
			if !on {
				continue
			}
			col := [3]float64{ar, ag, ab}
			if frac > 0.75 {
				col = [3]float64{a2r, a2g, a2b}
			}
			row := c.H - 1 - y
			c.blend(x, row, col[0], col[1], col[2], 0.75)
			c.glow(float64(x), float64(row), 2.5, col[0], col[1], col[2], 0.20)
		}
	}
	// Neones flotantes.
	for i := range r.part {
		q := &r.part[i]
		q.y += q.vy
		q.x += q.vx
		r.wrap(q)
		t := 0.5 + 0.5*math.Sin(fr*0.04+q.phase)
		c.glow(q.x, q.y, q.size*4, lerp(ar, a2r, t), lerp(ag, a2g, t), lerp(ab, a2b, t), 0.35)
	}
}

func (r *Renderer) drawEmbers(c *Canvas) {
	fr := float64(r.frame)
	ar, ag, ab := hexRGB(r.pal.Accent)
	a2r, a2g, a2b := hexRGB(r.pal.Accent2)
	for i := range r.part {
		q := &r.part[i]
		q.x += q.vx + math.Sin(fr*0.04+q.phase)*0.5
		q.y += q.vy
		r.wrap(q)
		life := 0.5 + 0.5*math.Sin(fr*0.03+q.phase)
		t := life
		c.glow(q.x, q.y, q.size*3.5, lerp(ar, a2r, t), lerp(ag, a2g, t), lerp(ab, a2b, t), 0.45)
		c.blend(int(q.x), int(q.y), 255, lerp(180, 230, life), lerp(80, 160, life), 0.85)
	}
}

func (r *Renderer) drawGrid(c *Canvas) {
	fr := float64(r.frame)
	ar, ag, ab := hexRGB(r.pal.Accent)
	a2r, a2g, a2b := hexRGB(r.pal.Accent2)
	horizon := c.H * 45 / 100

	// Sol de neón.
	radius := float64(c.H) * 0.22
	cx := float64(c.W) / 2
	cy := float64(horizon) - radius*0.35
	for y := int(cy - radius); y <= int(cy+radius); y++ {
		for x := int(cx - radius); x <= int(cx+radius); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			d := math.Hypot(dx, dy)
			if d > radius {
				continue
			}
			t := float64(y-int(cy+radius)) / (2 * radius)
			_ = t
			// Franjas horizontales estilo synthwave.
			if (y/3)%2 == 0 && y > int(cy) {
				continue
			}
			blendT := 1 - d/radius
			c.blend(x, y, lerp(ar, a2r, blendT), lerp(ag, a2g, blendT), lerp(ab, a2b, blendT), 0.75)
		}
	}
	c.glow(cx, cy, radius*2.2, ar, ag, ab, 0.16)

	// Rejilla en perspectiva.
	for i := 0; i < 14; i++ {
		z := math.Mod(float64(i)+fr*0.02, 14) / 14
		y := float64(horizon) + math.Pow(z, 2.2)*float64(c.H-horizon)
		c.cline(0, y, float64(c.W), y, ar, ag, ab, 0.35*(1-z*0.5))
	}
	for i := -12; i <= 12; i++ {
		xTop := cx + float64(i)*float64(c.W)/26
		xBot := cx + float64(i)*float64(c.W)/6
		c.cline(xTop, float64(horizon), xBot, float64(c.H), ar, ag, ab, 0.30)
	}
}

func (r *Renderer) drawMeadow(c *Canvas) {
	fr := float64(r.frame)
	gr, gg, gb := hexRGB(r.pal.Ok)
	ar, ag, ab := hexRGB(r.pal.Accent2)
	// Hierba: tallos que se mecen.
	rng := rand.New(rand.NewSource(11))
	for x := 0; x < c.W; x++ {
		baseH := c.H * 35 / 100
		height := baseH/2 + int(float64(baseH)*float64(rng.Intn(100))/100)
		sway := math.Sin(fr*0.04+float64(x)*0.2) * 1.6
		x0 := float64(x)
		y0 := float64(c.H - 1)
		x1 := x0 + sway
		y1 := float64(c.H - height)
		c.cline(x0, y0, x1, y1, gr, gg, gb, 0.35)
		if (x+rng.Intn(3))%7 == 0 {
			c.blend(int(x1), int(y1), ar, ag, ab, 0.8)
		}
	}
	// Polen flotando.
	for i := range r.part {
		q := &r.part[i]
		q.x += q.vx + math.Sin(fr*0.02+q.phase)*0.4
		q.y += q.vy
		r.wrap(q)
		c.glow(q.x, q.y, q.size*2.5, ar, ag, ab, 0.35)
	}
}

func (r *Renderer) drawBubbles(c *Canvas) {
	p := r.pal
	cr, cg, cb := hexRGB(p.Accent)
	ar, ag, ab := hexRGB(p.Accent2)
	fr := float64(r.frame)
	for i := range r.part {
		q := &r.part[i]
		q.x += q.vx + math.Sin(fr*0.03+q.phase)*0.6
		q.y += q.vy
		r.wrap(q)
		// Aro de la burbuja.
		steps := int(q.size * 8)
		if steps < 8 {
			steps = 8
		}
		for k := 0; k < steps; k++ {
			a := float64(k) / float64(steps) * 2 * math.Pi
			x := q.x + math.Cos(a)*q.size
			y := q.y + math.Sin(a)*q.size
			c.blend(int(x), int(y), cr, cg, cb, 0.55)
		}
		c.blend(int(q.x-q.size*0.35), int(q.y-q.size*0.35), 255, 255, 255, 0.7)
		c.glow(q.x, q.y, q.size*2, ar, ag, ab, 0.10)
	}
}

// ─── Utilidades de exportación ──────────────────────────────────────────────

// ToJSON serializa la información de las escenas (para `swaydesk wallpaper --scenes --json`).
func ToJSON(list []SceneInfo) string {
	b, _ := json.MarshalIndent(list, "", "  ")
	return string(b)
}
