package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
)

// FitSize calcula el tamaño en celdas del terminal que mejor conserva la
// proporción de una imagen. Cada celda es aproximadamente el doble de alta que
// de ancha, de ahí el factor 2 en el alto.
func FitSize(imgW, imgH, maxW, maxH, pixelRows int) (int, int) {
	if imgW <= 0 || imgH <= 0 || maxW <= 0 || maxH <= 0 {
		return 0, 0
	}
	if pixelRows < 1 {
		pixelRows = 2
	}
	// Relación ancho/alto de una celda en píxeles "cuadrados".
	cellRatio := float64(pixelRows) / 2.0
	w := maxW
	h := int(float64(imgH) / float64(imgW) * float64(w) / cellRatio)
	if h > maxH {
		h = maxH
		w = int(float64(imgW) / float64(imgH) * float64(h) * cellRatio)
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func toRGBA(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func hexOf(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// nearest devuelve el píxel de la imagen escalado al tamaño indicado usando
// interpolación bilineal (suficiente para miniaturas y fondos animados).
func nearest(src image.Image, x, y, sw, sh, dw, dh int) (uint8, uint8, uint8) {
	if sw <= 0 || sh <= 0 {
		return 0, 0, 0
	}
	// Centro del píxel destino mapeado al origen.
	fx := (float64(x)+0.5)*float64(sw)/float64(dw) - 0.5
	fy := (float64(y)+0.5)*float64(sh)/float64(dh) - 0.5
	x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
	dx, dy := fx-float64(x0), fy-float64(y0)
	clampX := func(v int) int {
		if v < 0 {
			return 0
		}
		if v >= sw {
			return sw - 1
		}
		return v
	}
	clampY := func(v int) int {
		if v < 0 {
			return 0
		}
		if v >= sh {
			return sh - 1
		}
		return v
	}
	r00, g00, b00 := toRGBA(src.At(clampX(x0), clampY(y0)))
	r10, g10, b10 := toRGBA(src.At(clampX(x0+1), clampY(y0)))
	r01, g01, b01 := toRGBA(src.At(clampX(x0), clampY(y0+1)))
	r11, g11, b11 := toRGBA(src.At(clampX(x0+1), clampY(y0+1)))
	mix := func(a, b, c, d uint8) uint8 {
		top := float64(a)*(1-dx) + float64(b)*dx
		bot := float64(c)*(1-dx) + float64(d)*dx
		return uint8(top*(1-dy) + bot*dy)
	}
	return mix(r00, r10, r01, r11), mix(g00, g10, g01, g11), mix(b00, b10, b01, b11)
}

// RenderImage convierte una imagen en líneas de texto que la representan en el
// terminal usando medios bloques ("▀"): cada celda muestra dos píxeles, uno en
// el color de primer plano y otro en el de fondo.
//
// Con color de 24 bits la fidelidad es alta; con 256 colores se aproxima y, sin
// color, se recurre a una rampa de densidades.
func RenderImage(img image.Image, w, h int) []string {
	if img == nil || w < 1 || h < 1 {
		return nil
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	lines := make([]string, 0, h)
	var sb strings.Builder

	for row := 0; row < h; row++ {
		sb.Reset()
		for col := 0; col < w; col++ {
			y1 := row * 2
			y2 := row*2 + 1
			r1, g1, b1 := nearest(img, col, y1, sw, sh, w, h*2)
			r2, g2, b2 := nearest(img, col, y2, sw, sh, w, h*2)
			switch profile {
			case ProfileNone:
				lum := (float64(r1) + float64(g1) + float64(b1)) / 3
				ramp := []rune(" .:-=+*#%@")
				sb.WriteRune(ramp[int(lum)/256*len(ramp)%len(ramp)])
			case ProfileTrueColor:
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
					r1, g1, b1, r2, g2, b2))
			default:
				fg := xterm256Index(int(r1), int(g1), int(b1))
				bg := xterm256Index(int(r2), int(g2), int(b2))
				sb.WriteString(fmt.Sprintf("\x1b[38;5;%dm\x1b[48;5;%dm▀", fg, bg))
			}
		}
		if profile != ProfileNone {
			sb.WriteString("\x1b[0m")
		}
		lines = append(lines, sb.String())
	}
	return lines
}

// AverageColor devuelve el color medio de la imagen (útil para fondos).
func AverageColor(img image.Image) Color {
	if img == nil {
		return ""
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	step := sw*sh/2000 + 1
	var r, g, bl, n uint64
	for y := 0; y < sh; y += max(1, step/sw+1) {
		for x := 0; x < sw; x += max(1, step/sh+1) {
			pr, pg, pb := toRGBA(img.At(b.Min.X+x, b.Min.Y+y))
			r += uint64(pr)
			g += uint64(pg)
			bl += uint64(pb)
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return Color(hexOf(uint8(r/n), uint8(g/n), uint8(bl/n)))
}
