package wallpaper

import (
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

var testPal = ui.PaletteByName("catppuccin")

func TestAvailableScenesCoversCatalogue(t *testing.T) {
	scenes := AvailableScenes()
	if len(scenes) < 10 {
		t.Fatalf("se esperaban al menos 10 escenas, hay %d", len(scenes))
	}
	seen := map[string]bool{}
	for _, s := range scenes {
		if s.Key == "" || s.Name == "" || s.Description == "" {
			t.Fatalf("escena incompleta: %+v", s)
		}
		if seen[s.Key] {
			t.Fatalf("escena duplicada: %s", s.Key)
		}
		seen[s.Key] = true
		if _, ok := SceneByKey(s.Key); !ok {
			t.Fatalf("SceneByKey no encuentra %q", s.Key)
		}
	}
	if !strings.Contains(ToJSON(scenes), scenes[0].Key) {
		t.Fatal("ToJSON no incluye las claves de las escenas")
	}
}

func TestRendererFramesHavePixels(t *testing.T) {
	for _, s := range AvailableScenes() {
		r := NewRenderer(s.Key, testPal)
		c := r.Frame(32, 18)
		if c == nil || c.W != 32 || c.H != 18 {
			t.Fatalf("%s: tamaño de lienzo inesperado: %+v", s.Key, c)
		}
		if len(c.Pix) != 32*18*3 {
			t.Fatalf("%s: búfer de píxeles con longitud %d", s.Key, len(c.Pix))
		}
		// Una escena nunca debe quedar completamente negra.
		sum := 0
		for _, v := range c.Pix {
			sum += int(v)
		}
		if sum == 0 {
			t.Fatalf("%s: fotograma completamente negro", s.Key)
		}
		// El segundo fotograma debe animarse sin desbordar.
		c2 := r.Frame(32, 18)
		if len(c2.Pix) != len(c.Pix) {
			t.Fatalf("%s: el tamaño cambió entre fotogramas", s.Key)
		}
	}
}

func TestCanvasOps(t *testing.T) {
	c := NewCanvas(8, 8, ui.Color("#101010"))
	if c.W != 8 || c.H != 8 || len(c.Pix) != 8*8*3 {
		t.Fatalf("lienzo mal inicializado: %d x %d, %d bytes", c.W, c.H, len(c.Pix))
	}
	c.set(0, 0, 255, 0, 0)
	if c.Pix[0] != 255 || c.Pix[1] != 0 {
		t.Fatal("set no escribió el píxel esperado")
	}
	// Coordenadas fuera de rango no deben provocar pánico.
	c.set(-1, -1, 1, 2, 3)
	c.set(100, 100, 1, 2, 3)
	c.blend(1, 1, 0, 0, 255, 0.5)
	c.disc(4, 4, 2, 10, 20, 30, 0.4)
	c.glow(4, 4, 3, 200, 100, 50, 0.5)
	c.cline(0, 0, 7, 7, 12, 34, 56, 0.7)
	if cap(c.Pix) < len(c.Pix) {
		t.Fatal("búfer inconsistente")
	}
	img := CanvasToImage(c)
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Fatalf("CanvasToImage devolvió %v", img.Bounds())
	}
}

func TestParseSpec(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
		name string
		path string
		err  bool
	}{
		{"scene:petals", KindScene, "petals", "", false},
		{"stars", KindScene, "stars", "", false},
		// Un nombre de paleta se resuelve a su escena asociada.
		{"scene:sakura", KindScene, "petals", "", false},
		{"miku", KindScene, "wave", "", false},
		{"gif:/tmp/a.gif", KindGIF, "", "/tmp/a.gif", false},
		{"image:/tmp/foto.png", KindImage, "", "/tmp/foto.png", false},
		{"video:/tmp/live.mp4", KindVideo, "", "/tmp/live.mp4", false},
		{"/tmp/otra.webp", KindImage, "", "/tmp/otra.webp", false},
		{"/tmp/raro.txt", "", "", "", true},
		{"scene:nope", "", "", "", true},
		{"", "", "", "", true},
	}
	for _, c := range cases {
		spec, err := ParseSpec(c.in, testPal)
		if c.err {
			if err == nil {
				t.Fatalf("%q: se esperaba error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: error inesperado: %v", c.in, err)
		}
		if spec.Kind != c.kind || spec.Path != c.path {
			t.Fatalf("%q: spec = %+v", c.in, spec)
		}
		if c.name != "" && spec.Scene != c.name {
			t.Fatalf("%q: escena = %q", c.in, spec.Scene)
		}
	}
}

