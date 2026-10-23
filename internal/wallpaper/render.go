package wallpaper

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Renderer de imágenes: convierte fotogramas RGB (escenas o GIF) en secuencias
// compatibles con cualquier terminal.

// ANSIRenderer pinta un fotograma usando medios bloques con color de 24 bits.
// Funciona en cualquier terminal: no necesita soporte de imágenes.
type ANSIRenderer struct {
	// Downscale reduce la resolución para acelerar el dibujado en terminales
	// lentos (0 = sin escalado).
	Downscale int
	buf       bytes.Buffer
}

// RenderCanvas convierte un Canvas en un fotograma ANSI.
func (a *ANSIRenderer) RenderCanvas(c *Canvas) string {
	a.buf.Reset()
	for row := 0; row < c.H/2; row++ {
		for col := 0; col < c.W; col++ {
			i1 := (row*2*c.W + col) * 3
			i2 := ((row*2+1)*c.W + col) * 3
			fmt.Fprintf(&a.buf, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				c.Pix[i1], c.Pix[i1+1], c.Pix[i1+2],
				c.Pix[i2], c.Pix[i2+1], c.Pix[i2+2])
		}
		a.buf.WriteString("\x1b[0m\n")
	}
	return a.buf.String()
}

// RenderImage convierte una imagen en un fotograma ANSI con medios bloques.
func (a *ANSIRenderer) RenderImage(img image.Image, w, h int) string {
	if img == nil {
		return ""
	}
	canvas := ImageToCanvas(img, w, h)
	return a.RenderCanvas(canvas)
}

// ImageToCanvas convierte una imagen a un Canvas del tamaño en celdas indicado
// (h son filas de caracteres; el lienzo tendrá 2·h píxeles de alto).
func ImageToCanvas(img image.Image, w, h int) *Canvas {
	if img == nil || w <= 0 || h <= 0 {
		return &Canvas{W: 1, H: 1, Pix: make([]uint8, 3)}
	}
	c := &Canvas{W: w, H: h * 2, Pix: make([]uint8, w*h*2*3)}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	// La imagen se recorta al centro para respetar la proporción del área.
	targetRatio := float64(w) / float64(h) * 0.5
	srcRatio := float64(sw) / float64(sh)
	cropW, cropH := float64(sw), float64(sh)
	if srcRatio > targetRatio {
		cropW = float64(sh) * targetRatio
	} else {
		cropH = float64(sw) / targetRatio
	}
	offX := (float64(sw) - cropW) / 2
	offY := (float64(sh) - cropH) / 2
	for y := 0; y < c.H; y++ {
		for x := 0; x < w; x++ {
			sx := b.Min.X + int(offX+(float64(x)+0.5)*cropW/float64(w))
			sy := b.Min.Y + int(offY+(float64(y)+0.5)*cropH/float64(c.H))
			if sx >= b.Max.X {
				sx = b.Max.X - 1
			}
			if sy >= b.Max.Y {
				sy = b.Max.Y - 1
			}
			r, g, bl, _ := img.At(sx, sy).RGBA()
			i := (y*w + x) * 3
			c.Pix[i] = uint8(r >> 8)
			c.Pix[i+1] = uint8(g >> 8)
			c.Pix[i+2] = uint8(bl >> 8)
		}
	}
	return c
}

// ─── Protocolos de imágenes del terminal ────────────────────────────────────

// ImageProtocol es el protocolo de imágenes soportado por el terminal.
type ImageProtocol int

const (
	// ProtoNone solo permite medios bloques.
	ProtoNone ImageProtocol = iota
	// ProtoKitty usa el protocolo gráfico de kitty (también lo entienden
	// Ghostty y, parcialmente, WezTerm).
	ProtoKitty
	// ProtoSixel envía imágenes codificadas en sixel (foot, wezterm, xterm).
	ProtoSixel
)

// DetectProtocol deduce el protocolo disponible a partir del entorno.
func DetectProtocol() ImageProtocol {
	switch os.Getenv("SWAYDESK_IMAGE_PROTOCOL") {
	case "kitty":
		return ProtoKitty
	case "sixel":
		return ProtoSixel
	case "none", "ansi":
		return ProtoNone
	}
	term := lowerEnv("TERM")
	termProgram := lowerEnv("TERM_PROGRAM")
	if os.Getenv("KITTY_WINDOW_ID") != "" || contains(term, "kitty") ||
		contains(termProgram, "ghostty") || contains(termProgram, "wezterm") ||
		os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return ProtoKitty
	}
	if contains(term, "foot") || contains(term, "sixel") || contains(term, "wezterm") {
		return ProtoSixel
	}
	return ProtoNone
}

