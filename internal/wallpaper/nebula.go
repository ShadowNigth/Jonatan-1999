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
	phase := float64(r.frame) * 0.03

	// La fase sigue al contador global de cuadros, sin límite de duración. El
	// ruido fluye continuamente sin calcular ruido fractal a resolución completa.
	for gy := 0; gy < gridH; gy++ {
		v := float64(gy*cellSize) / math.Max(1, float64(c.H-1))
		for gx := 0; gx < gridW; gx++ {
			u := float64(gx*cellSize) / math.Max(1, float64(c.W-1))
			nx := u*3.8 + phase*0.35
			ny := v*2.7 - phase*0.26
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
	breath := 0.88 + 0.12*math.Sin(phase*2.3)
	centerAX := 0.32 + 0.065*math.Sin(phase*1.05)
	centerAY := 0.46 + 0.050*math.Cos(phase*0.9)
	centerBX := 0.72 + 0.070*math.Sin(phase*0.75+1.1)
	centerBY := 0.58 + 0.060*math.Cos(phase*0.85+0.4)
	filamentPulse := 0.74 + 0.26*math.Sin(phase*3.0)
	filamentShift := 0.032 * math.Sin(phase*1.6)

	for y := 0; y < c.H; y++ {
		v := float64(y) / math.Max(1, float64(c.H-1))
		gy := y / cellSize
		ty := float64(y%cellSize) / cellSize
		ty = ty * ty * (3 - 2*ty)
		for x := 0; x < c.W; x++ {
			u := float64(x) / math.Max(1, float64(c.W-1))
			gx := x / cellSize
			tx := float64(x%cellSize) / cellSize
			tx = tx * tx * (3 - 2*tx)
			i := gy*gridW + gx
			n0 := lerp(noise[i], noise[i+1], tx)
			n1 := lerp(noise[i+gridW], noise[i+gridW+1], tx)
			cloudNoise := lerp(n0, n1, ty)
			warp := (cloudNoise - 0.5) * 0.22

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
			filamentY := 0.76 - 0.72*u + 0.22*u*u + (cloudNoise-0.5)*0.12 + filamentShift
			filament := math.Max(0, 1-math.Abs(v-filamentY)/0.085)
			filament *= filament * math.Max(0, 1-math.Abs(u-0.48)/0.55)
			filamentY2 := 0.22 + 0.55*u - 0.20*u*u - (cloudNoise-0.5)*0.10 - filamentShift*0.7
			filament2 := math.Max(0, 1-math.Abs(v-filamentY2)/0.07)
			filament2 *= filament2 * math.Max(0, 1-math.Abs(u-0.52)/0.58)

			alphaA := lobeA * strength * breath
			alphaB := lobeB * strength * 0.82 * breath
			alphaFilament := (filament*(0.18+0.42*lobeA) + filament2*(0.10+0.30*lobeB)) * strength * 0.40 * breath * filamentPulse
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
	r.drawNebulaDust(c, mixR, mixG, mixB, accent2R, accent2G, accent2B)
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
		baseX, baseY := float64(rng.Intn(c.W)), float64(rng.Intn(c.H))
		xPos := math.Mod(baseX+math.Sin(frame*0.010+float64(i)*1.5)*2.4, float64(c.W))
		yPos := math.Mod(baseY+math.Cos(frame*0.008+float64(i)*1.2)*1.4, float64(c.H))
		if xPos < 0 {
			xPos += float64(c.W)
		}
		if yPos < 0 {
			yPos += float64(c.H)
		}
		x, y := int(xPos), int(yPos)
		brightness := 0.45 + 0.55*(0.5+0.5*math.Sin(frame*0.09+float64(i)*1.83))
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

// drawNebulaDust añade motas cálidas y frías que atraviesan las nubes con una
// estela corta. Sus trayectorias lentas aportan parallax sin tapar el fondo.
func (r *Renderer) drawNebulaDust(c *Canvas, colorA, colorB, colorC, altR, altG, altB float64) {
	count := c.W * c.H / 14000
	if count < 16 {
		count = 16
	}
	if count > 120 {
		count = 120
	}
	rng := rand.New(rand.NewSource(0x44555354))
	frame := float64(r.frame)
	w, h := float64(c.W), float64(c.H)
	for i := 0; i < count; i++ {
		baseX, baseY := rng.Float64()*w, rng.Float64()*h
		speedX := 0.12 + rng.Float64()*0.22
		if rng.Intn(2) == 0 {
			speedX = -speedX
		}
		speedY := (rng.Float64() - 0.5) * 0.036
		phase := rng.Float64() * 2 * math.Pi
		x := math.Mod(baseX+frame*speedX, w)
		y := math.Mod(baseY+frame*speedY+math.Sin(frame*0.03+phase)*1.8, h)
		if x < 0 {
			x += w
		}
		if y < 0 {
			y += h
		}

		pulse := 0.42 + 0.58*(0.5+0.5*math.Sin(frame*0.11+phase))
		alpha := pulse * (0.15 + rng.Float64()*0.17)
		red, green, blue := colorA, colorB, colorC
		if i%3 == 0 {
			red, green, blue = altR, altG, altB
		}
		if i%6 == 0 {
			tailY := y - 1.4*math.Cos(frame*0.03+phase)
			c.cline(x-speedX*5, tailY, x, y, red, green, blue, alpha*0.42)
			c.glow(x, y, 2.8, red, green, blue, pulse*0.07)
		}
		c.blend(int(x), int(y), red, green, blue, alpha)
		c.blend(int(x-speedX*2), int(y), red, green, blue, alpha*0.16)
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
