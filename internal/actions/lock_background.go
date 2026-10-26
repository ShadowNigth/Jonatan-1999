package actions

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

type lockOptions struct {
	Color              string
	Image              string
	TimeFormat         string
	DateFormat         string
	ClockSize          int
	IndicatorRadius    int
	IndicatorThickness int
}

func lockOptionsFromConfig(cfg *config.Config) lockOptions {
	if cfg == nil {
		cfg = config.Default()
	}
	palette := ui.PaletteByName(cfg.Theme)
	fontSize := cfg.LockClockSize
	if fontSize <= 0 {
		fontSize = 48
	}
	radius := cfg.LockIndicatorRadius
	if radius <= 0 {
		radius = 148
	}
	thickness := cfg.LockIndicatorThickness
	if thickness <= 0 {
		thickness = 7
	}
	timeFormat := strings.TrimSpace(cfg.LockTimeFormat)
	if timeFormat == "" {
		timeFormat = "%H:%M"
	}
	dateFormat := strings.TrimSpace(cfg.LockDateFormat)
	if dateFormat == "" {
		dateFormat = "%A %d de %B"
	}
	return lockOptions{
		Color:              strings.TrimPrefix(string(palette.Bg), "#"),
		TimeFormat:         timeFormat,
		DateFormat:         dateFormat,
		ClockSize:          fontSize,
		IndicatorRadius:    radius,
		IndicatorThickness: thickness,
	}
}

// lockWallpaperImage prepara una imagen del fondo configurado, nunca una
// captura del escritorio. Si el fondo es "current", respeta también la regla
// del workspace enfocado. Un fallo solo omite la imagen: el bloqueo usa el
// color adaptativo como respaldo.
func lockWallpaperImage(cfg *config.Config, palette ui.Palette) (string, error) {
	spec, useWallpaper, err := lockWallpaperSpec(cfg, palette)
	if err != nil || !useWallpaper {
		return "", err
	}
	width, height := wallpaper.OutputSize()
	width, height = lockFrameSize(width, height)
	frame, err := renderLockFrame(spec, width, height)
	if err != nil {
		return "", err
	}
	if frame == nil || frame.W < 1 || frame.H < 1 || len(frame.Pix) < frame.W*frame.H*3 {
		return "", fmt.Errorf("el fondo no produjo un fotograma utilizable")
	}
	if cfg.Cache == "" {
		return "", fmt.Errorf("no se pudo determinar la carpeta de caché")
	}
	path := filepath.Join(cfg.Cache, "lockscreen.png")
	if err := saveLockFrame(frame, path); err != nil {
		return "", err
	}
	return path, nil
}

func lockWallpaperSpec(cfg *config.Config, palette ui.Palette) (wallpaper.Spec, bool, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	source := strings.TrimSpace(cfg.LockBackground)
	switch {
	case source == "" || strings.EqualFold(source, "current"):
		source = configuredWallpaperSource(cfg, palette)
		if len(cfg.WallpaperWorkspaces) > 0 {
			if number, name, ok := wallpaper.FocusedWorkspace(); ok {
				workspaceMap := wallpaper.NewWorkspaceMap(source, cfg.WallpaperWorkspaces)
				if active, _ := workspaceMap.SourceFor(number, name); active != "" {
					source = active
				}
			}
		}
	case strings.EqualFold(source, "none"):
		return wallpaper.Spec{Kind: wallpaper.KindNone, Pal: palette}, false, nil
	}

	spec, err := wallpaper.ParseSpec(source, palette)
	if err != nil {
		return wallpaper.Spec{}, false, err
	}
	if spec.Kind == wallpaper.KindNone {
		return spec, false, nil
	}
	if spec.FPS <= 0 {
		spec.FPS = cfg.WallpaperFPS
	}
	if spec.Path != "" {
		spec.Path, err = expandLockPath(spec.Path)
		if err != nil {
			return wallpaper.Spec{}, false, err
		}
	}
	return spec, true, nil
}

func configuredWallpaperSource(cfg *config.Config, palette ui.Palette) string {
	kind := wallpaper.Kind(strings.TrimSpace(cfg.WallpaperKind))
	if kind == "" {
		kind = wallpaper.KindScene
	}
	spec := wallpaper.Spec{
		Kind: kind, Path: cfg.WallpaperPath, Scene: cfg.WallpaperName,
		FPS: cfg.WallpaperFPS, Pal: palette,
	}
	if spec.Kind == wallpaper.KindScene && spec.Scene == "" {
		spec.Scene = palette.Scene
		if spec.Scene == "" {
			spec.Scene = "aurora"
		}
	}
	return spec.Source()
}

func expandLockPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path, err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}

func lockFrameSize(width, height int) (int, int) {
	if width < 1 || height < 1 {
		return 1280, 720
	}
	const maxWidth, maxHeight = 1920, 1080
	if width > maxWidth || height > maxHeight {
		scale := math.Min(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
		width = int(math.Round(float64(width) * scale))
		height = int(math.Round(float64(height) * scale))
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return width, height
}

func renderLockFrame(spec wallpaper.Spec, width, height int) (*wallpaper.Canvas, error) {
	if width < 2 || height < 2 {
		return nil, fmt.Errorf("resolución inválida para el fondo de bloqueo: %dx%d", width, height)
	}
	switch spec.Kind {
	case wallpaper.KindScene:
		if spec.Scene == "" {
			return nil, fmt.Errorf("la escena del fondo está vacía")
		}
		return wallpaper.NewRenderer(spec.Scene, spec.Pal).Frame(width, height), nil
	case wallpaper.KindImage, wallpaper.KindGIF:
		still, err := wallpaper.LoadStill(spec.Path)
		if err != nil {
			return nil, err
		}
		rows := (height + 1) / 2
		return wallpaper.ImageToCanvasWithBackground(still.Img, width, rows, spec.Pal.Bg), nil
	case wallpaper.KindVideo:
		return renderLockVideoFrame(spec, width, height)
	case wallpaper.KindNone:
		return nil, nil
	default:
		return nil, fmt.Errorf("tipo de fondo no soportado para el bloqueo: %s", spec.Kind)
	}
}

func renderLockVideoFrame(spec wallpaper.Spec, width, height int) (*wallpaper.Canvas, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	engine, err := wallpaper.NewEngineContext(ctx, spec)
	if err != nil {
		return nil, err
	}
	defer engine.Close()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		frame := engine.Frame(width, height)
		if engine.FrameReady() && frame != nil && frame.W == width && frame.H == height {
			return frame, nil
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, fmt.Errorf("el motor no obtuvo un fotograma de vídeo antes del límite de tiempo: %w", ctx.Err())
		}
	}
}

func saveLockFrame(frame *wallpaper.Canvas, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creando caché del fondo de bloqueo: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".lockscreen-*.png")
	if err != nil {
		return fmt.Errorf("creando archivo temporal del fondo de bloqueo: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	defer os.Remove(tmpPath)
	if err := wallpaper.SaveCanvas(frame, tmpPath); err != nil {
		return fmt.Errorf("guardando fotograma del wallpaper: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("actualizando el fondo del bloqueo: %w", err)
	}
	return nil
}
