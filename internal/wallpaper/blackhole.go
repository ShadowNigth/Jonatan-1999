package wallpaper

import (
	"math"
	"math/rand"
)

// drawBlackHole representa el cielo en coordenadas físicas: en la capa Wayland
// cada fila del lienzo se amplía al doble, mientras que la vista previa usa
// medios bloques casi cuadrados. pixelAspectY conserva la geometría circular
// del horizonte en ambos casos.
func (r *Renderer) drawBlackHole(c *Canvas, pixelAspectY float64) {
	if c == nil || c.W < 1 || c.H < 1 {
		return
	}
	if pixelAspectY <= 0 {
		pixelAspectY = 1
	}

	w, h := float64(c.W), float64(c.H)
	physicalH := h * pixelAspectY
	shortSide := math.Min(w, physicalH)
	cx, cy := w*0.5, physicalH*0.50
	horizon := math.Min(shortSide*0.18, w*0.10)
	if horizon < 1 {
		horizon = 1
	}
	innerDisk, outerDisk := horizon*1.28, horizon*3.55

	// Espacio profundo con una nebulosa azul muy tenue, más visible detrás del
	// disco, sin depender de que la paleta activa sea oscura.
	nebulaX, nebulaY := outerDisk*2.2, outerDisk*1.5
	for y := 0; y < c.H; y++ {
		fy := float64(y) / math.Max(1, h-1)
		dy := (float64(y)+0.5)*pixelAspectY - cy
		ny := dy / nebulaY
		for x := 0; x < c.W; x++ {
			dx := (float64(x) + 0.5) - cx
			nx := dx / nebulaX
			cloud := math.Max(0, 1-nx*nx-ny*ny)
			cloud *= cloud
			i := (y*c.W + x) * 3
			c.Pix[i] = clamp8(2 + 1.2*fy + 8*cloud)
			c.Pix[i+1] = clamp8(3 + 1.8*fy + 7*cloud)
			c.Pix[i+2] = clamp8(8 + 3*fy + 26*cloud)
		}
	}

	r.drawBlackHoleStars(c, cx, cy, pixelAspectY, horizon, outerDisk)
	r.drawAccretionDisk(c, cx, cy, pixelAspectY, horizon, innerDisk, outerDisk, false)
	drawBlackHoleShadow(c, cx, cy, pixelAspectY, horizon)
	// El lado cercano del plasma queda delante del borde de la sombra, pero
	// nunca pinta sobre el horizonte de sucesos.
	r.drawAccretionDisk(c, cx, cy, pixelAspectY, horizon, innerDisk, outerDisk, true)
	r.drawPhotonRing(c, cx, cy, pixelAspectY, horizon)
}

func (r *Renderer) drawBlackHoleStars(c *Canvas, cx, cy, pixelAspectY, horizon, outerDisk float64) {
	count := c.W * c.H / 3600
	if count < 32 {
		count = 32
	}
	if count > 320 {
		count = 320
	}

	// La semilla fija mantiene las estrellas en su sitio; solo su brillo titila.
	rng := rand.New(rand.NewSource(0xB10C0FF))
	frame := float64(r.frame)
	physicalH := float64(c.H) * pixelAspectY
	for i := 0; i < count; i++ {
		sx := rng.Float64() * float64(c.W)
		sy := rng.Float64() * physicalH
		dx, dy := sx-cx, sy-cy
		distance := math.Hypot(dx, dy)
		angle := math.Atan2(dy, dx)
		ghostX, ghostY := 0.0, 0.0
		hasGhost := false
		if distance < outerDisk*1.25 {
			// Ecuación de lente de masa puntual: produce una imagen principal
			// desviada y, si no cae tras el horizonte, una secundaria tenue.
			if distance < horizon*0.02 {
				angle = rng.Float64() * 2 * math.Pi
			}
			einsteinRadius := horizon * 1.32
			root := math.Sqrt(distance*distance + 4*einsteinRadius*einsteinRadius)
			primaryRadius := 0.5 * (distance + root)
			secondaryRadius := 0.5 * (root - distance)
			sx = cx + primaryRadius*math.Cos(angle)
			sy = cy + primaryRadius*math.Sin(angle)
			if secondaryRadius > horizon*1.045 {
				ghostX = cx - secondaryRadius*math.Cos(angle)
				ghostY = cy - secondaryRadius*math.Sin(angle)
				hasGhost = true
			}
		}

		x, y := int(sx), int(sy/pixelAspectY)
		if x < 0 || x >= c.W || y < 0 || y >= c.H {
			continue
		}
		peak := 65 + rng.Intn(150)
		twinkle := 0.42 + 0.58*(0.5+0.5*math.Sin(frame*0.045+float64(i)*1.71))
		alpha := twinkle * (0.35 + float64(peak)/330)
		c.blend(x, y, float64(peak)*0.82, float64(peak)*0.91, float64(peak), alpha)
		if hasGhost {
			c.blend(int(ghostX), int(ghostY/pixelAspectY), float64(peak)*0.68, float64(peak)*0.82, float64(peak), alpha*0.28)
		}
		if peak > 175 {
			c.blend(x-1, y, float64(peak)*0.48, float64(peak)*0.58, float64(peak)*0.72, alpha*0.22)
			c.blend(x+1, y, float64(peak)*0.48, float64(peak)*0.58, float64(peak)*0.72, alpha*0.22)
			c.blend(x, y-1, float64(peak)*0.48, float64(peak)*0.58, float64(peak)*0.72, alpha*0.22)
			c.blend(x, y+1, float64(peak)*0.48, float64(peak)*0.58, float64(peak)*0.72, alpha*0.22)
		}
	}
}

