package wallpaper

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Kind identifica el tipo de fondo configurado.
type Kind string

// Tipos de fondo soportados.
const (
	KindScene Kind = "scene"
	KindImage Kind = "image"
	KindGIF   Kind = "gif"
	KindVideo Kind = "video"
	KindNone  Kind = "none"
)

// Spec describe el fondo de pantalla activo.
type Spec struct {
	Kind  Kind
	Path  string
	Scene string
	FPS   int
	Pal   ui.Palette
}

// ParseSpec normaliza una descripción textual del tipo "scene:petals",
// "gif:/ruta.gif", "image:/foto.png" o una ruta suelta.
func ParseSpec(s string, pal ui.Palette) (Spec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Spec{}, fmt.Errorf("no se indicó ningún fondo")
	}
	spec := Spec{FPS: 24, Pal: pal}
	switch {
	case strings.HasPrefix(s, "scene:"):
		name := strings.TrimPrefix(s, "scene:")
		if _, ok := SceneByKey(name); !ok {
			// Comodidad: también se acepta el nombre de una paleta y se usa la
			// escena que esa paleta tiene asociada (scene:sakura → petals).
			if ui.IsPalette(name) {
				if p := ui.PaletteByName(name); p.Scene != "" {
					spec.Kind, spec.Scene = KindScene, p.Scene
					break
				}
			}
			return Spec{}, fmt.Errorf("escena desconocida: %s", name)
		}
		spec.Kind, spec.Scene = KindScene, name
	case strings.HasPrefix(s, "gif:"):
		spec.Kind, spec.Path = KindGIF, strings.TrimPrefix(s, "gif:")
	case strings.HasPrefix(s, "image:"):
		spec.Kind, spec.Path = KindImage, strings.TrimPrefix(s, "image:")
	case strings.HasPrefix(s, "video:"):
		spec.Kind, spec.Path = KindVideo, strings.TrimPrefix(s, "video:")
	case strings.HasPrefix(s, "none"):
		spec.Kind = KindNone
	default:
		// Ruta o nombre de escena suelto.
		if _, ok := SceneByKey(s); ok {
			spec.Kind, spec.Scene = KindScene, s
			break
		}
		if ui.IsPalette(s) && !strings.Contains(s, "/") {
			if p := ui.PaletteByName(s); p.Scene != "" {
				spec.Kind, spec.Scene = KindScene, p.Scene
				break
			}
		}
		ext := strings.ToLower(filepath.Ext(s))
		switch ext {
		case ".gif":
			spec.Kind, spec.Path = KindGIF, s
		case ".png", ".jpg", ".jpeg", ".webp", ".bmp":
			spec.Kind, spec.Path = KindImage, s
		case ".mp4", ".webm", ".mkv", ".mov":
			spec.Kind, spec.Path = KindVideo, s
		default:
			return Spec{}, fmt.Errorf("no se reconoce el fondo %q (usa scene:<nombre> o una ruta de imagen/gif)", s)
		}
	}
	return spec, nil
}

// Engine produce fotogramas RGB del fondo activo a cualquier resolución.
type Engine struct {
	spec  Spec
	scene *Renderer
	still *Still
	gif   *GIF
	video *videoSource
	start time.Time
	frame int
	W, H  int
	last  *Canvas
}

// NewEngine prepara el motor para la especificación dada.
func NewEngine(spec Spec) (*Engine, error) {
	e := &Engine{spec: spec, start: time.Now()}
	switch spec.Kind {
	case KindScene:
		e.scene = NewRenderer(spec.Scene, spec.Pal)
	case KindImage:
		s, err := LoadStill(spec.Path)
		if err != nil {
			return nil, err
		}
		e.still = s
	case KindGIF:
		g, err := LoadGIF(spec.Path)
		if err != nil {
			return nil, err
		}
		e.gif = g
	case KindVideo:
		v, err := NewVideoSource(spec.Path, spec.FPS)
		if err != nil {
			return nil, err
		}
		e.video = v
	case KindNone:
		// nada que cargar
	default:
		return nil, fmt.Errorf("tipo de fondo no soportado: %s", spec.Kind)
	}
	return e, nil
}

// Kind devuelve el tipo de fondo.
func (e *Engine) Kind() Kind { return e.spec.Kind }

// Spec devuelve la especificación usada.
func (e *Engine) Spec() Spec { return e.spec }

// Interval devuelve el tiempo entre fotogramas según los FPS configurados.
func (e *Engine) Interval() time.Duration {
	fps := e.spec.FPS
	if fps <= 0 {
		fps = 24
	}
	if fps > 60 {
		fps = 60
	}
	return time.Second / time.Duration(fps)
}