func lowerEnv(k string) string {
	v := os.Getenv(k)
	out := make([]byte, len(v))
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

func contains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// KitKat escribe una imagen usando el protocolo de kitty.
//
// La imagen se envía en PNG por la misma salida del terminal. Con placeless se
// permite que el terminal la posicione en el cursor; aquí usamos posicionamiento
// absoluto por filas/columnas para poder centrarla.
func KittySequence(img image.Image, cols, rows, offsetX, offsetY int) (string, error) {
	data, err := encodePNG(img)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	// Posiciona el cursor y transmite la imagen. a=T (transmitir y mostrar),
	// f=100 (PNG), C=1 (no mover el cursor), q=2 (silencioso), z=-1 (detrás del
	// texto), c/r = tamaño en celdas.
	fmt.Fprintf(&b, "\x1b[%d;%dH", offsetY+1, offsetX+1)
	enc := base64Encode(data)
	const chunk = 4096
	first := true
	for i := 0; i < len(enc); i += chunk {
		end := i + chunk
		if end > len(enc) {
			end = len(enc)
		}
		more := 1
		if end == len(enc) {
			more = 0
		}
		if first {
			fmt.Fprintf(&b, "\x1b_Ga=T,f=100,C=1,q=2,z=-1,c=%d,r=%d,m=%d;", cols, rows, more)
			first = false
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;", more)
		}
		b.WriteString(enc[i:end])
		b.WriteString("\x1b\\")
	}
	return b.String(), nil
}

// SixelSequence convierte la imagen a sixel. Es una implementación propia del
// codificador: paleta de 256 colores y bandas de 6 píxeles.
func SixelSequence(img image.Image, cols, rows int) (string, error) {
	if img == nil {
		return "", fmt.Errorf("imagen vacía")
	}
	c := ImageToCanvas(img, cols, rows)
	return EncodeSixel(c.Pix, c.W, c.H)
}

// EncodeSixel codifica un búfer RGB (3 bytes por píxel) como sixel.
func EncodeSixel(pix []uint8, w, h int) (string, error) {
	if w <= 0 || h <= 0 || len(pix) < w*h*3 {
		return "", fmt.Errorf("búfer sixel inválido")
	}
	// Cuantización en 6 niveles por canal (hasta 216 colores).
	pal := make([]color.RGBA, 0, 216)
	index := map[[3]uint8]int{}
	grid := make([]int, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			q := [3]uint8{quant(pix[i]), quant(pix[i+1]), quant(pix[i+2])}
			idx, ok := index[q]
			if !ok {
				idx = len(pal)
				index[q] = idx
				pal = append(pal, color.RGBA{q[0], q[1], q[2], 255})
			}
			grid[y*w+x] = idx
		}
	}
	var b bytes.Buffer
	b.WriteString("\x1bP0;1;0q")
	fmt.Fprintf(&b, "\"1;1;%d;%d", w, h)
	for i, c := range pal {
		fmt.Fprintf(&b, "#%d;2;%d;%d;%d", i, c.R, c.G, c.B)
	}
	for band := 0; band*6 < h; band++ {
		// Para cada color presente en la banda se emite su plano de bits.
		for ci := range pal {
			used := false
			var bits bytes.Buffer
			for x := 0; x < w; x++ {
				var bitsByte byte
				for bit := 0; bit < 6; bit++ {
					y := band*6 + bit
					if y < h && grid[y*w+x] == ci {
						bitsByte |= 1 << bit
						used = true
					}
				}
				bits.WriteByte(bitsByte + 63)
			}
			if used {
				fmt.Fprintf(&b, "#%d", ci)
				b.Write(bits.Bytes())
				b.WriteString("$")
			}
		}
		b.WriteString("-")
	}
	b.WriteString("\x1b\\")
	return b.String(), nil
}

func quant(v uint8) uint8 {
	// 6 niveles: 0, 51, 102, 153, 204, 255
	n := int(v) / 51
	if n > 5 {
		n = 5
	}
	return uint8(n * 51)
}

