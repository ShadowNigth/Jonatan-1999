package actions

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

func TestLockOptionsReadThePersonalizationSettings(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "sakura"
	cfg.LockTimeFormat = "%I:%M %p"
	cfg.LockDateFormat = "%d/%m/%Y"
	cfg.LockClockSize = 44
	cfg.LockIndicatorRadius = 132
	cfg.LockIndicatorThickness = 5

	options := lockOptionsFromConfig(cfg)
	if options.Color != strings.TrimPrefix(string(ui.PaletteByName("sakura").Bg), "#") ||
		options.TimeFormat != cfg.LockTimeFormat || options.DateFormat != cfg.LockDateFormat ||
		options.ClockSize != 44 || options.IndicatorRadius != 132 || options.IndicatorThickness != 5 {
		t.Fatalf("no se aplicaron las opciones personales de bloqueo: %+v", options)
	}
}

func TestLockWallpaperDefaultsToConfiguredThemeScene(t *testing.T) {
	cfg := config.Default()
	cfg.WallpaperKind = "scene"
	cfg.WallpaperName = ""
	cfg.LockBackground = "current"
	palette := ui.PaletteByName(cfg.Theme)

	spec, enabled, err := lockWallpaperSpec(cfg, palette)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled || spec.Kind != wallpaper.KindScene || spec.Scene != palette.Scene || spec.Pal.Name != palette.Name {
		t.Fatalf("el bloqueo no siguió la escena de la paleta: enabled=%t spec=%+v palette=%+v", enabled, spec, palette)
	}
}

func TestLockWallpaperCanBeSolidOrCustomPath(t *testing.T) {
	cfg := config.Default()
	palette := ui.PaletteByName(cfg.Theme)

	cfg.LockBackground = "none"
	if _, enabled, err := lockWallpaperSpec(cfg, palette); err != nil || enabled {
		t.Fatalf("none debe seleccionar color sólido; enabled=%t err=%v", enabled, err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg.LockBackground = "image:~/Imágenes/anime.png"
	spec, enabled, err := lockWallpaperSpec(cfg, palette)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Imágenes", "anime.png")
	if !enabled || spec.Kind != wallpaper.KindImage || spec.Path != want {
		t.Fatalf("ruta personalizada = %+v, enabled=%t; se esperaba %s", spec, enabled, want)
	}
}

func TestRenderLockSceneIsDeterministic(t *testing.T) {
	spec := wallpaper.Spec{Kind: wallpaper.KindScene, Scene: "petals", Pal: ui.PaletteByName("sakura")}
	first, err := renderLockFrame(spec, 96, 54)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderLockFrame(spec, 96, 54)
	if err != nil {
		t.Fatal(err)
	}
	if first.W != 96 || first.H != 54 || !bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("el fotograma de bloqueo de una escena debe ser estable entre ejecuciones")
	}
}

func TestRenderLockGIFUsesOneStillFrame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.gif")
	palette := color.Palette{color.Black, color.White}
	still := image.NewPaletted(image.Rect(0, 0, 2, 2), palette)
	still.Pix = []byte{0, 1, 1, 0}
	var encoded bytes.Buffer
	if err := gif.Encode(&encoded, still, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	frame, err := renderLockFrame(wallpaper.Spec{
		Kind: wallpaper.KindGIF, Path: path, Pal: ui.PaletteByName("sakura"),
	}, 32, 18)
	if err != nil {
		t.Fatal(err)
	}
	if frame == nil || frame.W != 32 || frame.H < 18 || len(frame.Pix) < frame.W*frame.H*3 {
		t.Fatalf("fotograma GIF inválido: %#v", frame)
	}
}

func TestRenderLockVideoUsesTheWallpaperEngine(t *testing.T) {
	binDir := t.TempDir()
	commands := map[string]string{
		"ffprobe": "#!/bin/sh\nprintf 'width=2\\nheight=2\\nduration=1.0\\n'\n",
		"ffmpeg":  "#!/bin/sh\nprintf '\\000\\000\\377\\377\\000\\000\\377\\377\\000\\000\\377\\377\\000\\000\\377\\377'\n",
	}
	for name, script := range commands {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir)
	frame, err := renderLockVideoFrame(wallpaper.Spec{
		Kind: wallpaper.KindVideo, Path: filepath.Join(t.TempDir(), "anime.mp4"),
		FPS: 30, Pal: ui.PaletteByName("sakura"),
	}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if frame == nil || frame.W != 2 || frame.H != 2 || len(frame.Pix) != 12 || frame.Pix[0] != 255 {
		t.Fatalf("fotograma del motor de wallpaper inválido: %#v", frame)
	}
}

func TestLockFrameSizeLimitsCostWithoutChangingAspectRatio(t *testing.T) {
	w, h := lockFrameSize(3840, 2160)
	if w != 1920 || h != 1080 {
		t.Fatalf("tamaño de fotograma = %dx%d; se esperaba 1920x1080", w, h)
	}
	w, h = lockFrameSize(3440, 1440)
	if w > 1920 || h > 1080 || w < 1919 || h < 803 || h > 805 {
		t.Fatalf("se perdió la proporción ultrapanorámica: %dx%d", w, h)
	}
}
