package wallpaper

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// VideoSource video: decodificación con ffmpeg.
//
// Un fondo de vídeo (incluidos los paquetes de Wallpaper Engine, que suelen ser
// .mp4/.webm) se decodifica una vez a baja resolución y se mantiene en memoria.
// Es suficiente para un fondo animado y evita lanzar procesos en cada fotograma.

// ProbeVideo devuelve la duración y el tamaño del vídeo usando ffprobe.
func ProbeVideo(path string) (time.Duration, int, error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0, 0, fmt.Errorf("se necesita ffprobe (paquete ffmpeg) para fondos de vídeo")
	}
	out, err := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,duration",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1", path).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe no pudo leer %s", path)
	}
	var dur float64
	var w int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "duration="); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				dur = f
			}
		}
		if v, ok := strings.CutPrefix(line, "width="); ok {
			w, _ = strconv.Atoi(v)
		}
	}
	if dur == 0 {
		dur = 5
	}
	return time.Duration(dur * float64(time.Second)), w, nil
}

// DecodeVideo decodifica el vídeo a `fps` fotogramas por segundo con un ancho
// máximo de `width` píxeles. Devuelve los fotogramas en formato RGB (3 bytes
// por píxel).
func DecodeVideo(path string, width, fps int) ([][]uint8, int, int, time.Duration, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("se necesita ffmpeg para los fondos de vídeo")
	}
	if width <= 0 {
		width = 160
	}
	if fps <= 0 {
		fps = 15
	}
	// Se limita la duración a 30 s para no consumir memoria sin control.
	args := []string{
		"-v", "error",
		"-t", "30",
		"-i", path,
		"-vf", fmt.Sprintf("fps=%d,scale=%d:-2", fps, width),
		"-pix_fmt", "rgb24",
		"-f", "rawvideo", "-",
	}
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("ffmpeg: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	dur, srcW, err := ProbeVideo(path)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	_ = srcW
	// El alto se deduce del tamaño total: ancho fijo, alto múltiplo par.
	if width == 0 || len(out) == 0 {
		return nil, 0, 0, 0, fmt.Errorf("el vídeo no contiene fotogramas")
	}
	height := 0
	// Se busca un alto que haga que el total sea múltiplo de w*h*3 probando
	// divisores razonables (el escalado -2 garantiza par).
	for _, h := range candidateHeights(len(out), width) {
		if len(out)%(width*h*3) == 0 {
			height = h
			break
		}
	}
	if height == 0 {
		return nil, 0, 0, 0, fmt.Errorf("no se pudo determinar el tamaño de los fotogramas")
	}
	frameSize := width * height * 3
	count := len(out) / frameSize
	frames := make([][]uint8, 0, count)
	for i := 0; i < count; i++ {
		f := make([]uint8, frameSize)
		copy(f, out[i*frameSize:(i+1)*frameSize])
		frames = append(frames, f)
	}
	if len(frames) > 0 {
		dur = time.Duration(len(frames)) * time.Second / time.Duration(fps)
	}
	return frames, width, height, dur, nil
}

// candidateHeights propone alturas probables para el búfer de vídeo.
func candidateHeights(total, width int) []int {
	var out []int
	for h := 32; h <= 2000; h += 2 {
		if total%(width*h*3) == 0 {
			out = append(out, h)
			if len(out) >= 4 {
				break
			}
		}
	}
	return out
}