func TestEngineSceneAndImage(t *testing.T) {
	eng, err := NewEngine(Spec{Kind: KindScene, Scene: "aurora", FPS: 20, Pal: testPal})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	if eng.Interval() <= 0 {
		t.Fatal("intervalo inválido")
	}
	w, h := eng.Size(80, 40)
	if w <= 0 || h <= 0 {
		t.Fatalf("tamaño inválido: %d x %d", w, h)
	}
	if c := eng.Frame(w, h); c == nil || c.W != w || c.H != h {
		t.Fatal("Frame no devolvió el tamaño solicitado")
	}
	if eng.Describe() == "" {
		t.Fatal("Describe vacío")
	}

	// Imagen temporal.
	dir := t.TempDir()
	path := filepath.Join(dir, "fondo.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 9))
	for x := 0; x < 16; x++ {
		for y := 0; y < 9; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 10), uint8(y * 20), 90, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	still, err := LoadStill(path)
	if err != nil {
		t.Fatal(err)
	}
	if still.W != 16 || still.H != 9 {
		t.Fatalf("imagen %d x %d", still.W, still.H)
	}
	eng2, err := NewEngine(Spec{Kind: KindImage, Path: path, Pal: testPal})
	if err != nil {
		t.Fatal(err)
	}
	defer eng2.Close()
	if c := eng2.Frame(40, 20); c == nil || len(c.Pix) != 40*20*3 {
		t.Fatal("Frame de imagen inválido")
	}
	if eng2.SizeBytes() <= 0 {
		t.Fatal("SizeBytes debería ser positivo para una imagen")
	}
}

func TestImageToCanvasCompositesTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	background := ui.Color("#204060")
	canvas := ImageToCanvasWithBackground(img, 2, 2, background)
	for i := 0; i < len(canvas.Pix); i += 3 {
		if canvas.Pix[i] != 32 || canvas.Pix[i+1] != 64 || canvas.Pix[i+2] != 96 {
			t.Fatalf("píxel transparente = %v; se esperaba el color de fondo", canvas.Pix[i:i+3])
		}
	}

	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	canvas = ImageToCanvasWithBackground(img, 2, 2, ui.Color("#0000ff"))
	if got := canvas.Pix[:3]; got[0] < 126 || got[0] > 130 || got[1] != 0 || got[2] < 125 || got[2] > 129 {
		t.Fatalf("alfa parcial compuesta = %v; se esperaba rojo sobre azul", got)
	}

	gradient := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	gradient.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	gradient.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})
	scaled := ImageToCanvasWithBackground(gradient, 1, 1, ui.Color("#000000"))
	if got := scaled.Pix[:3]; got[0] < 126 || got[0] > 130 || got[1] != 0 || got[2] < 125 || got[2] > 129 {
		t.Fatalf("remuestreo bilineal = %v; se esperaba mezcla rojo/azul", got)
	}
}

func TestGIFLoaderComposesDisposalMethods(t *testing.T) {
	palette := color.Palette{
		color.RGBA{R: 255, A: 255},
		color.RGBA{B: 255, A: 255},
		color.RGBA{G: 255, A: 255},
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
	}
	for _, tc := range []struct {
		name     string
		disposal byte
		want     int
	}{
		{name: "previous", disposal: gif.DisposalPrevious, want: 0},
		{name: "background", disposal: gif.DisposalBackground, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "disposal.gif")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			base := image.NewPaletted(image.Rect(0, 0, 4, 4), palette)
			partial := image.NewPaletted(image.Rect(1, 1, 3, 3), palette)
			for i := range partial.Pix {
				partial.Pix[i] = 1
			}
			last := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
			last.Pix[0] = 3
			animation := &gif.GIF{
				Image: []*image.Paletted{base, partial, last},
				Delay: []int{5, 5, 5},
				Disposal: []byte{gif.DisposalNone, tc.disposal, gif.DisposalNone},
				LoopCount: 0,
				BackgroundIndex: 2,
				Config: image.Config{ColorModel: palette, Width: 4, Height: 4},
			}
			if err := gif.EncodeAll(file, animation); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadGIF(path)
			if err != nil {
				t.Fatal(err)
			}
			r, g, b, _ := loaded.frames[2].At(1, 1).RGBA()
			want := palette[tc.want]
			wr, wg, wb, _ := want.RGBA()
			if r != wr || g != wg || b != wb {
				t.Fatalf("píxel tras disposición = (%d,%d,%d), se esperaba (%d,%d,%d)", r, g, b, wr, wg, wb)
			}
		})
	}
}