func (r *Renderer) drawAccretionDisk(c *Canvas, cx, cy, pixelAspectY, horizon, inner, outer float64, foregroundOnly bool) {
	const inclination = 0.105
	frame := float64(r.frame)
	yExtent := outer * inclination
	x0 := max(0, int(math.Floor(cx-outer)))
	x1 := min(c.W-1, int(math.Ceil(cx+outer)))
	y0 := max(0, int(math.Floor((cy-yExtent)/pixelAspectY)))
	y1 := min(c.H-1, int(math.Ceil((cy+yExtent)/pixelAspectY)))

	for y := y0; y <= y1; y++ {
		dy := (float64(y)+0.5)*pixelAspectY - cy
		if foregroundOnly && dy <= 0 {
			continue
		}
		for x := x0; x <= x1; x++ {
			dx := (float64(x) + 0.5) - cx
			diskRadius := math.Hypot(dx, dy/inclination)
			if diskRadius <= inner || diskRadius >= outer {
				continue
			}
			if foregroundOnly && math.Hypot(dx, dy) < horizon*1.045 {
				continue
			}

			innerFade := (diskRadius - inner) / (horizon * 0.12)
			outerFade := (outer - diskRadius) / (horizon * 0.24)
			edge := math.Min(1, math.Min(innerFade, outerFade))
			if edge <= 0 {
				continue
			}
			heat := math.Pow(1-(diskRadius-inner)/(outer-inner), 0.72)
			angle := math.Atan2(dy/inclination, dx)
			approaching := math.Max(0, -math.Cos(angle))
			doppler := 0.82 + 0.34*approaching
			flow := 0.84 + 0.10*math.Sin(angle*15+math.Log(diskRadius/horizon)*12-frame*0.035)
			flow += 0.06 * math.Sin(angle*31-diskRadius/horizon*8+frame*0.021)

			red := lerp(196, 255, heat)
			green := lerp(36, 225, heat)
			blue := lerp(10, 184, heat)
			red = lerp(red, 255, approaching*0.24)
			green = lerp(green, 246, approaching*0.19)
			blue = lerp(blue, 236, approaching*0.15)
			alpha := (0.34 + 0.38*heat) * edge * flow * doppler
			if foregroundOnly {
				alpha *= 0.24
			}
			c.blend(x, y, red, green, blue, alpha)
		}
	}
}

func drawBlackHoleShadow(c *Canvas, cx, cy, pixelAspectY, horizon float64) {
	x0 := max(0, int(math.Floor(cx-horizon*1.08)))
	x1 := min(c.W-1, int(math.Ceil(cx+horizon*1.08)))
	y0 := max(0, int(math.Floor((cy-horizon*1.08)/pixelAspectY)))
	y1 := min(c.H-1, int(math.Ceil((cy+horizon*1.08)/pixelAspectY)))
	for y := y0; y <= y1; y++ {
		dy := (float64(y)+0.5)*pixelAspectY - cy
		for x := x0; x <= x1; x++ {
			dx := (float64(x) + 0.5) - cx
			if math.Hypot(dx, dy) > horizon*1.025 {
				continue
			}
			i := (y*c.W + x) * 3
			if math.Hypot(dx, dy) > horizon*0.91 {
				c.Pix[i], c.Pix[i+1], c.Pix[i+2] = 1, 2, 7
			} else {
				c.Pix[i], c.Pix[i+1], c.Pix[i+2] = 0, 0, 1
			}
		}
	}
}

func (r *Renderer) drawPhotonRing(c *Canvas, cx, cy, pixelAspectY, horizon float64) {
	ringRadius := horizon * 1.075
	ringWidth := math.Max(0.7, horizon*0.026)
	bound := ringRadius + ringWidth*3.2
	x0 := max(0, int(math.Floor(cx-bound)))
	x1 := min(c.W-1, int(math.Ceil(cx+bound)))
	y0 := max(0, int(math.Floor((cy-bound)/pixelAspectY)))
	y1 := min(c.H-1, int(math.Ceil((cy+bound)/pixelAspectY)))
	frame := float64(r.frame)

	for y := y0; y <= y1; y++ {
		dy := (float64(y)+0.5)*pixelAspectY - cy
		for x := x0; x <= x1; x++ {
			dx := (float64(x) + 0.5) - cx
			distance := math.Hypot(dx, dy)
			delta := distance - ringRadius
			if math.Abs(delta) > ringWidth*3.2 {
				continue
			}
			angle := math.Atan2(dy, dx)
			beaming := 0.66 + 0.20*math.Max(0, -math.Sin(angle)) + 0.14*math.Max(0, -math.Cos(angle))
			flicker := 0.96 + 0.04*math.Sin(frame*0.032+angle*2)
			core := math.Exp(-0.5 * delta * delta / (ringWidth * ringWidth))
			c.blend(x, y, 255, 218, 160, core*beaming*flicker)
			corona := math.Exp(-0.5 * delta * delta / (horizon * horizon * 0.012))
			c.blend(x, y, 255, 103, 34, corona*0.18)
		}
	}
}
