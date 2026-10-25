package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/adaptive"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/gallery"
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
	if hasFlag(args, "--sync-daemon") {
		// Tarea interna desacoplada para que aplicar un fondo desde la galería
		// no espere a que Wayland y el decodificador arranquen.
		syncWallpaperDaemon(cfg)
		return
	}
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
	// Banderas de la galería: es exactamente lo que ejecuta Super+Shift+w
	// (`swaydesk wallpaper --gallery`). Sin esto caía en `set` y respondía
	// «no se reconoce el fondo "--gallery"».
	if isGalleryRequest(args) {
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
	case "logs", "log", "registro":
		printWallpaperLog(cfg, rest)
	case "status", "estado":
		wallpaperStatus(cfg, theme)
	case "adaptive", "adaptativo":
		must(adaptiveTheme(cfg, theme, rest))
	case "mode", "modo":
		must(setWallpaperMode(cfg, rest))
	case "workspaces", "espacios":
		must(wallpaperWorkspaces(cfg, rest))
	case "transition", "transicion", "transición":
		must(setWallpaperTransition(cfg, rest))
	case "clean", "limpiar":
		must(cleanWallpaper(cfg))
	case "help", "--help", "-h":
		printWallpaperHelp()
	default:
		// Atajo: `swaydesk wallpaper <algo>` equivale a `wallpaper set <algo>`.
		must(setWallpaper(cfg, args))
	}
}

// isGalleryRequest reconoce las banderas que abren el selector de fondos:
// `swaydesk wallpaper --gallery` es lo que ejecuta Super+Shift+w.
func isGalleryRequest(args []string) bool {
	return hasFlag(args, "--gallery") || hasFlag(args, "--pick") || hasFlag(args, "-g")
}

