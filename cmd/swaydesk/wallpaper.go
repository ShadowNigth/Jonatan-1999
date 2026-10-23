package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/adaptive"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/gallery"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/setup"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// cmdWallpaper gestiona los fondos de pantalla: escenas animadas, GIF, vídeo y
// paletas adaptativas generadas a partir de la imagen.
func cmdWallpaper(args []string) {
	if len(args) == 0 {
		printWallpaperHelp()
		return
	}
	sub := args[0]
	rest := args[1:]
	cfg, err := config.Load()
	must(err)
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	// Banderas de conveniencia que funcionan con cualquier subcomando.
	if hasFlag(args, "--scenes") || hasFlag(args, "--list") || hasFlag(args, "--ls") {
		listScenes(cfg, theme, hasFlag(args, "--json"))
		return
	}
	if hasFlag(args, "--stop") {
		wallpaper.StopDaemon(cfg.Cache)
		fmt.Println("✔ demonio de fondo detenido")
		return
	}
	if hasFlag(args, "--pick") {
		must(runWallpaperGallery(cfg))
		return
	}
	if hasFlag(args, "--live") {
		must(runWallpaperDaemon(cfg, args))
		return
	}
	if f := flagInt(args, "--fps", 0); f > 0 {
		cfg.WallpaperFPS = f
	}

	switch sub {
	case "list", "ls", "scenes", "escenas":
		listScenes(cfg, theme, hasFlag(rest, "--json"))
	case "set", "aplicar":
		must(setWallpaper(cfg, rest))
	case "gallery", "galeria", "picker":
		must(runWallpaperGallery(cfg))
	case "play", "preview", "ver", "reproducir":
		must(playWallpaper(cfg, rest))
	case "daemon", "start", "demonio":
		must(runWallpaperDaemon(cfg, rest))
	case "stop", "parar":
		if wallpaper.StopDaemon(cfg.Cache) {
			fmt.Println("✔ demonio de fondo detenido")
		} else {
			fmt.Println("· no había ningún demonio en ejecución")
		}
	case "status", "estado":
		wallpaperStatus(cfg, theme)
	case "adaptive", "adaptativo":
		must(adaptiveTheme(cfg, theme, rest))
	case "mode", "modo":
		must(setWallpaperMode(cfg, rest))
	case "clean", "limpiar":
		must(cleanWallpaper(cfg))
	case "help", "--help", "-h":
		printWallpaperHelp()
	default:
		// Atajo: `swaydesk wallpaper <algo>` equivale a `wallpaper set <algo>`.
		must(setWallpaper(cfg, args))
	}
}

func printWallpaperHelp() {
	fmt.Print(`swaydesk wallpaper · fondos estáticos y animados

  list [--json]           Escenas animadas disponibles y archivos encontrados
  set <fondo> [modo]      Aplica un fondo (escena, imagen, gif o vídeo)
  gallery                 Galería interactiva con vista previa animada
  play [fondo]            Reproduce el fondo a pantalla completa en la terminal
  daemon                  Demonio de fondo animado (wlr-layer-shell, detrás de todo)
  stop                    Detiene el demonio
  status                  Estado del fondo actual
  mode <fill|fit|...>     Modo de ajuste de imágenes y vídeos
  adaptive [archivo]      Genera un tema adaptado a los colores del fondo
  clean                   Elimina los archivos generados del fondo

Formatos de <fondo>:
  scene:petals            escena procedural animada (ver "swaydesk wallpaper list")
  gif:~/fondo.gif         GIF animado
  image:~/foto.png        imagen fija
  video:~/live.mp4        vídeo (requiere ffmpeg)
  ~/foto.png              el tipo se deduce de la extensión
  theme                   usa la escena asociada al tema activo

Ejemplos:
  swaydesk wallpaper set scene:sakura
  swaydesk wallpaper set ~/Imágenes/gif/lluvia.gif && swaydesk wallpaper daemon
  swaydesk wallpaper adaptive ~/Imágenes/fondo.png
`)
}

// flagInt lee una bandera con valor numérico (--fps 30, --fps=30).
func flagInt(args []string, name string, def int) int {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				return n
			}
		}
		if strings.HasPrefix(a, name+"=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(a, name+"=")); err == nil {
				return n
			}
		}
	}
	return def
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// ───────────────────────── list ─────────────────────────