func TestGIFLoader(t *testing.T) {
	// GIF animado de dos fotogramas generado a mano.
	dir := t.TempDir()
	path := filepath.Join(dir, "anim.gif")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 0, 0, 255}}
	writeGIF(t, f, pal)
	f.Close()

	g, err := LoadGIF(path)
	if err != nil {
		t.Fatal(err)
	}
	if g.Count() != 2 || !g.IsAnimated() {
		t.Fatalf("se esperaban 2 fotogramas, hay %d", g.Count())
	}
	if g.Duration() <= 0 {
		t.Fatal("duración inválida")
	}
	if _, d := g.FrameAt(50 * time.Millisecond); d <= 0 {
		t.Fatal("FrameAt sin duración")
	}
	if g.Frame(0) == nil {
		t.Fatal("Frame(0) nulo")
	}
}

// writeGIF genera un GIF animado de dos fotogramas para las pruebas.
func writeGIF(t *testing.T, w *os.File, pal color.Palette) {
	t.Helper()
	first := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
	second := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
	for i := range second.Pix {
		second.Pix[i] = 1
	}
	g := &gif.GIF{
		Image:     []*image.Paletted{first, second},
		Delay:     []int{5, 5},
		Disposal:  []byte{gif.DisposalNone, gif.DisposalNone},
		LoopCount: 0,
		Config:    image.Config{ColorModel: pal, Width: 4, Height: 4},
	}
	if err := gif.EncodeAll(w, g); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewBlockRendersImageAndGIFAtRequestedHeight(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "preview.png")
	f, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 12, 8))); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	still, err := NewEngine(Spec{Kind: KindImage, Path: imagePath, Pal: testPal})
	if err != nil {
		t.Fatal(err)
	}
	if got := PreviewBlock(still, 12, 6); len(got) != 6 {
		t.Fatalf("preview de imagen devolvió %d filas; se esperaban 6", len(got))
	}
	still.Close()

	gifPath := filepath.Join(dir, "preview.gif")
	gifFile, err := os.Create(gifPath)
	if err != nil {
		t.Fatal(err)
	}
	writeGIF(t, gifFile, color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 0, 0, 255}})
	if err := gifFile.Close(); err != nil {
		t.Fatal(err)
	}
	animated, err := NewEngine(Spec{Kind: KindGIF, Path: gifPath, Pal: testPal})
	if err != nil {
		t.Fatal(err)
	}
	defer animated.Close()
	if got := PreviewBlock(animated, 12, 6); len(got) != 6 {
		t.Fatalf("preview de GIF devolvió %d filas; se esperaban 6", len(got))
	}
}

func TestPreviewAndANSI(t *testing.T) {
	eng, err := NewEngine(Spec{Kind: KindScene, Scene: "stars", FPS: 24, Pal: testPal})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	lines := PreviewBlock(eng, 24, 12)
	if len(lines) == 0 {
		t.Fatal("PreviewBlock vacío")
	}
	for _, l := range lines {
		if strings.Contains(l, "{{") {
			t.Fatalf("token sin sustituir en la vista previa: %q", l)
		}
	}
	c := eng.Frame(16, 8)
	if s := ANSIFrame(c); !strings.Contains(s, "\x1b[38;2;") {
		t.Fatal("ANSIFrame no genera color de 24 bits")
	}
}

func TestScaleFor(t *testing.T) {
	for _, tc := range []struct{ w, h, want int }{
		{1280, 720, 2},
		{1920, 1080, 2},
		{2560, 1440, 3},
		{3840, 2160, 3},
		{7680, 4320, 4},
		{320, 200, 2},
	} {
		if got := ScaleFor(tc.w, tc.h); got != tc.want {
			t.Fatalf("ScaleFor(%d,%d) = %d, se esperaba %d", tc.w, tc.h, got, tc.want)
		}
	}
}

