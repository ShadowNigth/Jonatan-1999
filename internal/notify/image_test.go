package notify

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/dbus"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// pngBytes genera un PNG de w×h con un patrón sencillo.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

func TestDecodeImagePNG(t *testing.T) {
	im, err := DecodeImage(pngBytes(t, 40, 20))
	if err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if im.W != 40 || im.H != 20 {
		t.Fatalf("tamaño = %dx%d", im.W, im.H)
	}
	if len(im.Pix) != 40*20*4 {
		t.Fatalf("búfer de %d bytes", len(im.Pix))
	}
	r, g, b, a := im.At(0, 0)
	if a != 255 || b != 120 {
		t.Fatalf("píxel (0,0) = %d,%d,%d,%d", r, g, b, a)
	}
	// Fuera de límites: transparente, sin pánico.
	if _, _, _, a := im.At(-1, 99); a != 0 {
		t.Fatalf("fuera de límites debe ser transparente: %d", a)
	}
}

func TestDecodeImageRejects(t *testing.T) {
	if _, err := DecodeImage(nil); err == nil {
		t.Fatal("una imagen vacía debe fallar")
	}
	if _, err := DecodeImage([]byte("no soy una imagen")); err == nil {
		t.Fatal("un texto no debe decodificarse como imagen")
	}
	// Un PNG roto: cabecera correcta y datos basura.
	roto := append([]byte("\x89PNG\r\n\x1a\n"), []byte("basura")...)
	if _, err := DecodeImage(roto); err == nil {
		t.Fatal("un PNG corrupto debe fallar")
	}
}

func TestDecodeImageDataRowStrideAndAlpha(t *testing.T) {
	// 2x2 RGBA con relleno al final de cada fila (rowstride 12 con 8 útiles).
	const row = 12
	data := make([]byte, row*2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			i := y*row + x*4
			data[i] = uint8(10 + x)
			data[i+1] = uint8(20 + y)
			data[i+2] = 30
			data[i+3] = 255
		}
	}
	im, err := DecodeImageData(2, 2, row, true, 8, 4, data)
	if err != nil {
		t.Fatalf("image-data: %v", err)
	}
	if im.W != 2 || im.H != 2 {
		t.Fatalf("tamaño = %dx%d", im.W, im.H)
	}
	r, g, b, a := im.At(1, 1)
	if r != 11 || g != 21 || b != 30 || a != 255 {
		t.Fatalf("píxel (1,1) = %d,%d,%d,%d (no se respetó el salto de fila)", r, g, b, a)
	}

	// Sin alfa: tres canales y todo opaco.
	rgb := []byte{1, 2, 3, 4, 5, 6}
	im, err = DecodeImageData(2, 1, 0, false, 8, 3, rgb)
	if err != nil {
		t.Fatalf("RGB: %v", err)
	}
	if im.W != 2 || im.Pix[3] != 255 || im.Pix[7] != 255 {
		t.Fatalf("RGB mal convertido: %+v", im.Pix)
	}

	// Entradas inválidas.
	casos := []struct {
		nombre string
		w, h   int
		salto  int
		alfa   bool
		bits   int
		can    int
		datos  []byte
	}{
		{"sin tamaño", 0, 4, 0, true, 8, 4, data},
		{"demasiado grande", 5000, 4, 0, true, 8, 4, data},
		{"bits raros", 2, 2, 0, true, 16, 4, data},
		{"canales raros", 2, 2, 0, true, 8, 2, data},
		{"datos cortos", 40, 40, 0, true, 8, 4, data},
	}
	for _, c := range casos {
		if _, err := DecodeImageData(c.w, c.h, c.salto, c.alfa, c.bits, c.can, c.datos); err == nil {
			t.Fatalf("%s: debería fallar", c.nombre)
		}
	}
}