func listScenes(cfg *config.Config, theme *ui.Theme, asJSON bool) {
	scenes := wallpaper.AvailableScenes()
	if asJSON {
		fmt.Println(wallpaper.ToJSON(scenes))
		return
	}
	p := theme.Palette
	fmt.Println()
	fmt.Printf("  %s %s\n\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("escenas animadas"),
		ui.Dim("(swaydesk wallpaper set scene:<clave>)"))
	for _, s := range scenes {
		active := " "
		if cfg.WallpaperKind == "scene" && cfg.WallpaperName == s.Key {
			active = ui.Style{Fg: p.Ok}.Render("●")
		}
		desc := ui.PadRight(ui.Truncate(s.Description, 34), 34)
		tags := ui.Truncate(strings.Join(s.Tags, ", "), 30)
		fmt.Printf("  %s %-9s %-21s %s %s\n", active, s.Key, ui.Truncate(s.Name, 20),
			ui.Dim(desc), ui.Dim(tags))
	}
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("archivos"),
		ui.Dim("buscando en "+cfg.WallpaperDir))
	files := findWallpapers(cfg)
	if len(files) == 0 {
		fmt.Printf("  %s\n", ui.Dim("no se encontraron imágenes, gifs ni vídeos"))
	}
	for _, f := range files {
		kind := strings.TrimPrefix(filepath.Ext(f), ".")
		fmt.Printf("   %-10s %s\n", ui.Dim(kind), f)
	}
	fmt.Println()
	fmt.Println()
	fmt.Printf("  %s\n", ui.Dim("Galería interactiva: swaydesk wallpaper gallery"))
	fmt.Printf("  %s\n", ui.Dim("Atajo de la sesión: Super+Shift+w"))
}

// findWallpapers recorre la carpeta configurada y las habituales.
func findWallpapers(cfg *config.Config) []string {
	home, _ := os.UserHomeDir()
	dirs := []string{cfg.WallpaperDir,
		filepath.Join(home, "Imágenes"), filepath.Join(home, "Pictures"),
		filepath.Join(home, "Vídeos"), filepath.Join(home, "Videos"),
	}
	seen := map[string]bool{}
	var out []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !wallpaper.Supported(e.Name()) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// ───────────────────────── set ─────────────────────────

// resolveSpec construye la especificación a partir de un texto, resolviendo la
// palabra clave "theme" y las rutas relativas.
func resolveSpec(cfg *config.Config, text string) (wallpaper.Spec, error) {
	pal := ui.PaletteByName(cfg.Theme)
	if strings.EqualFold(text, "theme") || text == "" {
		scene := pal.Scene
		if scene == "" {
			scene = "aurora"
		}
		return wallpaper.Spec{Kind: wallpaper.KindScene, Scene: scene, FPS: cfg.WallpaperFPS, Pal: pal}, nil
	}
	spec, err := wallpaper.ParseSpec(text, pal)
	if err != nil {
		return spec, err
	}
	if spec.Path != "" {
		abs, err := expandPath(spec.Path)
		if err != nil {
			return spec, err
		}
		if _, err := os.Stat(abs); err != nil {
			return spec, fmt.Errorf("no se puede leer %s", abs)
		}
		spec.Path = abs
	}
	if spec.FPS == 0 {
		spec.FPS = cfg.WallpaperFPS
	}
	return spec, nil
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p, err
		}
		p = filepath.Join(home, p[2:])
	}
	return filepath.Abs(p)
}