func printWallpaperHelp() {
	fmt.Print(`swaydesk wallpaper · fondos estáticos y animados

  list [--json]           Escenas animadas disponibles y archivos encontrados
  set <fondo> [modo]      Aplica un fondo (escena, imagen, gif o vídeo)
  gallery                 Galería interactiva con vista previa animada
  play [fondo]            Reproduce el fondo a pantalla completa en la terminal
  daemon [--if-needed]    Demonio de fondo animado (--if-needed: no reiniciar
                          si ya está en marcha, para el autostart)
  stop                    Detiene el demonio
  logs                    Registro del demonio (por qué no arranca)
  status                  Estado del fondo actual
  mode <fill|fit|...>     Modo de ajuste de imágenes y vídeos
  workspaces [list]       Fondos por workspace y transición actual
  workspaces set <ws> <fondo>
                          Fondo propio para un workspace (nombre, número o
                          sufijo tras ':')
  workspaces unset <ws>   Quita el fondo propio de un workspace
  workspaces clear        Quita todos los fondos por workspace
  transition <tipo> [ms]  fade · slide · wipe · zoom · none
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
		ui.Dim("en "+cfg.WallpaperDir+" y ~/Imágenes, ~/Pictures, ~/Vídeos"))
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
		// Subcarpetas incluidas (hasta tres niveles): la gente organiza los
		// fondos en ~/Imágenes/Fondos/anime/… y antes no aparecían.
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if rel, relErr := filepath.Rel(dir, path); relErr == nil {
					if base := d.Name(); rel != "." &&
						(strings.HasPrefix(base, ".") || strings.Count(rel, string(os.PathSeparator)) >= 3) {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if !wallpaper.Supported(d.Name()) || seen[path] {
				return nil
			}
			seen[path] = true
			out = append(out, path)
			return nil
		})
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

	// El demonio se pone al día con el fondo nuevo. Si es una escena, GIF,
	// vídeo o hay reglas por workspace, se arranca aunque no estuviera en marcha;
	// de otro modo la configuración se guardaba pero el fondo no se veía.
	syncWallpaperDaemon(cfg)

	if cfg.WallpaperAdaptive && spec.Path != "" {
		if err := adaptThemeToWallpaper(cfg, ui.NewTheme(cfg.Theme, cfg.NerdFont)); err == nil {
			fmt.Printf("  %s\n", ui.Dim("paleta adaptada al fondo y aplicada"))
		}
	}
	return nil
}

// wallpaperFallbackLine deja las imágenes fijas como respaldo directo; para
// fondos animados usa un color sólido para que el primer fotograma no quede
// superpuesto al GIF/vídeo que dibuja la superficie Wayland.
func wallpaperFallbackLine(cfg *config.Config, spec wallpaper.Spec) string {
	if spec.Kind == wallpaper.KindImage {
		return fmt.Sprintf("output * bg \"%s\" %s", spec.Path, cfg.WallpaperMode)
	}
	bg := ui.PaletteByName(cfg.Theme).Bg
	return fmt.Sprintf("output * bg %s solid_color", bg)
}

// applyStaticFallback deja el respaldo de Sway acorde al fondo activo.
func applyStaticFallback(cfg *config.Config, spec wallpaper.Spec) {
	confDir := filepath.Join(configDir(), "sway", "config.d")
	_ = os.MkdirAll(confDir, 0o755)
	line := wallpaperFallbackLine(cfg, spec)
	if spec.Kind != wallpaper.KindImage && cfg.Cache != "" {
		// Quitar el fotograma que generaban versiones anteriores: conservarlo
		// detrás de una superficie transparente puede parecer una captura fija.
		_ = os.Remove(filepath.Join(cfg.Cache, "fondo-estatico.png"))
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
	spec, err := currentSpec(cfg)
	if err != nil {
		return err
	}
	applyStaticFallback(cfg, spec)
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
	if spec.Path != "" {
		path, err := expandPath(spec.Path)
		if err != nil {
			return wallpaper.Spec{}, err
		}
		spec.Path = path
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
	pid := wallpaper.RunningPID(cfg.Cache)
	if hasFlag(args, "--if-needed") {
		// El autostart se repite en cada sway reload: conserva un demonio vivo,
		// pero solo crea uno cuando la animación o el mapa de workspaces lo usa.
		if pid != 0 {
			if !wallpaperDaemonNeeded(cfg) {
				wallpaper.StopDaemon(cfg.Cache)
			}
			return nil
		}
		if _, err := currentSpec(cfg); err != nil {
			return err
		}
		if !wallpaperDaemonNeeded(cfg) {
			return nil
		}
	} else if pid != 0 {
		wallpaper.StopDaemon(cfg.Cache)
	}
	if hasFlag(args, "--foreground") || hasFlag(args, "-f") {
		return startWallpaperDaemon(cfg)
	}
	// Se relanza en segundo plano (con la salida a un registro: si muere, el
	// motivo queda escrito en vez de perderse).
	if err := spawnWallpaperDaemon(cfg); err != nil {
		return err
	}
	// Comprobar que sobrevive de verdad. Antes se decía «iniciado» aunque
	// hubiera muerto al instante por falta de compositor o de shm.
	for i := 0; i < 30; i++ {
		if pid := wallpaper.RunningPID(cfg.Cache); pid != 0 {
			fmt.Printf("✔ demonio de fondo animado iniciado (pid %d)\n", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	tail := lastLogLines(wallpaperLogPath(cfg), 12)
	if tail == "" {
		tail = "(el registro está vacío)"
	}
	return fmt.Errorf("el demonio no llegó a arrancar. Registro %s:\n%s",
		wallpaperLogPath(cfg), tail)
}

// wallpaperLogPath es el registro del demonio de fondo.
func wallpaperLogPath(cfg *config.Config) string {
	return filepath.Join(cfg.Cache, "wallpaper.log")
}

// spawnWallpaperDaemon lanza `swaydesk wallpaper daemon --foreground` en su
// propia sesión, con stdout y stderr al registro.
func spawnWallpaperDaemon(cfg *config.Config) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := wallpaperLogPath(cfg)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd := exec.Command(exe, "wallpaper", "daemon", "--foreground")
	cmd.Stdout, cmd.Stderr = f, f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// lastLogLines devuelve las últimas n líneas no vacías de un archivo.
func lastLogLines(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// printWallpaperLog muestra el registro del demonio; `--clear` lo vacía.
func printWallpaperLog(cfg *config.Config, args []string) {
	path := wallpaperLogPath(cfg)
	if hasFlag(args, "--clear") || hasFlag(args, "-c") {
		if err := os.Remove(path); err == nil {
			fmt.Printf("✔ registro borrado (%s)\n", path)
		} else {
			fmt.Printf("· no había registro (%s)\n", path)
		}
		return
	}
	fmt.Printf("  %s %s\n", ui.Dim("registro"), path)
	txt := lastLogLines(path, 40)
	if txt == "" {
		fmt.Printf("  %s\n", ui.Dim("vacío: el demonio no ha escrito nada todavía"))
		return
	}
	fmt.Println()
	fmt.Println(txt)
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
	return wallpaper.RunDaemonOptions(wallpaper.DaemonOptions{
		Engine:     engine,
		Workspaces: workspacePlan(cfg, spec),
		Transition: wallpaper.NewTransition(cfg.WallpaperTransition, cfg.WallpaperTransitionMS),
		Palette:    ui.PaletteByName(cfg.Theme),
		FPS:        cfg.WallpaperFPS,
	}, nil)
}

// workspacePlan construye el mapa de fondos por workspace; el fondo base es el
// que está configurado para toda la sesión.
func workspacePlan(cfg *config.Config, spec wallpaper.Spec) *wallpaper.WorkspaceMap {
	return wallpaper.NewWorkspaceMap(spec.Source(), cfg.WallpaperWorkspaces)
}

// animatedSpec indica si el fondo lo tiene que dibujar el demonio (todo lo que
// no es una imagen fija: escenas, GIF y vídeo).
func animatedSpec(spec wallpaper.Spec) bool {
	switch spec.Kind {
	case wallpaper.KindScene, wallpaper.KindGIF, wallpaper.KindVideo:
		return true
	}
	return false
}

// syncWallpaperDaemon deja el demonio acorde al fondo y a las reglas por
// workspace. Solo se necesita para fondos animados o mapas por workspace;
// para una imagen fija global basta el `output bg` de sway.
func syncWallpaperDaemon(cfg *config.Config) {
	if !wallpaperDaemonNeeded(cfg) {
		wallpaper.StopDaemon(cfg.Cache)
		return
	}
	if !wallpaper.WaylandReady() {
		fmt.Printf("  %s\n", ui.Dim("sin compositor Wayland: el fondo animado no se puede dibujar desde aquí"))
		return
	}
	if err := runWallpaperDaemon(cfg, nil); err != nil {
		fmt.Printf("  %s\n", ui.Dim("el demonio de fondo no pudo arrancar: "+err.Error()))
		return
	}
	fmt.Printf("  %s\n", ui.Dim("demonio de fondo al día con el nuevo fondo"))
}

// scheduleWallpaperDaemonSync delega el reinicio del demonio a un proceso
// desacoplado: aplicar el fondo no queda bloqueado por la inicialización de
// Wayland, FFmpeg ni la primera decodificación.
func scheduleWallpaperDaemonSync(cfg *config.Config) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if cfg.Cache == "" {
		return fmt.Errorf("la ruta de caché no está configurada")
	}
	if err := os.MkdirAll(cfg.Cache, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(cfg.Cache, "wallpaper-sync.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command(exe, "wallpaper", "--sync-daemon")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// restartWallpaperDaemon aplica los cambios del plan de fondos por workspace.
func restartWallpaperDaemon(cfg *config.Config) {
	if _, err := currentSpec(cfg); err != nil {
		return
	}
	// Con fondos por workspace el demonio es imprescindible aunque el fondo
	// base sea una imagen fija.
	if wallpaper.RunningPID(cfg.Cache) == 0 && !wallpaperDaemonNeeded(cfg) {
		return
	}
	syncWallpaperDaemon(cfg)
}

// wallpaperWorkspaces muestra y edita los fondos por workspace.
func wallpaperWorkspaces(cfg *config.Config, args []string) error {
	spec, err := currentSpec(cfg)
	if err != nil {
		return err
	}
	plan := workspacePlan(cfg, spec)
	action := "list"
	if len(args) > 0 {
		action = strings.ToLower(args[0])
		args = args[1:]
	}
	switch action {
	case "set", "add", "poner":
		if len(args) < 2 {
			return fmt.Errorf("uso: swaydesk wallpaper workspaces set <workspace> <fondo>")
		}
		ws := args[0]
		target, err := resolveSpec(cfg, strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		engine, err := wallpaper.NewEngine(target)
		if err != nil {
			return fmt.Errorf("fondo para workspace %q: %w", ws, err)
		}
		engine.Close()
		if err := plan.Set(ws, target.Source()); err != nil {
			return err
		}
		cfg.WallpaperWorkspaces = plan.Raw()
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✔ %s → %s\n", ui.Style{Fg: ui.PaletteByName(cfg.Theme).Accent}.B().Render(ws),
			ui.Style{Fg: ui.PaletteByName(cfg.Theme).Ok}.Render(target.Source()))
		restartWallpaperDaemon(cfg)
		return nil
	case "unset", "remove", "quitar":
		if len(args) < 1 {
			return fmt.Errorf("uso: swaydesk wallpaper workspaces unset <workspace>")
		}
		if !plan.Unset(args[0]) {
			return fmt.Errorf("el workspace %q no tiene un fondo propio", args[0])
		}
		cfg.WallpaperWorkspaces = plan.Raw()
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✔ %s vuelve al fondo base\n", args[0])
		restartWallpaperDaemon(cfg)
		return nil
	case "clear", "limpiar":
		if plan.Len() == 0 {
			fmt.Println("· no hay fondos por workspace")
			return nil
		}
		plan.Clear()
		cfg.WallpaperWorkspaces = plan.Raw()
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Println("✔ fondos por workspace eliminados")
		restartWallpaperDaemon(cfg)
		return nil
	case "list", "ls", "lista":
	default:
		return fmt.Errorf("órdenes: list, set, unset, clear")
	}

	pal := ui.PaletteByName(cfg.Theme)
	st := ui.Style{Fg: pal.Accent}
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Style{Fg: pal.Accent, Bg: pal.Bg}.B().Render("fondos por workspace"),
		ui.Dim("· "+wallpaper.NewTransition(cfg.WallpaperTransition, cfg.WallpaperTransitionMS).String()))
	fmt.Printf("  %-14s %s\n", ui.Dim("base"), ui.Dim(spec.Source()))
	curKey := ""
	if num, name, ok := wallpaper.FocusedWorkspace(); ok {
		if src, key, found := plan.Lookup(num, name); found {
			curKey = key
			fmt.Printf("  %-14s %s → %s\n", ui.Dim("ahora"), ui.Dim(name), src)
		} else {
			fmt.Printf("  %-14s %s → %s\n", ui.Dim("ahora"), ui.Dim(name), ui.Dim("fondo base"))
		}
	}
	entries := plan.Entries()
	if len(entries) == 0 {
		fmt.Printf("  %s\n", ui.Dim("ningún workspace tiene fondo propio (workspaces set 1 scene:petals)"))
	}
	for _, e := range entries {
		mark := " "
		if e.Key == curKey {
			mark = "›"
		}
		fmt.Printf("  %s %-14s %s\n", st.Render(mark), e.Key, e.Source)
	}
	if pid := wallpaper.RunningPID(cfg.Cache); pid != 0 {
		fmt.Printf("  %-14s %s\n", ui.Dim("demonio"), ui.Style{Fg: pal.Ok}.Render(fmt.Sprintf("activo (pid %d)", pid)))
	} else {
		fmt.Printf("  %-14s %s\n", ui.Dim("demonio"), ui.Style{Fg: pal.Warn}.Render("detenido"))
	}
	fmt.Println()
	return nil
}

// setWallpaperTransition cambia el tipo y la duración de la transición.
func setWallpaperTransition(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		t := wallpaper.NewTransition(cfg.WallpaperTransition, cfg.WallpaperTransitionMS)
		fmt.Printf("transición actual: %s\n", t.String())
		fmt.Printf("tipos: %s\n", strings.Join(wallpaper.TransitionNames(), " · "))
		return nil
	}
	kind := args[0]
	if !wallpaper.ValidTransition(kind) {
		return fmt.Errorf("transición desconocida %q (usa %s)", kind,
			strings.Join(wallpaper.TransitionNames(), ", "))
	}
	ms := cfg.WallpaperTransitionMS
	if len(args) > 1 {
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 || n > 5000 {
			return fmt.Errorf("duración inválida %q (0–5000 ms)", args[1])
		}
		ms = n
	}
	cfg.WallpaperTransition = kind
	cfg.WallpaperTransitionMS = ms
	if err := cfg.Save(); err != nil {
		return err
	}
	t := wallpaper.NewTransition(kind, ms)
	fmt.Printf("✔ transición: %s\n", t.String())
	restartWallpaperDaemon(cfg)
	return nil
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
	fmt.Printf("  %-12s %s\n", ui.Dim("transición"), wallpaper.NewTransition(cfg.WallpaperTransition, cfg.WallpaperTransitionMS).String())
	if n := len(cfg.WallpaperWorkspaces); n > 0 {
		fmt.Printf("  %-12s %d con fondo propio (swaydesk wallpaper workspaces)\n", ui.Dim("workspaces"), n)
	}
	fmt.Printf("  %-12s %s\n", ui.Dim("tipo"), kind)
	fmt.Printf("  %-12s %s\n", ui.Dim("origen"), where)
	if pid := wallpaper.RunningPID(cfg.Cache); pid != 0 {
		fmt.Printf("  %-12s %s\n", ui.Dim("demonio"), ui.Style{Fg: p.Ok}.Render(fmt.Sprintf("activo (pid %d)", pid)))
	} else if animatedSpec(wallpaper.Spec{Kind: wallpaper.Kind(kind)}) || len(cfg.WallpaperWorkspaces) > 0 {
		fmt.Printf("  %-12s %s\n", ui.Dim("demonio"), ui.Style{Fg: p.Warn}.Render("detenido: por eso no se ve el fondo"))
		fmt.Printf("  %-12s %s\n", "", ui.Dim("arráncalo con: swaydesk wallpaper daemon · errores: swaydesk wallpaper logs"))
	} else {
		fmt.Printf("  %-12s %s\n", ui.Dim("demonio"), ui.Dim("no hace falta (imagen fija)"))
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
	if cfg.WallpaperAdaptive && res.Spec.Path != "" {
		_ = adaptThemeToWallpaper(cfg, ui.NewTheme(cfg.Theme, cfg.NerdFont))
	}
	applyStaticFallback(cfg, res.Spec)
	fmt.Printf("✔ fondo aplicado: %s\n", res.Spec.Source())
	if wallpaperDaemonNeeded(cfg) || wallpaper.RunningPID(cfg.Cache) != 0 {
		if err := scheduleWallpaperDaemonSync(cfg); err != nil {
			return fmt.Errorf("fondo guardado, pero no se pudo programar el reinicio del demonio: %w", err)
		}
		fmt.Printf("  %s\n", ui.Dim("animación actualizándose en segundo plano"))
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
