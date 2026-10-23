package notify

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// Límites de las imágenes de las notificaciones. Una tarjeta mide unos 360 px
// de ancho: por encima de 512 px de lado no se gana nada y se puede llenar la
// memoria con una foto de 40 megapíxeles.
const (
	// MaxImageSide es el lado máximo, en píxeles, que se conserva de una imagen.
	MaxImageSide = 512
	// MaxImageBytes es el tamaño máximo de archivo que se acepta leer.
	MaxImageBytes = 8 << 20
	// MaxImageHint es el tamaño máximo del hint `image-data` (bytes crudos).
	MaxImageHint = 12 << 20
)

// Image es una imagen RGBA de 8 bits lista para dibujar en una tarjeta.
//
// Se guarda con 4 bytes por píxel (rojo, verde, azul, alfa) en orden de filas;
// es el mismo formato que la memoria compartida del panel, así que dibujarla es
// copiar píxeles.
type Image struct {
	W, H int
	Pix  []uint8
}

// Empty indica si la imagen no tiene contenido.
func (im *Image) Empty() bool { return im == nil || im.W <= 0 || im.H <= 0 }

// At devuelve el color de un píxel. Fuera de límites devuelve negro
// transparente.
func (im *Image) At(x, y int) (r, g, b, a uint8) {
	if im.Empty() || x < 0 || y < 0 || x >= im.W || y >= im.H {
		return 0, 0, 0, 0
	}
	i := (y*im.W + x) * 4
	return im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]
}

// DecodeImage decodifica PNG o JPEG. Es lo que usan tanto el hint
// `image-data` (que llega como PNG) como los archivos de `image-path`.
func DecodeImage(data []byte) (*Image, error) {
	if len(data) == 0 {
		return nil, errors.New("imagen vacía")
	}
	if len(data) > MaxImageHint {
		return nil, fmt.Errorf("imagen demasiado grande (%d bytes)", len(data))
	}
	var (
		src image.Image
		err error
	)
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		src, err = png.Decode(bytes.NewReader(data))
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8}):
		src, err = jpeg.Decode(bytes.NewReader(data))
	default:
		// SVG, WebP, GIF…: se intenta con el decodificador genérico (que solo
		// conoce los formatos registrados) y si no, se avisa.
		src, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("imagen ilegible: %w", err)
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, errors.New("imagen sin píxeles")
	}
	im := &Image{W: b.Dx(), H: b.Dy(), Pix: make([]uint8, b.Dx()*b.Dy()*4)}
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			r, g, bb, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := (y*im.W + x) * 4
			im.Pix[i] = uint8(r >> 8)
			im.Pix[i+1] = uint8(g >> 8)
			im.Pix[i+2] = uint8(bb >> 8)
			im.Pix[i+3] = uint8(a >> 8)
		}
	}
	return im.Fit(MaxImageSide, MaxImageSide), nil
}

// LoadImageFile lee y decodifica una imagen del disco. La ruta puede venir con
// `file://` delante (algunos clientes lo hacen).
func LoadImageFile(path string) (*Image, error) {
	path = strings.TrimPrefix(strings.TrimSpace(path), "file://")
	if path == "" {
		return nil, errors.New("ruta de imagen vacía")
	}
	path = expandHome(path)
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%s es un directorio", path)
	}
	if st.Size() > MaxImageBytes {
		return nil, fmt.Errorf("imagen demasiado grande (%d bytes)", st.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodeImage(data)
}

// expandHome sustituye un ~ inicial por el directorio del usuario.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

// DecodeImageData interpreta los bytes del hint `image-data` de la
// especificación de notificaciones, cuya firma es (iiibiiay):
//
//	ancho, alto, salto de fila, ¿alfa?, bits por canal, canales, datos
//
// Los datos vienen en RGBA (o RGB si no hay alfa) y las filas pueden llevar
// relleno al final, de ahí el salto de fila.
func DecodeImageData(width, height, rowstride int, hasAlpha bool, bits, channels int, data []byte) (*Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("tamaño de imagen inválido: %dx%d", width, height)
	}
	if width > 4096 || height > 4096 {
		return nil, fmt.Errorf("imagen demasiado grande: %dx%d", width, height)
	}
	if bits != 8 {
		return nil, fmt.Errorf("solo se admiten 8 bits por canal (llegaron %d)", bits)
	}
	if channels != 3 && channels != 4 {
		return nil, fmt.Errorf("número de canales no soportado: %d", channels)
	}
	if rowstride <= 0 {
		rowstride = width * channels
	}
	need := rowstride * (height - 1)
	need += width * channels
	if need > len(data) || len(data) > MaxImageHint {
		return nil, fmt.Errorf("datos incompletos: %d bytes para %dx%d", len(data), width, height)
	}
	im := &Image{W: width, H: height, Pix: make([]uint8, width*height*4)}
	for y := 0; y < height; y++ {
		row := data[y*rowstride:]
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			s := x * channels
			im.Pix[i] = row[s]
			im.Pix[i+1] = row[s+1]
			im.Pix[i+2] = row[s+2]
			if channels == 4 {
				im.Pix[i+3] = row[s+3]
			} else if hasAlpha {
				im.Pix[i+3] = 255
			} else {
				im.Pix[i+3] = 255
			}
		}
	}
	return im.Fit(MaxImageSide, MaxImageSide), nil
}