// ─── PNG / archivos ─────────────────────────────────────────────────────────

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func base64Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b bytes.Buffer
	b.Grow((len(data) + 2) / 3 * 4)
	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i
		n = uint32(data[i]) << 16
		if remaining > 1 {
			n |= uint32(data[i+1]) << 8
		}
		if remaining > 2 {
			n |= uint32(data[i+2])
		}
		b.WriteByte(alphabet[(n>>18)&63])
		b.WriteByte(alphabet[(n>>12)&63])
		if remaining > 1 {
			b.WriteByte(alphabet[(n>>6)&63])
		} else {
			b.WriteByte('=')
		}
		if remaining > 2 {
			b.WriteByte(alphabet[n&63])
		} else {
			b.WriteByte('=')
		}
	}
	return b.String()
}

// SaveCanvas guarda un lienzo como PNG (para fondos estáticos o capturas).
func SaveCanvas(c *Canvas, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			i := (y*c.W + x) * 3
			img.Set(x, y, color.RGBA{c.Pix[i], c.Pix[i+1], c.Pix[i+2], 255})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// SaveImage guarda una imagen arbitraria como PNG (fotogramas de GIF o vídeo
// que se usan de respaldo estático para sway).
func SaveImage(img image.Image, path string) error {
	if img == nil {
		return fmt.Errorf("imagen vacía")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// CanvasToImage convierte un lienzo en image.Image.
func CanvasToImage(c *Canvas) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	copy(img.Pix, c.Pix)
	return img
}

// ─── Utilidades de vídeo y Wallpaper Engine ─────────────────────────────────

// VideoFrame extrae un fotograma de un vídeo usando ffmpeg, si está instalado.
// Los archivos de Wallpaper Engine suelen ser .mp4 o .webm en formato VP9.
func VideoFrame(path string, at time.Duration, width int) (image.Image, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("se necesita ffmpeg para los fondos de vídeo")
	}
	args := []string{
		"-v", "error", "-ss", fmt.Sprintf("%.3f", at.Seconds()),
		"-i", path, "-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:-1", width),
		"-f", "image2pipe", "-vcodec", "png", "-",
	}
	out, err := exec.Command("ffmpeg", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decodificando el fotograma: %w", err)
	}
	return img, nil
}

// VideoInfo comprueba que un archivo pueda usarse como fondo animado.
func VideoInfo(path string) (duration time.Duration, frames int, err error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0, 0, fmt.Errorf("se necesita ffprobe para inspeccionar el vídeo")
	}
	out, err := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=duration,nb_frames",
		"-of", "default=noprint_wrappers=1", path).Output()
	if err != nil {
		return 0, 0, err
	}
	var dur float64
	for _, line := range splitLines(string(out)) {
		if hasPrefix(line, "duration=") {
			fmt.Sscanf(trimPrefix(line, "duration="), "%f", &dur)
		}
		if hasPrefix(line, "nb_frames=") {
			fmt.Sscanf(trimPrefix(line, "nb_frames="), "%d", &frames)
		}
	}
	return time.Duration(dur * float64(time.Second)), frames, nil
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func trimPrefix(s, p string) string {
	if hasPrefix(s, p) {
		return s[len(p):]
	}
	return s
}

// CoverCanvas adapta un Canvas a un tamaño de celdas recortando al centro, con
// la misma lógica que ImageToCanvas pero sin volver a decodificar.
func CoverCanvas(src *Canvas, w, h int) *Canvas {
	if src == nil || w <= 0 || h <= 0 {
		return NewCanvas(w, h, "#000000")
	}
	return &Canvas{W: w, H: h, Pix: scaleRGB(src.Pix, src.W, src.H, w, h)}
}

// scaleRGB reescala un búfer RGB con muestreo bilineal.
func scaleRGB(src []uint8, sw, sh, dw, dh int) []uint8 {
	if sw == dw && sh == dh {
		cp := make([]uint8, len(src))
		copy(cp, src)
		return cp
	}
	out := make([]uint8, dw*dh*3)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx := int(float64(x) * float64(sw) / float64(dw))
			sy := int(float64(y) * float64(sh) / float64(dh))
			if sx >= sw {
				sx = sw - 1
			}
			if sy >= sh {
				sy = sh - 1
			}
			si := (sy*sw + sx) * 3
			di := (y*dw + x) * 3
			out[di] = src[si]
			out[di+1] = src[si+1]
			out[di+2] = src[si+2]
		}
	}
	return out
}