func TestVideoScaleForKeepsNormalDisplaysAtNativeResolution(t *testing.T) {
	for _, tc := range []struct{ w, h, want int }{
		{1280, 720, 1},
		{1920, 1080, 1},
		{2560, 1440, 2},
		{3840, 2160, 2},
		{7680, 4320, 4},
	} {
		if got := videoScaleFor(tc.w, tc.h); got != tc.want {
			t.Fatalf("videoScaleFor(%d,%d) = %d, se esperaba %d", tc.w, tc.h, got, tc.want)
		}
	}
}

func TestSupported(t *testing.T) {
	yes := []string{"a.gif", "b.PNG", "c.jpg", "d.jpeg", "f.mp4", "g.mkv", "h.webm"}
	for _, n := range yes {
		if !Supported(n) {
			t.Fatalf("%s debería estar soportado", n)
		}
	}
	for _, n := range []string{"a.txt", "b.pdf", "c", "e.webp"} {
		if Supported(n) {
			t.Fatalf("%s no debería estar soportado", n)
		}
	}
}

func TestKittySequence(t *testing.T) {
	// Contenido pseudoaleatorio para que el PNG no se comprima demasiado y la
	// secuencia tenga que partirse en varios trozos.
	img := image.NewRGBA(image.Rect(0, 0, 160, 160))
	seed := uint32(12345)
	for x := 0; x < 160; x++ {
		for y := 0; y < 160; y++ {
			seed = seed*1664525 + 1013904223
			img.Set(x, y, color.RGBA{uint8(seed >> 24), uint8(seed >> 16), uint8(seed >> 8), 255})
		}
	}
	seq, err := KittySequence(img, 20, 10, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seq, "\x1b_G") {
		t.Fatalf("secuencia kitty inválida: %q", seq[:10])
	}
	// Se coloca el cursor y luego transmite la imagen.
	if !strings.Contains(seq, "\x1b[3;2H") {
		t.Fatal("la secuencia no posiciona el cursor")
	}
	if !strings.Contains(seq, "a=T") || !strings.Contains(seq, "f=100") {
		t.Fatal("faltan parámetros de transmisión")
	}
	if !strings.Contains(seq, "c=20,r=10") {
		t.Fatal("faltan las celdas de destino")
	}
	if !strings.HasSuffix(seq, "\x1b\\") {
		t.Fatal("la secuencia no termina con ST")
	}
	// Los trozos grandes se parten en varias transmisiones.
	if n := strings.Count(seq, "\x1b_G"); n < 2 {
		t.Fatalf("se esperaban varias transmisiones, hay %d", n)
	}
	if !strings.Contains(seq, "m=1") || !strings.Contains(seq, "m=0") {
		t.Fatal("la secuencia no marca el final de los datos")
	}
}

func TestSixelEncode(t *testing.T) {
	pix := make([]uint8, 16*8*3)
	for i := range pix {
		pix[i] = uint8(i * 7)
	}
	seq, err := EncodeSixel(pix, 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(seq, "\x1bP") {
		t.Fatal("sixel debe empezar con DCS")
	}
	if !strings.HasSuffix(seq, "\x1b\\") {
		t.Fatal("sixel debe terminar con ST")
	}
	if !strings.Contains(seq, "q") {
		t.Fatal("sixel sin cabecera de modo")
	}
}

func TestCanvasToImageKeepsPixels(t *testing.T) {
	c := NewCanvas(3, 2, ui.Color("#010203"))
	c.set(0, 0, 10, 20, 30)
	c.set(2, 1, 200, 100, 50)
	img := CanvasToImage(c)
	r, g, b, a := img.At(0, 0).RGBA()
	if r>>8 != 10 || g>>8 != 20 || b>>8 != 30 || a>>8 != 255 {
		t.Fatalf("píxel (0,0) = %d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
	r, g, b, _ = img.At(2, 1).RGBA()
	if r>>8 != 200 || g>>8 != 100 || b>>8 != 50 {
		t.Fatalf("píxel (2,1) = %d,%d,%d", r>>8, g>>8, b>>8)
	}
	// El relleno de fondo también debe conservarse.
	r, _, _, _ = img.At(1, 0).RGBA()
	if r>>8 != 1 {
		t.Fatalf("fondo = %d", r>>8)
	}
}