func TestImageFitAndThumbnail(t *testing.T) {
	im, err := DecodeImage(pngBytes(t, 900, 300))
	if err != nil {
		t.Fatal(err)
	}
	// Al decodificar se limita el lado mayor a MaxImageSide.
	if im.W != MaxImageSide || im.H > MaxImageSide {
		t.Fatalf("no se limitó el tamaño: %dx%d", im.W, im.H)
	}
	esperado := float64(MaxImageSide) * 300 / 900
	if diff := float64(im.H) - esperado; diff < -1 || diff > 1 {
		t.Fatalf("no se conservó la proporción: %dx%d", im.W, im.H)
	}

	// Miniaturas cuadradas.
	thumb := im.Thumbnail(24)
	if thumb.W != 24 || thumb.H != 24 {
		t.Fatalf("miniatura = %dx%d", thumb.W, thumb.H)
	}
	// Recorte por el centro de una imagen vertical ancha.
	ancha := &Image{W: 100, H: 20, Pix: make([]uint8, 100*20*4)}
	for i := 0; i < len(ancha.Pix); i += 4 {
		ancha.Pix[i+3] = 255
	}
	cuadro := ancha.CropSquare(20)
	if cuadro.W != 20 || cuadro.H != 20 {
		t.Fatalf("recorte = %dx%d", cuadro.W, cuadro.H)
	}
	// Escalado de caja: un 2x2 mitad y mitad sale de un color intermedio.
	mitad := &Image{W: 2, H: 2, Pix: []uint8{
		255, 0, 0, 255, 255, 0, 0, 255,
		0, 0, 255, 255, 0, 0, 255, 255,
	}}
	uno := mitad.Resize(1, 1)
	r, g, b, a := uno.At(0, 0)
	if r != 127 && r != 128 {
		t.Fatalf("mezcla de rojo = %d", r)
	}
	if g != 0 || b != 127 && b != 128 || a != 255 {
		t.Fatalf("mezcla = %d,%d,%d,%d", r, g, b, a)
	}
	// Una imagen vacía no entra en pánico.
	var vacia *Image
	if !vacia.Empty() {
		t.Fatal("una imagen nil debe estar vacía")
	}
	if vacia.Fit(10, 10) != nil {
		t.Fatal("Fit sobre nil debe devolver nil")
	}
	if vacia.Thumbnail(10) != nil {
		t.Fatal("Thumbnail sobre nil debe devolver nil")
	}
}

func TestLoadImageFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foto.png")
	if err := os.WriteFile(path, pngBytes(t, 32, 32), 0o644); err != nil {
		t.Fatal(err)
	}
	im, err := LoadImageFile(path)
	if err != nil || im.W != 32 {
		t.Fatalf("cargar archivo: %v (%v)", err, im)
	}
	// Con el prefijo file:// de algunos clientes.
	if im, err = LoadImageFile("file://" + path); err != nil || im.W != 32 {
		t.Fatalf("file://: %v", err)
	}
	if _, err := LoadImageFile(filepath.Join(dir, "no-existe.png")); err == nil {
		t.Fatal("un archivo inexistente debe fallar")
	}
	if _, err := LoadImageFile(dir); err == nil {
		t.Fatal("un directorio debe fallar")
	}
	if _, err := LoadImageFile(""); err == nil {
		t.Fatal("una ruta vacía debe fallar")
	}
	// Un archivo demasiado grande se rechaza antes de leerlo.
	grande := filepath.Join(dir, "grande.png")
	f, err := os.Create(grande)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxImageBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := LoadImageFile(grande); err == nil {
		t.Fatal("un archivo demasiado grande debe rechazarse")
	}
}

func TestRenderCardWithImage(t *testing.T) {
	now := time.Now()
	im, err := DecodeImage(pngBytes(t, 64, 64))
	if err != nil {
		t.Fatal(err)
	}
	r := New(RenderOptions{
		Width: 360, Padding: 14, Gap: 8, Radius: 14,
		Face: font.Builtin(), Theme: ui.PaletteByName("sakura"), Scale: 2, MaxLines: 3,
	})
	sin := []Notification{{ID: 1, App: "mpd", Summary: "Reproduciendo", Body: "Lofi Girl", Created: now, Progress: -1}}
	con := []Notification{{ID: 2, App: "mpd", Summary: "Reproduciendo", Body: "Lofi Girl", Created: now, Progress: -1, Image: im}}

	cSin, cardsSin := r.RenderGroups(Views(sin, nil), now)
	cCon, cardsCon := r.RenderGroups(Views(con, nil), now)
	if cCon.H <= cSin.H {
		t.Fatalf("la tarjeta con imagen debería ser más alta: %d <= %d", cCon.H, cSin.H)
	}
	if len(cardsCon) != 1 || !cardsCon[0].Notification.HasImage() {
		t.Fatalf("la tarjeta no conserva la imagen: %+v", cardsCon)
	}
	// Hay píxeles de la miniatura: se buscan colores variados en la franja
	// izquierda del bloque de contenido.
	thumb := r.thumbSize()
	y0 := cardsCon[0].Rect.Y + r.Opt.Padding + r.rows(r.sc) + 4
	colores := map[bar.RGBA]bool{}
	for y := y0; y < y0+thumb && y < cCon.H; y++ {
		for x := r.Opt.Padding; x < r.Opt.Padding+thumb; x++ {
			i := (y*cCon.W + x) * 4
			if cCon.Pix[i+3] == 0 {
				continue
			}
			colores[bar.RGBA{R: cCon.Pix[i], G: cCon.Pix[i+1], B: cCon.Pix[i+2], A: cCon.Pix[i+3]}] = true
		}
	}
	if len(colores) < 8 {
		t.Fatalf("la miniatura no se dibujó (solo %d colores distintos)", len(colores))
	}
	if cardsSin[0].Rect.H >= cardsCon[0].Rect.H {
		t.Fatalf("alto sin imagen %d, con imagen %d", cardsSin[0].Rect.H, cardsCon[0].Rect.H)
	}
}

