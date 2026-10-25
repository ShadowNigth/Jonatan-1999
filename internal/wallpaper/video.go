package wallpaper

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// VideoSource video: decodificación con ffmpeg.
//
// El demonio intenta decodificar y escalar mediante VAAPI; usa escalado por
// software o decodificación por software como respaldo. Entrega fotogramas BGRA
// por una tubería acotada, sin cargar el clip entero.

type videoMetadata struct {
	duration time.Duration
	width    int
	height   int
}

func probeVideo(path string) (videoMetadata, error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return videoMetadata{}, fmt.Errorf("se necesita ffprobe (paquete ffmpeg) para fondos de vídeo")
	}
	out, err := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,duration:format=duration",
		"-of", "default=noprint_wrappers=1", path).Output()
	if err != nil {
		return videoMetadata{}, fmt.Errorf("ffprobe no pudo leer %s: %w", path, err)
	}

	var info videoMetadata
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "width":
			info.width, _ = strconv.Atoi(value)
		case "height":
			info.height, _ = strconv.Atoi(value)
		case "duration":
			if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds > 0 {
				info.duration = time.Duration(seconds * float64(time.Second))
			}
		}
	}
	if info.width < 1 || info.height < 1 {
		return videoMetadata{}, fmt.Errorf("ffprobe no pudo determinar el tamaño del vídeo %s", path)
	}
	if info.duration <= 0 {
		info.duration = 5 * time.Second
	}
	return info, nil
}

// ProbeVideo devuelve la duración y el ancho del vídeo usando ffprobe.
func ProbeVideo(path string) (time.Duration, int, error) {
	info, err := probeVideo(path)
	if err != nil {
		return 0, 0, err
	}
	return info.duration, info.width, nil
}

// scaledVideoHeight calcula la altura par que usa scale=ancho:-2 en ffmpeg.
func scaledVideoHeight(sourceWidth, sourceHeight, width int) int {
	if sourceWidth < 1 || sourceHeight < 1 || width < 1 {
		return 0
	}
	height := int(math.Round(float64(width) * float64(sourceHeight) / float64(sourceWidth) / 2)) * 2
	if height < 2 {
		height = 2
	}
	return height
}

// DecodeVideo decodifica hasta 30 segundos a `fps` fotogramas por segundo y
// a un ancho máximo de `width`. Se conserva como utilidad para consumidores
// puntuales; el demonio usa el decodificador en streaming de engine.go.
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
	info, err := probeVideo(path)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	height := scaledVideoHeight(info.width, info.height, width)
	args := []string{
		"-v", "error",
		"-i", path,
		"-t", "30",
		"-an", "-sn", "-dn",
		"-vf", fmt.Sprintf("fps=%d,scale=%d:%d", fps, width, height),
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
	if len(out) == 0 {
		return nil, 0, 0, 0, fmt.Errorf("el vídeo no contiene fotogramas")
	}
	frameSize := width * height * 3
	if frameSize <= 0 || len(out)%frameSize != 0 {
		return nil, 0, 0, 0, fmt.Errorf("ffmpeg devolvió %d bytes para fotogramas de %dx%d", len(out), width, height)
	}
	count := len(out) / frameSize
	frames := make([][]uint8, 0, count)
	for i := 0; i < count; i++ {
		frame := make([]uint8, frameSize)
		copy(frame, out[i*frameSize:(i+1)*frameSize])
		frames = append(frames, frame)
	}
	duration := time.Duration(count) * time.Second / time.Duration(fps)
	return frames, width, height, duration, nil
}
