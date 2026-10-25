package wallpaper

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	start    time.Time
	frame    int
	W, H     int
	last     *Canvas
	paused   bool
	pausedAt time.Time
}

// NewEngine prepara el motor para la especificación dada.
func NewEngine(spec Spec) (*Engine, error) {
	e := &Engine{spec: spec}
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
	e.start = time.Now()
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

// Frame produce el siguiente fotograma al tamaño del lienzo indicado. La capa
// Wayland escala cada fila al doble al mostrar el lienzo.
func (e *Engine) Frame(w, h int) *Canvas {
	return e.frameWithPixelAspect(w, h, 2)
}

func (e *Engine) frameWithPixelAspect(w, h int, pixelAspectY float64) *Canvas {
	e.frame++
	switch e.spec.Kind {
	case KindScene:
		c := e.scene.frameWithPixelAspect(w, h, pixelAspectY)
		e.last = c
		return c
	case KindImage:
		if e.last == nil || e.last.W != w || e.last.H != h {
			e.last = imageToCanvasWithBackground(e.still.Img, w, h/2, e.spec.Pal.Bg, false)
		}
		return e.last
	case KindGIF:
		img, _ := e.gif.FrameAt(time.Since(e.start))
		e.last = imageToCanvasWithBackground(img, w, h/2, e.spec.Pal.Bg, false)
		return e.last
	case KindVideo:
		frame := e.video.Frame(w, h)
		if frame == nil {
			e.last = NewCanvas(w, h, e.spec.Pal.Bg)
			return e.last
		}
		e.last = frame
		return e.last
	default:
		e.last = NewCanvas(w, h, e.spec.Pal.Bg)
		return e.last
	}
}

func (e *Engine) videoFrameBGRA(w, h int) []uint8 {
	if e.video == nil {
		return nil
	}
	e.last = nil
	return e.video.FrameBGRA(w, h)
}

func (e *Engine) videoSnapshot() *Canvas {
	if e.video == nil {
		return nil
	}
	return e.video.SnapshotVideo()
}

// PreviewError devuelve el último error de decodificación de vídeo, si existe.
func (e *Engine) PreviewError() error {
	if e == nil || e.video == nil || !e.video.errorLogged {
		return nil
	}
	return e.video.decodeErr
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

// Pause congela la animación sin perder su posición actual.
func (e *Engine) Pause() {
	if e == nil || e.paused {
		return
	}
	e.paused = true
	e.pausedAt = time.Now()
	if e.video != nil {
		e.video.Pause()
	}
}

// Resume continúa la animación desde el fotograma en el que se pausó.
func (e *Engine) Resume() {
	if e == nil || !e.paused {
		return
	}
	pausedFor := time.Since(e.pausedAt)
	if e.spec.Kind == KindGIF {
		e.start = e.start.Add(pausedFor)
	}
	if e.video != nil {
		e.video.Resume()
	}
	e.paused = false
	e.pausedAt = time.Time{}
}

// Close libera los recursos del motor.
func (e *Engine) Close() {
	if e.video != nil {
		e.video.Close()
	}
}

// ─── Vídeo (opcional con ffmpeg) ───────────────────────────────────────────

// videoSource decodifica vídeo en tiempo real: prueba VAAPI para decodificación
// y escalado, y conserva fallback a escalado o decodificación por software. La
// salida usa un canal de un fotograma para no acumular latencia ni memoria.
type videoSource struct {
	path            string
	fps             int
	duration        time.Duration
	vaapiDevice     string
	usingVAAPI      bool
	scaleWithVAAPI  bool
	vaapiLogged     bool
	forceSoftware   bool

	width  int
	height int
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr bytes.Buffer
	frames chan []uint8
	free   chan []uint8
	stop   chan struct{}
	done   chan struct{}

	current     []uint8
	decodeErr   error
	errorLogged bool
	paused      bool
}

// NewVideoSource valida el archivo y detecta un dispositivo VAAPI accesible.
// El proceso decodificador se inicia cuando el motor conoce la resolución real.
func NewVideoSource(path string, fps int) (*videoSource, error) {
	if fps <= 0 {
		fps = 24
	}
	if fps > 60 {
		fps = 60
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("se necesita ffmpeg para los fondos de vídeo")
	}
	info, err := probeVideo(path)
	if err != nil {
		return nil, err
	}
	return &videoSource{
		path: path, fps: fps, duration: info.duration,
		vaapiDevice:    findVAAPIDevice(),
		scaleWithVAAPI: true,
	}, nil
}

// findVAAPIDevice devuelve el primer nodo de render accesible. FFmpeg intentará
// usarlo para decodificar en la GPU y caerá a software si el driver no sirve.
func findVAAPIDevice() string {
	devices, _ := filepath.Glob("/dev/dri/renderD*")
	for _, device := range devices {
		f, err := os.OpenFile(device, os.O_RDWR, 0)
		if err == nil {
			_ = f.Close()
			return device
		}
	}
	return ""
}

// videoFilter crea el filtro de salida. En la ruta por software limita FPS
// antes de escalar; con VAAPI escala en la GPU antes de descargar los frames al
// formato NV12 y convertirlos a BGRA.
func videoFilter(width, height, fps int, useVAAPI, scaleWithVAAPI bool) string {
	if useVAAPI && scaleWithVAAPI {
		return fmt.Sprintf("scale_vaapi=w=%d:h=%d:format=nv12:mode=hq:force_original_aspect_ratio=increase:force_divisible_by=2,hwdownload,format=nv12,fps=%d,crop=%d:%d,format=bgra",
			width, height, fps, width, height)
	}
	if useVAAPI {
		return fmt.Sprintf("hwdownload,format=nv12,fps=%d,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,format=bgra",
			fps, width, height, width, height)
	}
	return fmt.Sprintf("fps=%d,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
		fps, width, height, width, height)
}

// Frame devuelve el fotograma más reciente como RGB para las transiciones.
func (v *videoSource) Frame(width, height int) *Canvas {
	return canvasFromBGRA(v.FrameBGRA(width, height), width, height)
}

// FrameBGRA devuelve el fotograma más reciente en el formato nativo de wl_shm.
func (v *videoSource) FrameBGRA(width, height int) []uint8 {
	if width <= 0 || height <= 0 {
		return nil
	}
	if err := v.startDecoder(width, height); err != nil {
		v.reportError(err)
		return nil
	}

	if v.current == nil {
		select {
		case frame, ok := <-v.frames:
			if !ok {
				if v.retryDecoder(width, height) {
					return nil
				}
				v.reportError(v.decodeErr)
				return nil
			}
			v.current = frame
		default:
			// No bloquea el bucle Wayland durante el arranque: el siguiente
			// ciclo tomará el primer fotograma en cuanto ffmpeg lo entregue.
			return nil
		}
	} else {
		select {
		case frame, ok := <-v.frames:
			if ok {
				previous := v.current
				v.current = frame
				select {
				case v.free <- previous:
				default:
				}
			} else {
				if v.retryDecoder(width, height) {
					return nil
				}
				v.reportError(v.decodeErr)
			}
		default:
		}
	}

	if len(v.current) != width*height*4 {
		return nil
	}
	if v.usingVAAPI && !v.vaapiLogged {
		stage := "decodificación"
		if v.scaleWithVAAPI {
			stage += " y escalado"
		}
		log.Printf("swaydesk: VAAPI activa (%s) para %q", stage, filepath.Base(v.path))
		v.vaapiLogged = true
	}
	return v.current
}

func (v *videoSource) retryDecoder(width, height int) bool {
	if !v.usingVAAPI {
		return false
	}
	cause := v.decodeErr
	if cause == nil {
		cause = fmt.Errorf("FFmpeg terminó antes de entregar el siguiente fotograma")
	}
	v.stopDecoder()
	if v.scaleWithVAAPI {
		v.scaleWithVAAPI = false
		log.Printf("swaydesk: escalado VAAPI no disponible para %q (%v); se reintenta con escalado por software", filepath.Base(v.path), cause)
	} else if !v.forceSoftware {
		v.forceSoftware = true
		log.Printf("swaydesk: VAAPI no pudo decodificar %q (%v); se prueba con software", filepath.Base(v.path), cause)
	} else {
		return false
	}
	if err := v.startDecoder(width, height); err != nil {
		v.reportError(err)
		return false
	}
	return true
}

// SnapshotVideo crea una copia RGB del fotograma actual, usada solo al iniciar
// una transición de workspace desde un vídeo.
func (v *videoSource) SnapshotVideo() *Canvas {
	return canvasFromBGRA(v.current, v.width, v.height)
}

func canvasFromBGRA(pix []uint8, width, height int) *Canvas {
	if width <= 0 || height <= 0 || len(pix) < width*height*4 {
		return nil
	}
	canvas := &Canvas{W: width, H: height, Pix: make([]uint8, width*height*3)}
	for i := 0; i < width*height; i++ {
		src, dst := i*4, i*3
		canvas.Pix[dst] = pix[src+2]
		canvas.Pix[dst+1] = pix[src+1]
		canvas.Pix[dst+2] = pix[src]
	}
	return canvas
}

func (v *videoSource) startDecoder(width, height int) error {
	if v.cmd != nil && v.width == width && v.height == height {
		return nil
	}
	if v.cmd != nil {
		v.stopDecoder()
	}
	v.current = nil
	v.decodeErr = nil
	v.stderr.Reset()

	useVAAPI := v.vaapiDevice != "" && !v.forceSoftware
	useVAAPIScale := useVAAPI && v.scaleWithVAAPI
	filter := videoFilter(width, height, v.fps, useVAAPI, useVAAPIScale)
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error"}
	if useVAAPI {
		args = append(args,
			"-hwaccel", "vaapi",
			"-hwaccel_device", v.vaapiDevice,
			"-hwaccel_output_format", "vaapi")
	}
	args = append(args,
		"-stream_loop", "-1", "-re", "-i", v.path,
		"-map", "0:v:0", "-an", "-sn", "-dn",
		"-vf", filter,
		"-pix_fmt", "bgra", "-f", "rawvideo", "pipe:1")
	cmd := exec.Command("ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("abriendo la salida de ffmpeg: %w", err)
	}
	cmd.Stderr = &v.stderr
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return fmt.Errorf("iniciando ffmpeg: %w", err)
	}
	v.errorLogged = false
	v.vaapiLogged = false
	v.cmd, v.stdout = cmd, stdout
	v.usingVAAPI = useVAAPI
	if useVAAPI {
		stage := "decodificación"
		if useVAAPIScale {
			stage += " y escalado"
		}
		log.Printf("swaydesk: probando VAAPI (%s) para %q en %s", stage, filepath.Base(v.path), v.vaapiDevice)
	}
	v.width, v.height = width, height
	v.frames = make(chan []uint8, 1)
	v.free = make(chan []uint8, 3)
	for i := 0; i < cap(v.free); i++ {
		v.free <- make([]uint8, width*height*4)
	}
	v.stop = make(chan struct{})
	v.done = make(chan struct{})
	go v.readFrames(width * height * 4)
	return nil
}

func (v *videoSource) readFrames(frameSize int) {
	defer close(v.done)
	defer close(v.frames)
	var readErr error
	defer func() {
		waitErr := v.cmd.Wait()
		select {
		case <-v.stop:
			return // cierre solicitado: no es un error de reproducción
		default:
		}
		if waitErr != nil {
			v.decodeErr = fmt.Errorf("ffmpeg: %v (%s)", waitErr, strings.TrimSpace(v.stderr.String()))
		} else if readErr != nil {
			v.decodeErr = fmt.Errorf("se interrumpió un fotograma de vídeo: %w", readErr)
		} else {
			v.decodeErr = fmt.Errorf("ffmpeg terminó la reproducción inesperadamente")
		}
	}()

	for {
		var frame []uint8
		select {
		case frame = <-v.free:
		case <-v.stop:
			return
		}
		if _, err := io.ReadFull(v.stdout, frame[:frameSize]); err != nil {
			readErr = err
			return
		}
		select {
		case <-v.stop:
			return
		default:
		}
		select {
		case v.frames <- frame:
		default:
			// Mantiene solo el fotograma más reciente; no deja que una salida
			// lenta acumule una cola creciente de vídeo atrasado.
			select {
			case dropped := <-v.frames:
				v.free <- dropped
			default:
			}
			select {
			case v.frames <- frame:
			case <-v.stop:
				return
			}
		}
	}
}

func (v *videoSource) reportError(err error) {
	if v.errorLogged {
		return
	}
	v.errorLogged = true
	if err == nil {
		err = fmt.Errorf("ffmpeg no entregó fotogramas")
	}
	v.decodeErr = err
	log.Printf("swaydesk: fondo de vídeo %q: %v", filepath.Base(v.path), err)
}

func (v *videoSource) Pause() {
	if v.paused || v.cmd == nil || v.cmd.Process == nil {
		return
	}
	if err := v.cmd.Process.Signal(syscall.SIGSTOP); err != nil {
		log.Printf("swaydesk: no se pudo pausar ffmpeg: %v", err)
		return
	}
	v.paused = true
}

func (v *videoSource) Resume() {
	if !v.paused {
		return
	}
	if v.cmd == nil || v.cmd.Process == nil {
		v.paused = false
		return
	}
	if err := v.cmd.Process.Signal(syscall.SIGCONT); err != nil {
		log.Printf("swaydesk: no se pudo reanudar ffmpeg: %v", err)
	}
	v.paused = false
}

func (v *videoSource) stopDecoder() {
	if v.done == nil {
		return
	}
	select {
	case <-v.stop:
	default:
		close(v.stop)
	}
	if v.stdout != nil {
		_ = v.stdout.Close()
	}
	if v.cmd != nil && v.cmd.Process != nil {
		_ = v.cmd.Process.Kill()
	}
	<-v.done
	v.cmd, v.stdout = nil, nil
	v.frames, v.free, v.stop, v.done = nil, nil, nil, nil
	v.width, v.height = 0, 0
	v.usingVAAPI = false
	v.paused = false
}

// Duration devuelve la duración del vídeo.
func (v *videoSource) Duration() time.Duration { return v.duration }

// Close detiene ffmpeg y libera el búfer del último fotograma.
func (v *videoSource) Close() {
	v.stopDecoder()
	v.current = nil
}

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
