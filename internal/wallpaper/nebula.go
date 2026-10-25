package wallpaper

import (
	"math"
	"math/rand"
)

// drawNebula genera nubes y filamentos de polvo con ruido de valor a baja
// resolución, interpolado al lienzo. Los colores vienen de la paleta activa;
// applyTheme reinicia el motor para que la escena adopte el tema nuevo.
func (r *Renderer) drawNebula(c *Canvas) {
	if c == nil || c.W < 1 || c.H < 1 {
		return
	}

	const cellSize = 6
	gridW := (c.W+cellSize-1)/cellSize + 1
	gridH := (c.H+cellSize-1)/cellSize + 1
	noise := make([]float64, gridW*gridH)
	phase := float64(r.frame) * 0.012

	// El campo de ruido se desplaza lentamente para que el gas parezca fluir,
	// sin tener que calcular ruido fractal a resolución completa en cada frame.
	for gy := 0; gy < gridH; gy++ {
		v := float64(gy*cellSize) / math.Max(1, float64(c.H-1))
		for gx := 0; gx < gridW; gx++ {
			u := float64(gx*cellSize) / math.Max(1, float64(c.W-1))
			nx := u*3.8 + phase*0.14
			ny := v*2.7 - phase*0.10
			coarse := nebulaValueNoise(nx, ny, 17)
			medium := nebulaValueNoise(nx*2.03+2.4, ny*2.03-1.7, 53)
			fine := nebulaValueNoise(nx*4.07-3.1, ny*4.07+5.2, 97)
			noise[gy*gridW+gx] = 0.5 + 0.25*coarse + 0.16*medium + 0.09*fine
		}
	}

	accentR, accentG, accentB := hexRGB(r.pal.Accent)
	accent2R, accent2G, accent2B := hexRGB(r.pal.Accent2)
	mutedR, mutedG, mutedB := hexRGB(r.pal.Muted)
	mixR, mixG, mixB := lerp(accentR, accent2R, 0.5), lerp(accentG, accent2G, 0.5), lerp(accentB, accent2B, 0.5)
	strength := 0.48
	if r.light {
		strength = 0.28
	}
	breath := 0.94 + 0.06*math.Sin(phase*1.7)
	centerAX := 0.32 + 0.035*math.Sin(phase*0.8)
	centerAY := 0.46 + 0.028*math.Cos(phase*0.7)
	centerBX := 0.72 + 0.040*math.Sin(phase*0.53+1.1)
	centerBY := 0.58 + 0.035*math.Cos(phase*0.61+0.4)

	for y := 0; y < c.H; y++ {
		v := float64(y) / math.Max(1, float64(c.H-1))
		gy := y / cellSize
		ty := float64(y%cellSize) / cellSize
		ty = tty * tty * (3 - 2*tty)
		for x := 0; x < c.W; x++ {
			u := float64(x) / math.Max(1, float64(c.W-1))
			gx := x / cellSize
			tx := float64(x%cellSize) / cellSize
			tx = tx * tx * (3 - 2*tx)
			i := gy*gridW + gx
			n0 := lerp(noise[i], noise[i+1], tx)
			n1 := lerp(noise[i+gridW], noise[i+gridW+1], tx)
			cloudNoise := lerp(n0, n1, ty)
			warp := (cloudNoise - 0.5) * 0.15

			// Dos masas elongadas y rotadas forman la nube principal y su cola.
			dxA, dyA := u-centerAX, v-centerAY+warp
			aX, aY := dxA+dyA*0.24, dyA-dxA*0.20
			qA := (aX*aX)/(0.58*0.58) + (aY*aY)/(0.48*0.48)
			lobeA := math.Max(0, 1-qA)
			lobeA *= lobeA * (0.55 + 0.45*cloudNoise)

			dxB, dyB := u-centerBX, v-centerBY-warp*0.65
			bX, bY := dxB-dyB*0.31, dyB+dxB*0.18
			qB := (bX*bX)/(0.43*0.43) + (bY*bY)/(0.62*0.62)
			lobeB := math.Max(0, 1-qB)
			lobeB *= lobeB * (0.52 + 0.48*(1-cloudNoise))

			// Un filamento curvo rompe los bordes redondos y añade estructura.
			filamentY := 0.76 - 0.72*u + 0.22*u*u + (cloudNoise-0.5)*0.12
			filament := math.Max(0, 1-math.Abs(v-filamentY)/0.085)
			filament *= filament * math.Max(0, 1-math.Abs(u-0.48)/0.55)

			alphaA := lobeA * strength * breath
			alphaB := lobeB * strength * 0.82 * breath
			alphaFilament := filament * (0.18 + 0.42*lobeA) * strength * 0.40 * breath
			if alphaA > 0.004 {
				c.blend(x, y, accentR, accentG, accentB, alphaA)
			}
			if alphaB > 0.004 {
				c.blend(x, y, accent2R, accent2G, accent2B, alphaB)
			}
			if alphaFilament > 0.004 {
				c.blend(x, y, mixR, mixG, mixB, alphaFilament)
			}
			mist := (lobeA*0.12 + lobeB*0.10) * strength
			if mist > 0.004 {
				c.blend(x, y, mutedR, mutedG, mutedB, mist)
			}
		}
	}

	r.drawNebulaStars(c, accent2R, accent2G, accent2B)
}

func (r *Renderer) drawNebulaStars(c *Canvas, starR, starG, starB float64) {
	count := c.W * c.H / 6000
	if count < 24 {
		count = 24
	}
	if count > 180 {
		count = 180
	}
	rng := rand.New(rand.NewSource(0x4E454255))
	frame := float64(r.frame)
	for i := 0; i < count; i++ {
		x := rng.Intn(c.W)
		y := rng.Intn(c.H)
		brightness := 0.48 + 0.52*(0.5+0.5*math.Sin(frame*0.045+float64(i)*1.83))
		alpha := brightness * (0.30 + rng.Float64()*0.38)
		c.blend(x, y, starR, starG, starB, alpha)
		if i%13 == 0 {
			c.blend(x-1, y, starR, starG, starB, alpha*0.18)
			c.blend(x+1, y, starR, starG, starB, alpha*0.18)
			c.blend(x, y-1, starR, starG, starB, alpha*0.18)
			c.blend(x, y+1, starR, starG, starB, alpha*0.18)
		}
	}
}

func nebulaValueNoise(x, y float64, seed uint64) float64 {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	tx, ty := x-float64(x0), y-float64(y0)
	tx = tx * tx * (3 - 2*tx)
	ty = ty * ty * (3 - 2*ty)
	a := nebulaHash(x0, y0, seed)
	b := nebulaHash(x0+1, y0, seed)
	c := nebulaHash(x0, y0+1, seed)
	d := nebulaHash(x0+1, y0+1, seed)
	return lerp(lerp(a, b, tx), lerp(c, d, tx), ty)
}

func nebulaHash(x, y int, seed uint64) float64 {
	n := seed ^ uint64(int64(x))*0x9e3779b185ebca87 ^ uint64(int64(y))*0xc2b2ae3d27d4eb4f
	n ^= n >> 30
	n *= 0xbf58476d1ce4e5b9
	n ^= n >> 27
	n *= 0x94d049bb133111eb
	n ^= n >> 31
	return float64(n>>40)/float64(1<<23) - 1
}
