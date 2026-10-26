package wallpaper

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Still es una imagen fija ya cargada.
type Still struct {
	Path string
	Img  image.Image
	W, H int
}

// LoadStill abre una imagen (PNG/JPEG/GIF) y devuelve su primer fotograma.
func LoadStill(path string) (*Still, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	b := img.Bounds()
	return &Still{Path: path, Img: img, W: b.Dx(), H: b.Dy()}, nil
}

// GIF es un GIF animado cargado en memoria, listo para dibujar.
type GIF struct {
	Path     string
	frames   []*image.RGBA
	delays   []time.Duration
	frameEnd []time.Duration // instante acumulado en que termina cada fotograma
	W, H     int
	total    time.Duration
	// Loop indica si el GIF debe repetirse al terminar (siempre true por ahora).
	Loop bool
}

// LoadGIF carga y decodifica un GIF animado.
//
// Los fotogramas se componen siguiendo el método de disposición del estándar,
// de modo que los fotogramas parciales (los que solo contienen la zona que
// cambia) se dibujan correctamente.
func LoadGIF(path string) (*GIF, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("el gif no tiene fotogramas: %s", path)
	}
	width, height := g.Config.Width, g.Config.Height
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	frames := make([]*image.RGBA, 0, len(g.Image))
	delays := make([]time.Duration, 0, len(g.Image))
	frameEnd := make([]time.Duration, 0, len(g.Image))
	var total time.Duration

	var previousFrame *image.Paletted
	var canvasBeforePrevious *image.RGBA
	for i, frame := range g.Image {
		if i > 0 && previousFrame != nil {
			disposal := byte(0)
			if i-1 < len(g.Disposal) {
				disposal = g.Disposal[i-1]
			}
			switch disposal {
			case gif.DisposalBackground:
				clearGIFFrame(canvas, previousFrame, g)
			case gif.DisposalPrevious:
				if canvasBeforePrevious != nil {
					copy(canvas.Pix, canvasBeforePrevious.Pix)
				}
			}
		}
		before := cloneRGBA(canvas)
		drawOver(canvas, frame)
		frames = append(frames, cloneRGBA(canvas))
		previousFrame = frame
		canvasBeforePrevious = before

		d := time.Duration(0)
		if i < len(g.Delay) {
			d = time.Duration(g.Delay[i]) * 10 * time.Millisecond
		}
		if d <= 0 {
			d = 100 * time.Millisecond
		}
		delays = append(delays, d)
		total += d
		frameEnd = append(frameEnd, total)
	}
	return &GIF{
		Path: path, frames: frames, delays: delays, frameEnd: frameEnd,
		W: width, H: height, total: total, Loop: true,
	}, nil
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	copyImage := image.NewRGBA(src.Bounds())
	copy(copyImage.Pix, src.Pix)
	return copyImage
}

func clearGIFFrame(canvas *image.RGBA, frame *image.Paletted, animation *gif.GIF) {
	bounds := frame.Bounds().Intersect(canvas.Bounds())
	if bounds.Empty() {
		return
	}
	background := color.Color(color.Transparent)
	if transparencyIndex(frame) < 0 {
		if palette, ok := animation.Config.ColorModel.(color.Palette); ok &&
			int(animation.BackgroundIndex) < len(palette) {
			background = palette[animation.BackgroundIndex]
		}
	}
	draw.Draw(canvas, bounds, image.NewUniform(background), image.Point{}, draw.Src)
}

// drawOver compone un fotograma parcial sobre el lienzo respetando su paleta y
// el índice de transparencia.
func drawOver(dst *image.RGBA, src *image.Paletted) {
	b := src.Bounds()
	ti := transparencyIndex(src)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			idx := int(src.ColorIndexAt(x, y))
			if idx == ti {
				continue
			}
			dst.Set(x, y, src.At(x, y))
		}
	}
}

// transparencyIndex busca el índice transparente en la paleta del fotograma.
func transparencyIndex(src *image.Paletted) int {
	for i, c := range src.Palette {
		if _, _, _, a := c.RGBA(); a == 0 {
			return i
		}
	}
	return -1
}

// FrameAt devuelve el fotograma y su duración para un instante de tiempo.
func (g *GIF) FrameAt(t time.Duration) (image.Image, time.Duration) {
	frame, delay, _ := g.frameAt(t)
	return frame, delay
}

func (g *GIF) frameAt(t time.Duration) (image.Image, time.Duration, int) {
	if len(g.frames) == 0 {
		return nil, time.Second, -1
	}
	if g.total <= 0 {
		return g.frames[0], time.Second, 0
	}
	t %= g.total
	if len(g.frameEnd) == len(g.frames) {
		i := sort.Search(len(g.frameEnd), func(i int) bool { return t < g.frameEnd[i] })
		if i >= len(g.frames) {
			i = len(g.frames) - 1
		}
		return g.frames[i], g.delays[i], i
	}
	// Compatibilidad defensiva con un GIF construido sin la tabla acumulada.
	for i, delay := range g.delays {
		if t < delay {
			return g.frames[i], delay, i
		}
		t -= delay
	}
	i := len(g.frames) - 1
	return g.frames[i], g.delays[i], i
}

// Frame devuelve el fotograma i (con ajuste circular).
func (g *GIF) Frame(i int) image.Image {
	if len(g.frames) == 0 {
		return nil
	}
	return g.frames[((i%len(g.frames))+len(g.frames))%len(g.frames)]
}

// Count devuelve el número de fotogramas.
func (g *GIF) Count() int { return len(g.frames) }

// Duration devuelve la duración total del bucle.
func (g *GIF) Duration() time.Duration { return g.total }

// IsAnimated indica si el GIF tiene más de un fotograma.
func (g *GIF) IsAnimated() bool { return len(g.frames) > 1 }

// IsGIF indica si la ruta parece un GIF por su extensión.
func IsGIF(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".gif")
}

// Supported extiende la comprobación a los formatos que Go decodifica de serie.
func Supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp",
		".mp4", ".webm", ".mkv", ".mov":
		return true
	}
	return false
}