// Fit devuelve una copia escalada para que quepa en maxW×maxH conservando la
// proporción. Si ya cabe, devuelve la misma imagen.
func (im *Image) Fit(maxW, maxH int) *Image {
	if im.Empty() || maxW <= 0 || maxH <= 0 {
		return im
	}
	if im.W <= maxW && im.H <= maxH {
		return im
	}
	scale := float64(maxW) / float64(im.W)
	if s := float64(maxH) / float64(im.H); s < scale {
		scale = s
	}
	w := int(float64(im.W)*scale + 0.5)
	h := int(float64(im.H)*scale + 0.5)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return im.Resize(w, h)
}

// Resize escala a un tamaño exacto promediando cada bloque de origen (filtro
// de caja). Para miniaturas da mejor resultado que copiar el píxel más
// cercano y no necesita dependencias.
func (im *Image) Resize(w, h int) *Image {
	if im.Empty() || w <= 0 || h <= 0 {
		return &Image{W: 1, H: 1, Pix: make([]uint8, 4)}
	}
	if w == im.W && h == im.H {
		return im
	}
	out := &Image{W: w, H: h, Pix: make([]uint8, w*h*4)}
	for y := 0; y < h; y++ {
		y0 := y * im.H / h
		y1 := (y + 1) * im.H / h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0 := x * im.W / w
			x1 := (x + 1) * im.W / w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sr, sg, sb, sa, n uint64
			for yy := y0; yy < y1 && yy < im.H; yy++ {
				for xx := x0; xx < x1 && xx < im.W; xx++ {
					i := (yy*im.W + xx) * 4
					sr += uint64(im.Pix[i])
					sg += uint64(im.Pix[i+1])
					sb += uint64(im.Pix[i+2])
					sa += uint64(im.Pix[i+3])
					n++
				}
			}
			if n == 0 {
				continue
			}
			i := (y*w + x) * 4
			out.Pix[i] = uint8(sr / n)
			out.Pix[i+1] = uint8(sg / n)
			out.Pix[i+2] = uint8(sb / n)
			out.Pix[i+3] = uint8(sa / n)
		}
	}
	return out
}

// CropSquare recorta la imagen por el centro a un cuadrado de lado side, que
// es como se dibujan las miniaturas de las tarjetas.
func (im *Image) CropSquare(side int) *Image {
	if im.Empty() || side <= 0 {
		return im
	}
	if im.W == im.H && im.W == side {
		return im
	}
	cx := im.W / 2
	cy := im.H / 2
	x0 := cx - side/2
	y0 := cy - side/2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x0+side > im.W {
		x0 = im.W - side
	}
	if y0+side > im.H {
		y0 = im.H - side
	}
	if x0 < 0 || y0 < 0 {
		return im
	}
	out := &Image{W: side, H: side, Pix: make([]uint8, side*side*4)}
	for y := 0; y < side; y++ {
		src := ((y0+y)*im.W + x0) * 4
		copy(out.Pix[y*side*4:(y+1)*side*4], im.Pix[src:src+side*4])
	}
	return out
}

// Thumbnail prepara la imagen para una tarjeta: cuadrada (recortada por el
// centro) y del tamaño pedido.
func (im *Image) Thumbnail(side int) *Image {
	if im.Empty() || side <= 0 {
		return im
	}
	short := im.W
	if im.H < short {
		short = im.H
	}
	return im.CropSquare(short).Resize(side, side)
}