func setWallpaper(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("indica el fondo: swaydesk wallpaper set scene:sakura")
	}
	text := ""
	mode := cfg.WallpaperMode
	for _, a := range args {
		switch a {
		case "fill", "fit", "stretch", "center", "tile", "scaled":
			mode = a
		default:
			if text == "" {
				text = a
			}
		}
	}
	spec, err := resolveSpec(cfg, text)
	if err != nil {
		return err
	}
	// Valida que el motor pueda cargarlo antes de guardar nada.
	engine, err := wallpaper.NewEngine(spec)
	if err != nil {
		return err
	}
	defer engine.Close()

	cfg.WallpaperKind = string(spec.Kind)
	cfg.WallpaperName = spec.Scene
	cfg.WallpaperPath = spec.Path
	cfg.WallpaperMode = mode
	if err := saveWallpaperConfig(cfg); err != nil {
		return err
	}

	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	fmt.Printf("%s fondo: %s\n", ui.Style{Fg: theme.Palette.Ok}.Render("✔"),
		ui.Style{Fg: theme.Palette.Accent}.B().Render(engine.Describe()))

	// Fondo estático de respaldo en sway (visible debajo y al iniciar sesión).
	applyStaticFallback(cfg, spec)

	// El demonio animado se reinicia con el nuevo fondo.
	if wallpaper.RunningPID(cfg.Cache) != 0 {
		wallpaper.StopDaemon(cfg.Cache)
		if err := startWallpaperDaemon(cfg); err != nil {
			fmt.Printf("  %s\n", ui.Dim("el demonio animado no pudo reiniciarse: "+err.Error()))
		}
	}

	if cfg.WallpaperAdaptive && spec.Path != "" {
		if err := adaptThemeToWallpaper(cfg, ui.NewTheme(cfg.Theme, cfg.NerdFont)); err == nil {
			fmt.Printf("  %s\n", ui.Dim("paleta adaptada al fondo y aplicada"))
		}
	}
	return nil
}

// applyStaticFallback deja un fondo estático en la configuración de sway.
func applyStaticFallback(cfg *config.Config, spec wallpaper.Spec) {
	confDir := filepath.Join(configDir(), "sway", "config.d")
	_ = os.MkdirAll(confDir, 0o755)
	var line string
	switch spec.Kind {
	case wallpaper.KindImage:
		line = fmt.Sprintf("output * bg \"%s\" %s", spec.Path, cfg.WallpaperMode)
	case wallpaper.KindGIF:
		// sway no dibuja GIF: se usa el primer fotograma exportado a PNG.
		if still, err := wallpaper.LoadStill(spec.Path); err == nil {
			path := filepath.Join(cfg.Cache, "fondo-estatico.png")
			if err := wallpaper.SaveImage(still.Img, path); err == nil {
				line = fmt.Sprintf("output * bg \"%s\" %s", path, cfg.WallpaperMode)
			}
		}
	case wallpaper.KindVideo:
		if frame, err := wallpaper.VideoFrame(spec.Path, 0, 1920); err == nil {
			path := filepath.Join(cfg.Cache, "fondo-estatico.png")
			if err := wallpaper.SaveImage(frame, path); err == nil {
				line = fmt.Sprintf("output * bg \"%s\" %s", path, cfg.WallpaperMode)
			}
		}
	default:
		bg := ui.PaletteByName(cfg.Theme).Bg
		line = fmt.Sprintf("output * bg %s solid_color", bg)
	}
	if line == "" {
		return
	}
	content := "# Fondo de pantalla · generado por swaydesk\n" + line + "\n"
	_ = os.WriteFile(filepath.Join(confDir, "50-wallpaper.conf"), []byte(content), 0o644)
	if c, err := swayipc.Dial(); err == nil {
		_ = c.CommandOK(line)
		c.Close()
	}
}

// saveWallpaperConfig persiste la configuración completa con el nuevo fondo.
func saveWallpaperConfig(cfg *config.Config) error { return cfg.Save() }

func setWallpaperMode(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("modos: fill, fit, stretch, center, tile")
	}
	mode := args[0]
	switch mode {
	case "fill", "fit", "stretch", "center", "tile":
	default:
		return fmt.Errorf("modo inválido %q", mode)
	}
	cfg.WallpaperMode = mode
	if err := saveWallpaperConfig(cfg); err != nil {
		return err
	}
	applyStaticFallback(cfg, wallpaper.Spec{Kind: wallpaper.KindImage, Path: cfg.WallpaperPath})
	fmt.Printf("✔ modo de fondo: %s\n", mode)
	return nil
}

// cleanWallpaper borra los archivos generados para el fondo.
func cleanWallpaper(cfg *config.Config) error {
	wallpaper.StopDaemon(cfg.Cache)
	targets := []string{
		filepath.Join(configDir(), "sway", "config.d", "50-wallpaper.conf"),
		filepath.Join(cfg.Cache, "fondo-estatico.png"),
	}
	for _, t := range targets {
		if err := os.Remove(t); err == nil {
			fmt.Printf("  %s eliminado\n", ui.Dim(t))
		}
	}
	if c, err := swayipc.Dial(); err == nil {
		bg := ui.PaletteByName(cfg.Theme).Bg
		_ = c.CommandOK(fmt.Sprintf("output * bg %s solid_color", bg))
		c.Close()
	}
	fmt.Println("✔ fondo restablecido al color del tema")
	return nil
}