func TestImageHintsResolve(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "arte.png")
	if err := os.WriteFile(path, pngBytes(t, 40, 40), 0o644); err != nil {
		t.Fatal(err)
	}

	// image-path: se carga del disco al recibir la notificación.
	n := Notification{ImagePath: path, Progress: -1}
	resolveImage(&n)
	if !n.HasImage() || n.Image.W != 40 {
		t.Fatalf("no se cargó la imagen de image-path: %+v", n.Image)
	}

	// Una ruta rota no rompe nada: la tarjeta se dibuja sin imagen.
	n = Notification{ImagePath: filepath.Join(dir, "falta.png"), Progress: -1}
	resolveImage(&n)
	if n.HasImage() {
		t.Fatal("no debería haber imagen")
	}

	// Sin image-path, un icono con forma de ruta sirve de miniatura.
	n = Notification{Icon: path, Progress: -1}
	resolveImage(&n)
	if !n.HasImage() || n.Image.W != 40 {
		t.Fatalf("el icono-ruta no se usó como imagen: %+v", n.Image)
	}
	// Un nombre de icono normal no intenta abrirse como archivo.
	n = Notification{Icon: "audio-x-generic", Progress: -1}
	resolveImage(&n)
	if n.HasImage() {
		t.Fatal("un nombre de icono no es una imagen")
	}

	// image-data con la firma (iiibiiay).
	data := []byte{255, 0, 0, 255, 0, 255, 0, 255}
	crudo := []any{int32(2), int32(1), int32(8), true, int32(8), int32(4), data}
	n = Notification{Progress: -1, imageRaw: crudo}
	resolveImage(&n)
	if !n.HasImage() || n.Image.W != 2 || n.Image.H != 1 {
		t.Fatalf("no se decodificó image-data: %+v", n.Image)
	}
	if r, _, _, _ := n.Image.At(0, 0); r != 255 {
		t.Fatalf("píxel decodificado mal: %d", r)
	}

	// Un blob inválido se ignora en vez de propagar el error.
	n = Notification{Progress: -1, imageRaw: []any{int32(0), int32(0)}}
	resolveImage(&n)
	if n.HasImage() {
		t.Fatal("un image-data inválido no debe dar imagen")
	}
}

func TestApplyHintsImagePath(t *testing.T) {
	h := dbus.Dict{{Key: "image-path", Value: "/tmp/arte.png"}}
	var n Notification
	applyHints(&n, h)
	if n.ImagePath != "/tmp/arte.png" {
		t.Fatalf("image-path = %q", n.ImagePath)
	}
	// La variante con guion bajo que usan algunos clientes.
	h = dbus.Dict{{Key: "image_path", Value: "/tmp/otro.png"}}
	n = Notification{}
	applyHints(&n, h)
	if n.ImagePath != "/tmp/otro.png" {
		t.Fatalf("image_path = %q", n.ImagePath)
	}
}

func TestHistoryKeepsImagePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "arte.png")
	if err := os.WriteFile(path, pngBytes(t, 16, 16), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(Options{Timeout: time.Hour})
	n := store.Add(Notification{App: "mpd", Summary: "play", Progress: -1, ImagePath: path})
	store.Dismiss(n.ID, "dismissed")

	hpath := filepath.Join(dir, "historial.json")
	if err := store.SaveHistory(hpath); err != nil {
		t.Fatal(err)
	}
	// El JSON no lleva la imagen, pero sí la ruta.
	raw, err := os.ReadFile(hpath)
	if err != nil {
		t.Fatal(err)
	}
	var recs []map[string]any
	if err := json.Unmarshal(raw, &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0]["image_path"] != path {
		t.Fatalf("historial sin image_path: %s", raw)
	}
	if _, ok := recs[0]["Image"]; ok {
		t.Fatal("la imagen no debe serializarse")
	}

	otro := NewStore(Options{Timeout: time.Hour})
	if err := otro.LoadHistory(hpath); err != nil {
		t.Fatal(err)
	}
	hist := otro.History()
	if len(hist) != 1 || !hist[0].HasImage() || hist[0].ImagePath != path {
		t.Fatalf("al cargar no se recuperó la imagen: %+v", hist)
	}
}

func TestEntryBadgeShowsImage(t *testing.T) {
	e := Entry{Notification: Notification{Summary: "x", ImagePath: "/tmp/a.png"}}
	found := false
	for _, b := range e.Badges() {
		if b == "imagen" {
			found = true
		}
	}
	if !found {
		t.Fatalf("falta la insignia de imagen: %v", e.Badges())
	}
}
