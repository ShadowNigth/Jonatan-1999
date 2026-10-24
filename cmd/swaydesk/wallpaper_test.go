package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// Super+Shift+w ejecuta `swaydesk wallpaper --gallery`; si la bandera no se
// reconoce, el atajo parece roto (antes contestaba «no se reconoce el fondo»).
func TestIsGalleryRequest(t *testing.T) {
	yes := [][]string{
		{"--gallery"},
		{"--pick"},
		{"-g"},
		{"gallery"},
	}
	for _, args := range yes {
		if args[0] == "gallery" {
			continue // ese lo resuelve el switch de subcomandos
		}
		if !isGalleryRequest(args) {
			t.Fatalf("%v debería abrir la galería", args)
		}
	}
	no := [][]string{nil, {"set", "scene:petals"}, {"scene:petals"}, {"--scenes"}, {"--live"}}
	for _, args := range no {
		if isGalleryRequest(args) {
			t.Fatalf("%v no debería abrir la galería", args)
		}
	}
}

// Escenas, GIF y vídeo necesitan el demonio; una imagen fija no.
func TestAnimatedSpec(t *testing.T) {
	for _, k := range []wallpaper.Kind{wallpaper.KindScene, wallpaper.KindGIF, wallpaper.KindVideo} {
		if !animatedSpec(wallpaper.Spec{Kind: k}) {
			t.Fatalf("%s debería considerarse animado", k)
		}
	}
	if animatedSpec(wallpaper.Spec{Kind: wallpaper.KindImage}) {
		t.Fatal("una imagen fija no necesita demonio")
	}
}

func TestWallpaperFallbackUsesSolidColorForAnimatedSources(t *testing.T) {
	cfg := config.Default()
	color := ui.PaletteByName(cfg.Theme).Bg
	want := fmt.Sprintf("output * bg %s solid_color", color)
	for _, kind := range []wallpaper.Kind{wallpaper.KindGIF, wallpaper.KindVideo, wallpaper.KindScene} {
		got := wallpaperFallbackLine(cfg, wallpaper.Spec{Kind: kind, Path: "/tmp/wallpaper"})
		if got != want {
			t.Errorf("respaldo para %s = %q, se esperaba %q", kind, got, want)
		}
	}
}

func TestWallpaperFallbackKeepsStaticImage(t *testing.T) {
	cfg := config.Default()
	cfg.WallpaperMode = "fill"
	got := wallpaperFallbackLine(cfg, wallpaper.Spec{Kind: wallpaper.KindImage, Path: "/tmp/fondo.png"})
	if want := `output * bg "/tmp/fondo.png" fill`; got != want {
		t.Fatalf("respaldo para imagen = %q, se esperaba %q", got, want)
	}
}