// ───────────────────────── play / daemon ─────────────────────────

func playWallpaper(cfg *config.Config, args []string) error {
	spec, err := currentOrArgSpec(cfg, args)
	if err != nil {
		return err
	}
	engine, err := wallpaper.NewEngine(spec)
	if err != nil {
		return err
	}
	defer engine.Close()
	opts := wallpaper.Options{FPS: spec.FPS, ShowInfo: true}
	return wallpaper.RunTerminal(engine, opts)
}

func currentOrArgSpec(cfg *config.Config, args []string) (wallpaper.Spec, error) {
	if len(args) > 0 {
		return resolveSpec(cfg, args[0])
	}
	return currentSpec(cfg)
}

// currentSpec devuelve el fondo configurado, resolviendo el modo "theme".
func currentSpec(cfg *config.Config) (wallpaper.Spec, error) {
	pal := ui.PaletteByName(cfg.Theme)
	spec := wallpaper.Spec{
		Kind: wallpaper.Kind(cfg.WallpaperKind), Path: cfg.WallpaperPath,
		Scene: cfg.WallpaperName, FPS: cfg.WallpaperFPS, Pal: pal,
	}
	if spec.Kind == "" {
		spec.Kind = wallpaper.KindScene
	}
	if spec.Kind == wallpaper.KindScene && spec.Scene == "" {
		spec.Scene = pal.Scene
		if spec.Scene == "" {
			spec.Scene = "aurora"
		}
	}
	return spec, nil
}

// runWallpaperDaemon arranca el fondo animado real (detrás de las ventanas).
func runWallpaperDaemon(cfg *config.Config, args []string) error {
	if wallpaper.RunningPID(cfg.Cache) != 0 {
		wallpaper.StopDaemon(cfg.Cache)
	}
	if hasFlag(args, "--foreground") || hasFlag(args, "-f") {
		return startWallpaperDaemon(cfg)
	}
	// Se relanza en segundo plano para no bloquear el arranque de sway.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := launcher.StartDetached(exe, "wallpaper", "daemon", "--foreground"); err != nil {
		return err
	}
	fmt.Println("✔ demonio de fondo animado iniciado")
	return nil
}

func startWallpaperDaemon(cfg *config.Config) error {
	spec, err := currentSpec(cfg)
	if err != nil {
		return err
	}
	engine, err := wallpaper.NewEngine(spec)
	if err != nil {
		return err
	}
	defer engine.Close()
	wallpaper.SetPIDPath(wallpaper.PIDPath(cfg.Cache))
	if !wallpaper.WaylandReady() {
		return fmt.Errorf("no hay compositor Wayland con wlr-layer-shell")
	}
	return wallpaper.RunDaemon(engine, nil, 0)
}

func wallpaperStatus(cfg *config.Config, theme *ui.Theme) {
	p := theme.Palette
	pal := ui.PaletteByName(cfg.Theme)
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("fondo de pantalla"), ui.Dim("· "+cfg.WallpaperMode))
	kind := cfg.WallpaperKind
	if kind == "" {
		kind = "scene"
	}
	where := cfg.WallpaperName
	if where == "" {
		where = cfg.WallpaperPath
	}
	if where == "" {
		where = "(según el tema)"
	}
	fmt.Printf("  %-12s %s\n", ui.Dim("tipo"), kind)
	fmt.Printf("  %-12s %s\n", ui.Dim("origen"), where)
	if pid := wallpaper.RunningPID(cfg.Cache); pid != 0 {
		fmt.Printf("  %-12s %s\n", ui.Dim("demonio"), ui.Style{Fg: p.Ok}.Render(fmt.Sprintf("activo (pid %d)", pid)))
	} else {
		fmt.Printf("  %-12s %s\n", ui.Dim("demonio"), ui.Style{Fg: p.Warn}.Render("detenido · swaydesk wallpaper daemon"))
	}
	ds := wallpaper.DetectDisplayServer()
	if ds.LayerShell {
		fmt.Printf("  %-12s %s\n", ui.Dim("compositor"), ui.Style{Fg: p.Ok}.Render("Wayland + wlr-layer-shell"))
	} else {
		fmt.Printf("  %-12s %s\n", ui.Dim("compositor"), ui.Style{Fg: p.Warn}.Render("sin wlr-layer-shell"))
	}
	proto := wallpaper.DetectProtocol()
	protoName := "ninguno (medios bloques)"
	switch proto {
	case wallpaper.ProtoKitty:
		protoName = "kitty"
	case wallpaper.ProtoSixel:
		protoName = "sixel"
	}
	fmt.Printf("  %-12s %s\n", ui.Dim("imágenes"), protoName)
	fmt.Printf("  %-12s %s\n", ui.Dim("escena tema"), pal.Scene)
	fmt.Println()
}