// Size devuelve el tamaño deseado del lienzo según el tipo de fondo.
//
// Las escenas se dibujan a la resolución pedida (en "píxeles" del lienzo, es
// decir, 2·filas de terminal) o a la mitad para ahorrar CPU en pantallas
// grandes; las imágenes conservan su proporción.
func (e *Engine) Size(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		w, h = 80, 24
	}
	switch e.spec.Kind {
	case KindImage:
		if e.still != nil {
			return ui.FitSize(e.still.W, e.still.H, w, h*2, 2)
		}
	case KindGIF:
		if e.gif != nil {
			return ui.FitSize(e.gif.W, e.gif.H, w, h*2, 2)
		}
	}
	return w, h * 2
}

// Frame produce el fotograma correspondiente al instante t.
func (e *Engine) Frame(w, h int) *Canvas {
	e.frame++
	switch e.spec.Kind {
	case KindScene:
		c := e.scene.Frame(w, h)
		e.last = c
		return c
	case KindImage:
		if e.last == nil || e.last.W != w || e.last.H != h {
			e.last = ImageToCanvas(e.still.Img, w/1, h/2)
		}
		return e.last
	case KindGIF:
		img, _ := e.gif.FrameAt(time.Since(e.start))
		e.last = ImageToCanvas(img, w, h/2)
		return e.last
	case KindVideo:
		img := e.video.FrameAt(time.Since(e.start))
		if img == nil {
			e.last = NewCanvas(w, h, e.spec.Pal.Bg)
			return e.last
		}
		e.last = ImageToCanvas(img, w, h/2)
		return e.last
	default:
		e.last = NewCanvas(w, h, e.spec.Pal.Bg)
		return e.last
	}
}

// Duration devuelve la duración del bucle (0 si es infinito o procedural).
func (e *Engine) Duration() time.Duration {
	switch {
	case e.gif != nil:
		return e.gif.Duration()
	case e.video != nil:
		return e.video.Duration()
	}
	return 0
}

// Describe devuelve una descripción legible del fondo activo.
func (e *Engine) Describe() string {
	switch e.spec.Kind {
	case KindScene:
		if info, ok := SceneByKey(e.spec.Scene); ok {
			return info.Name
		}
		return e.spec.Scene
	case KindImage, KindGIF, KindVideo:
		return filepath.Base(e.spec.Path)
	}
	return "sin fondo"
}

// SizeBytes devuelve el tamaño en memoria de los fotogramas cargados.
func (e *Engine) SizeBytes() int64 {
	var n int64
	if e.gif != nil {
		n += int64(e.gif.W * e.gif.H * 4 * len(e.gif.frames))
	}
	if e.still != nil {
		n += int64(e.still.W * e.still.H * 4)
	}
	return n
}

// Close libera los recursos del motor.
func (e *Engine) Close() {
	if e.video != nil {
		e.video.Close()
	}
}

// ─── Vídeo (opcional con ffmpeg) ───────────────────────────────────────────

// videoSource decodifica fotogramas de vídeo bajo demanda usando ffmpeg.
//
// Para evitar lanzar un proceso por fotograma, se mantiene una tubería abierta
// con fotogramas RGB en bruto a resolución reducida.
type videoSource struct {
	path     string
	fps      int
	frames   [][]uint8
	width    int
	height   int
	duration time.Duration
	loaded   bool
}

// NewVideoSource prepara la fuente de vídeo (la descodificación real es perezosa).
func NewVideoSource(path string, fps int) (*videoSource, error) {
	if fps <= 0 {
		fps = 15
	}
	v := &videoSource{path: path, fps: fps}
	// Comprobación temprana para dar un error claro.
	if _, _, err := ProbeVideo(path); err != nil {
		return nil, err
	}
	if err := v.load(); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *videoSource) load() error {
	frames, w, h, dur, err := DecodeVideo(v.path, 160, v.fps)
	if err != nil {
		return err
	}
	v.frames, v.width, v.height, v.duration, v.loaded = frames, w, h, dur, true
	return nil
}

// FrameAt devuelve el fotograma más cercano al instante t.
func (v *videoSource) FrameAt(t time.Duration) image.Image {
	if !v.loaded || len(v.frames) == 0 {
		return nil
	}
	dur := v.duration
	if dur <= 0 {
		dur = time.Duration(len(v.frames)) * time.Second / time.Duration(v.fps)
	}
	if dur > 0 {
		t = t % dur
	}
	idx := int(float64(t) / float64(dur) * float64(len(v.frames)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(v.frames) {
		idx = len(v.frames) - 1
	}
	return RGBAFromRGB(v.frames[idx], v.width, v.height)
}

// Duration devuelve la duración del vídeo.
func (v *videoSource) Duration() time.Duration { return v.duration }

// Close libera los fotogramas en memoria.
func (v *videoSource) Close() { v.frames = nil }

// RGBAFromRGB envuelve un búfer RGB en una imagen.
func RGBAFromRGB(pix []uint8, w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h && i*3+2 < len(pix); i++ {
		img.Pix[i*4] = pix[i*3]
		img.Pix[i*4+1] = pix[i*3+1]
		img.Pix[i*4+2] = pix[i*3+2]
		img.Pix[i*4+3] = 255
	}
	return img
}