func TestApplyAnimatedFallbackRemovesLegacyStillFrame(t *testing.T) {
	base := t.TempDir()
	configHome := filepath.Join(base, "config")
	cache := filepath.Join(base, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("SWAYSOCK", filepath.Join(base, "missing-sway.sock"))

	cfg := config.Default()
	cfg.Cache = cache
	legacyFrame := filepath.Join(cache, "fondo-estatico.png")
	if err := os.WriteFile(legacyFrame, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	spec := wallpaper.Spec{Kind: wallpaper.KindGIF, Path: "/tmp/animado.gif"}
	applyStaticFallback(cfg, spec)

	if _, err := os.Stat(legacyFrame); !os.IsNotExist(err) {
		t.Fatalf("el fotograma estático antiguo debería eliminarse; stat err=%v", err)
	}
	path := filepath.Join(configHome, "sway", "config.d", "50-wallpaper.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Fondo de pantalla · generado por swaydesk\n" + wallpaperFallbackLine(cfg, spec) + "\n"
	if string(data) != want {
		t.Fatalf("config de respaldo = %q, se esperaba %q", data, want)
	}
}

func TestSetWallpaperModeKeepsAnimatedFallback(t *testing.T) {
	base := t.TempDir()
	configHome := filepath.Join(base, "config")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("SWAYSOCK", filepath.Join(base, "missing-sway.sock"))

	cfg := config.Default()
	cfg.Dir = filepath.Join(base, "state")
	cfg.Path = filepath.Join(cfg.Dir, "config.toml")
	cfg.Cache = filepath.Join(base, "cache")
	cfg.WallpaperKind = string(wallpaper.KindGIF)
	cfg.WallpaperPath = filepath.Join(base, "animado.gif")

	if err := setWallpaperMode(cfg, []string{"center"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configHome, "sway", "config.d", "50-wallpaper.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Fondo de pantalla · generado por swaydesk\n" + wallpaperFallbackLine(cfg, wallpaper.Spec{Kind: wallpaper.KindGIF, Path: cfg.WallpaperPath}) + "\n"
	if string(data) != want {
		t.Fatalf("set mode escribió un respaldo incorrecto: %q, se esperaba %q", data, want)
	}
}

func TestThemeWallpaperNeedsDaemon(t *testing.T) {
	cfg := config.Default()
	if !themeWallpaperNeedsDaemon(cfg) {
		t.Fatal("la escena por defecto debe continuar tras cambiar la paleta")
	}
	cfg.WallpaperKind = "image"
	cfg.WallpaperPath = "/tmp/fondo.png"
	if themeWallpaperNeedsDaemon(cfg) {
		t.Fatal("una imagen fija sin fondos por workspace no necesita demonio")
	}
	cfg.WallpaperWorkspaces = map[string]string{"2": "scene:petals"}
	if !themeWallpaperNeedsDaemon(cfg) {
		t.Fatal("los fondos por workspace requieren conservar el demonio")
	}
}

func TestWallpaperDaemonNeededForStaticBaseWithWorkspaceRules(t *testing.T) {
	cfg := config.Default()
	cfg.WallpaperKind = "image"
	cfg.WallpaperPath = "/tmp/fondo.png"
	if wallpaperDaemonNeeded(cfg) {
		t.Fatal("una imagen fija global sin reglas no necesita demonio")
	}
	cfg.WallpaperWorkspaces = map[string]string{"2": "image:~/anime.png"}
	if !wallpaperDaemonNeeded(cfg) {
		t.Fatal("un fondo fijo por workspace sí necesita daemon Wayland")
	}
}

func TestRunWallpaperDaemonIfNeededSkipsStaticBase(t *testing.T) {
	cfg := config.Default()
	cfg.Cache = t.TempDir()
	cfg.WallpaperKind = "image"
	cfg.WallpaperPath = "/tmp/fixed-background.png"
	if err := runWallpaperDaemon(cfg, []string{"--if-needed"}); err != nil {
		t.Fatalf("el autostart no debería crear un demonio para una imagen fija global: %v", err)
	}
}

func TestCurrentWallpaperSpecExpandsHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.Default()
	cfg.WallpaperKind = "image"
	cfg.WallpaperPath = "~/base.png"
	spec, err := currentSpec(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "base.png"); spec.Path != want {
		t.Fatalf("path = %q, se esperaba %q", spec.Path, want)
	}
}

func TestWorkspacePlanUsesBaseAndRules(t *testing.T) {
	cfg := config.Default()
	cfg.WallpaperKind = "image"
	cfg.WallpaperPath = "/tmp/base.png"
	cfg.WallpaperWorkspaces = map[string]string{"web": "scene:petals"}
	spec, err := currentSpec(cfg)
	if err != nil {
		t.Fatal(err)
	}
	plan := workspacePlan(cfg, spec)
	if got, key := plan.SourceFor(2, "2:web"); got != "scene:petals" || key != "web" {
		t.Fatalf("workspace web: source=%q key=%q", got, key)
	}
	if got, key := plan.SourceFor(1, "1:term"); got != "image:/tmp/base.png" || key != "" {
		t.Fatalf("fallback: source=%q key=%q", got, key)
	}
}

func TestSetWallpaperTransitionPersists(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Dir = dir
	cfg.Path = filepath.Join(dir, "config.toml")
	cfg.Cache = filepath.Join(dir, "cache")
	cfg.WallpaperKind = "image"
	cfg.WallpaperPath = filepath.Join(dir, "still.png")

	if err := setWallpaperTransition(cfg, []string{"wipe", "475"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadFile(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.WallpaperTransition != "wipe" || loaded.WallpaperTransitionMS != 475 {
		t.Fatalf("transición guardada = %q / %d", loaded.WallpaperTransition, loaded.WallpaperTransitionMS)
	}
	if err := setWallpaperTransition(cfg, []string{"unknown"}); err == nil {
		t.Fatal("se aceptó un tipo de transición desconocido")
	}
	if err := setWallpaperTransition(cfg, []string{"fade", "5001"}); err == nil {
		t.Fatal("se aceptó una duración superior a 5000 ms")
	}
}