// ───────────────────────── galería ─────────────────────────

func runWallpaperGallery(cfg *config.Config) error {
	if relaunchInTerminal(cfg, "swaydesk-wallpapers", 108, 34, []string{"wallpaper", "gallery"}) {
		return nil
	}
	pal := ui.PaletteByName(cfg.Theme)
	dirs := []string{cfg.WallpaperDir}
	home, _ := os.UserHomeDir()
	dirs = append(dirs, filepath.Join(home, "Imágenes"), filepath.Join(home, "Pictures"),
		filepath.Join(home, "Vídeos"), filepath.Join(home, "Videos"))
	res, err := gallery.WallpaperPicker(pal, cfg.NerdFont, dirs, cfg.WallpaperKind+":"+cfg.WallpaperName)
	if err != nil {
		return err
	}
	if !res.Applied {
		return nil
	}
	cfg.WallpaperKind = string(res.Spec.Kind)
	cfg.WallpaperName = res.Spec.Scene
	cfg.WallpaperPath = res.Spec.Path
	if err := saveWallpaperConfig(cfg); err != nil {
		return err
	}
	applyStaticFallback(cfg, res.Spec)
	if wallpaper.RunningPID(cfg.Cache) != 0 {
		wallpaper.StopDaemon(cfg.Cache)
		_ = startWallpaperDaemon(cfg)
	}
	if cfg.WallpaperAdaptive && res.Spec.Path != "" {
		_ = adaptThemeToWallpaper(cfg, ui.NewTheme(cfg.Theme, cfg.NerdFont))
	}
	return nil
}

// ───────────────────────── paleta adaptativa ─────────────────────────

func adaptiveTheme(cfg *config.Config, theme *ui.Theme, args []string) error {
	path := cfg.WallpaperPath
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		p, err := expandPath(args[0])
		if err != nil {
			return err
		}
		path = p
	}
	if path == "" {
		return fmt.Errorf("indica una imagen o configura antes un fondo con imagen/gif")
	}
	pal, err := adaptive.FromFile(path, "")
	if err != nil {
		return err
	}
	key := "adaptativo"
	ui.RegisterPalette(key, pal)
	if err := adaptive.Save(pal, filepath.Join(cfg.Dir, "themes", "adaptativo.toml")); err != nil {
		return err
	}
	if err := cfg.SaveTheme(key); err != nil {
		return err
	}
	if _, err := setup.Apply(cfg, configDir()); err != nil {
		return err
	}
	fmt.Printf("✔ tema adaptativo generado y aplicado\n")
	fmt.Printf("  %s\n", gallery.DescribeTheme(pal))
	fmt.Printf("  %s\n", ui.Dim("guardado en "+filepath.Join(cfg.Dir, "themes", "adaptativo.toml")))
	return nil
}

// adaptThemeToWallpaper aplica una paleta derivada del fondo activo.
func adaptThemeToWallpaper(cfg *config.Config, theme *ui.Theme) error {
	if cfg.WallpaperPath == "" {
		return nil
	}
	pal, err := adaptive.FromFile(cfg.WallpaperPath, "Adaptativo · "+time.Now().Format("02 ene 15:04"))
	if err != nil {
		return err
	}
	ui.RegisterPalette("adaptativo", pal)
	if err := adaptive.Save(pal, filepath.Join(cfg.Dir, "themes", "adaptativo.toml")); err != nil {
		return err
	}
	if err := cfg.SaveTheme("adaptativo"); err != nil {
		return err
	}
	_, err = setup.Apply(cfg, configDir())
	return err
}
