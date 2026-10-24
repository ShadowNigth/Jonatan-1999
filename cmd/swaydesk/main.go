// swaydesk: escritorio para Arch Linux + sway escrito en Go.
//
// Uso: swaydesk <comando> [opciones]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/actions"
	"github.com/ShadowNigth/swaydesk/internal/adaptive"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/dbus"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/gallery"
	"github.com/ShadowNigth/swaydesk/internal/host"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/setup"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// Version es la versión de swaydesk.
const Version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "init", "install":
		cmdInit(args)
	case "apply", "reload-theme":
		cmdApply(args)
	case "theme", "themes":
		cmdTheme(args)
	case "status":
		cmdStatus(args)
	case "doctor", "check":
		cmdDoctor(args)
	case "keys", "atajos", "teclas":
		cmdKeys(args)
	case "notify":
		cmdNotify(args)
	case "volume", "vol":
		cmdVolume(args)
	case "lock":
		must(actions.Lock())
	case "reload":
		must(reloadSway())
	case "power":
		cmdPower(args)
	case "screenshot":
		must(actions.Screenshot(flagValue(args, "region")))
	case "config":
		cmdConfig(args)
	case "wallpaper", "fondo":
		cmdWallpaper(args)
	case "boot", "arranque", "splash":
		cmdBoot(args)
	case "opacity", "opacidad", "transparencia":
		cmdOpacity(args)
	case "bar", "barra":
		cmdBar(args)
	case "notifications", "notificaciones":
		cmdNotifications(args)
	case "notify-enabled", "notificaciones-activas":
		// Para scripts: código 0 si el daemon propio está activado.
		if cfg, err := config.Load(); err == nil && cfg.NotifyEnabled {
			return
		}
		os.Exit(1)
	case "osd", "avisos":
		cmdOSD(args)
	case "osd-enabled", "avisos-activos":
		// Para el autostart: basta con que esté activo el OSD de volumen o MPRIS.
		if cfg, err := config.Load(); err == nil && (cfg.OSDEnabled || cfg.OSDMediaEnabled) {
			return
		}
		os.Exit(1)
	case "version", "-v", "--version":
		fmt.Printf("swaydesk %s (go %s, %s/%s)\n", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "swaydesk: comando desconocido %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`swaydesk · escritorio para Arch Linux + sway, escrito en Go

Uso:
  swaydesk <comando> [opciones]

Configuración
  init [--force] [--only sway|waybar|foot|mako|swaylock]
        Escribe la configuración del escritorio en ~/.config
  apply          Regenera los archivos del tema activo y recarga waybar/sway
  theme [nombre]              Lista las paletas o cambia la activa
  theme --gallery             Galería interactiva de temas (con vista previa)
  theme --from-wallpaper      Deriva un tema a partir de los colores del fondo
  config                      Muestra las rutas y los valores de configuración

Sesión
  status [--json|--module M]  Estado del escritorio (sway, CPU, RAM, batería...)
  doctor                      Diagnóstico de dependencias y configuración
  keys [filtro] [--exec]      Atajos de sway expandidos y cuáles no funcionan
  notify <título> [cuerpo]    Envía una notificación de escritorio
  notifications [status|dnd|history|clear|preview|test]
                              Daemon de notificaciones propio (swaynotify)
  volume <up|down|mute>       Controla el volumen (con aviso en pantalla)
  osd [show|preview|volume]   Avisos en pantalla (swayosd): volumen
  screenshot [region|full]    Captura de pantalla con grim
  wallpaper <fondo> [modo]    Fondo estático o animado (ver "swaydesk wallpaper help")
  wallpaper --gallery         Selector de fondos con vista previa animada
  wallpaper daemon | --stop   Inicia/detiene el fondo animado (live wallpaper)
  wallpaper workspaces ...    Un fondo distinto por workspace, con transición
  wallpaper --scenes [--json] Escenas animadas disponibles
  boot [--style=s] [--duration=ms]  Animación de arranque de la sesión
  opacity [list|presets|add|remove|all]  Translucidez por aplicación
  bar [status|on|off|preview|modules|opacity] Barra propia y translucidez
  lock                        Bloquea la pantalla
  reload                      Recarga la configuración de sway
  power [acción]              Menú de energía (lock, suspend, logout, reboot, poweroff)
  version                     Muestra la versión

Ejemplos:
  swaydesk init --force && swaymsg reload
  swaydesk theme gruvbox
  swaydesk theme --gallery
  swaydesk wallpaper set scene:petals && swaydesk wallpaper daemon
  swaydesk wallpaper set ~/Imágenes/lofi.gif --live
  swaydesk boot --style=neon --duration=2000
  swaydesk status --module clock

Atajos por defecto (Super = Mod4):
  Super+Return  terminal        Super+d       lanzador
  Super+p       energía         Super+t       temas
  Super+c       portapapeles    Super+i       panel de control
  Super+Shift+l bloquear        Super+q       cerrar ventana
  Super+r       redimensionar    Super+w       pestañas
  Super+Shift+w fondos           Super+Shift+t galería de temas
`)
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		os.Exit(1)
	}
}

// optValue devuelve el valor de --nombre valor o --nombre=valor.
func optValue(args []string, name string) string {
	long := "--" + name
	for i, a := range args {
		if strings.HasPrefix(a, long+"=") {
			return a[len(long)+1:]
		}
		if a == long && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func flagValue(args []string, def string) string {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0]
	}
	return def
}

// relaunchInTerminal relanza swaydesk dentro de una ventana flotante cuando no
// hay terminal interactiva (por ejemplo, al invocarlo desde sway).
func relaunchInTerminal(cfg *config.Config, appID string, w, h int, args []string) bool {
	if ui.IsInteractive() {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		must(err)
	}
	if err := launcher.LaunchTerminal(cfg.Terminal, appID, "swaydesk", w, h, append([]string{exe}, args...)); err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		return true
	}
	return true
}

// configDir devuelve ~/.config o $XDG_CONFIG_HOME.
func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// ───────────────────────── init ─────────────────────────

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "sobrescribe archivos existentes (con respaldo)")
	only := fs.String("only", "", "solo estos componentes, separados por comas")
	dest := fs.String("dest", configDir(), "directorio de configuración destino")
	dry := fs.Bool("dry-run", false, "muestra lo que haría sin escribir")
	_ = fs.Parse(args)

	cfg, err := config.Load()
	must(err)
	var onlyList []string
	for _, o := range strings.Split(*only, ",") {
		if o = strings.TrimSpace(o); o != "" {
			onlyList = append(onlyList, o)
		}
	}

	if *dry {
		files, err := setup.Render(cfg)
		must(err)
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Printf("Se escribirían %d archivos en %s:\n", len(names), *dest)
		for _, n := range names {
			fmt.Printf("  %s\n", filepath.Join(*dest, n))
		}
		return
	}

	results, err := setup.Install(cfg, *dest, *force, onlyList)
	must(err)

	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	ok := ui.Style{Fg: th.Palette.Ok}.Render("✔")
	warn := ui.Style{Fg: th.Palette.Warn}.Render("•")
	fmt.Printf("swaydesk · instalando configuración en %s\n\n", *dest)
	for _, r := range results {
		rel, _ := filepath.Rel(*dest, r.Path)
		mark := ok
		switch {
		case strings.HasPrefix(r.Action, "omitido"):
			mark = warn
		case r.Action == "respaldo":
			mark = ui.Style{Fg: th.Palette.Accent}.Render("↩")
		}
		fmt.Printf("  %s %-52s %s\n", mark, rel, ui.Dim(r.Action))
	}
	fmt.Println()
	if !setup.HasNerdFont() {
		fmt.Printf("%s No se detectó ninguna Nerd Font: las barras usarán símbolos\n",
			ui.Style{Fg: th.Palette.Warn}.Render("aviso:"))
		fmt.Println("        alternativos. Instala: sudo pacman -S ttf-jetbrains-mono-nerd")
	}
	fmt.Println("Siguiente paso:")
	fmt.Println("  swaymsg reload          (o cierra sesión y vuelve a entrar en sway)")
	fmt.Println("  swaydesk doctor         (para comprobar dependencias)")
}

// ───────────────────────── apply ─────────────────────────

func cmdApply(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	dest := fs.String("dest", configDir(), "directorio de configuración")
	_ = fs.Parse(args)

	cfg, err := config.Load()
	must(err)
	results, err := setup.Apply(cfg, *dest)
	must(err)
	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	fmt.Printf("%s tema %s\n", ui.Style{Fg: th.Palette.Ok}.Render("✔"), th.Palette.Name)
	for _, r := range results {
		fmt.Printf("  %s actualizado\n", ui.Dim(r.Path))
	}
	if len(results) == 0 {
		fmt.Println(ui.Dim("  no hay archivos generados en " + *dest + " (ejecuta swaydesk init)"))
	}
}

// ───────────────────────── theme ─────────────────────────

func cmdTheme(args []string) {
	cfg, err := config.Load()
	must(err)
	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	// Subcomandos nuevos: galería interactiva y tema derivado del fondo.
	for _, a := range args {
		switch a {
		case "--from-wallpaper", "--wallpaper", "-w":
			must(themeFromWallpaper(cfg, th))
			return
		case "--gallery", "-g", "--pick":
			must(themeGallery(cfg, th))
			return
		case "--json":
			printThemesJSON()
			return
		}
	}
	if len(args) == 0 {
		printThemeList(cfg, th)
		return
	}
	name := strings.ToLower(args[0])
	if !ui.IsPalette(name) {
		fmt.Fprintf(os.Stderr, "swaydesk: paleta desconocida %q\n", name)
		fmt.Fprintf(os.Stderr, "disponibles: %s\n", strings.Join(ui.PaletteNames(), ", "))
		os.Exit(1)
	}
	p := ui.PaletteByName(name)
	must(applyTheme(cfg, name))
	fmt.Printf("%s Tema cambiado a %s %s\n", ui.Style{Fg: th.Palette.Ok}.Render("✔"),
		ui.Style{Fg: ui.PaletteByName(name).Accent, Bg: ui.PaletteByName(name).Bg}.B().Render(p.Name),
		ui.Dim("· "+p.Category))
}

// applyTheme guarda el tema y regenera todos los archivos del escritorio.
func applyTheme(cfg *config.Config, name string) error {
	if err := cfg.SaveTheme(name); err != nil {
		return err
	}
	if _, err := setup.Apply(cfg, configDir()); err != nil {
		return err
	}
	// Las escenas usan los colores de la paleta; GIF/vídeo y fondos por
	// workspace también dependen del demonio. Reiniciarlo de forma separada
	// conserva el fondo y evita dejarlo atado al proceso `swaydesk theme`.
	if themeWallpaperNeedsDaemon(cfg) && wallpaper.WaylandReady() {
		if err := runWallpaperDaemon(cfg, nil); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk: tema aplicado, pero no se pudo reanudar el fondo: %v\n", err)
		}
	}
	return nil
}

func themeWallpaperNeedsDaemon(cfg *config.Config) bool {
	return wallpaperDaemonNeeded(cfg)
}

func printThemeList(cfg *config.Config, th *ui.Theme) {
	fmt.Println(ui.Bold("Paletas disponibles:"))
	for _, cat := range ui.PaletteCategories() {
		fmt.Printf("\n  %s\n", ui.Style{Fg: th.Palette.Accent}.B().Render(strings.ToUpper(cat)))
		for _, meta := range ui.PaletteList(cat) {
			name := meta.Key
			p := ui.PaletteByName(name)
			mark := " "
			if name == cfg.Theme {
				mark = ui.Style{Fg: p.Ok}.Render("●")
			}
			swatch := ""
			for _, c := range []ui.Color{p.Bg, p.BgAlt, p.BgSel, p.Accent, p.Accent2, p.Ok, p.Warn, p.Err} {
				swatch += ui.Style{Fg: c, Bg: c}.Render("  ")
			}
			scene := ""
			if p.Scene != "" {
				scene = ui.Dim("◍ " + p.Scene)
			}
			fmt.Printf(" %s %s %s %s %s\n", mark, ui.PadRight(name, 13), ui.PadRight(p.Name, 18), swatch, scene)
		}
	}
	fmt.Println()
	fmt.Println(ui.Dim("Cambiar:   swaydesk theme <nombre>"))
	fmt.Println(ui.Dim("Galería:   swaydesk theme --gallery"))
	fmt.Println(ui.Dim("Adaptativo: swaydesk theme --from-wallpaper"))
}

func printThemesJSON() {
	type entry struct {
		Key      string `json:"key"`
		Name     string `json:"name"`
		Category string `json:"category"`
		Scene    string `json:"scene,omitempty"`
		Light    bool   `json:"light"`
		Adaptive bool   `json:"adaptive,omitempty"`
	}
	var out []entry
	for _, name := range ui.PaletteNames() {
		p := ui.PaletteByName(name)
		out = append(out, entry{name, p.Name, p.Category, p.Scene, p.Light, p.Adaptive})
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))
}

func themeGallery(cfg *config.Config, th *ui.Theme) error {
	if relaunchInTerminal(cfg, "swaydesk-themes", 92, 32, []string{"theme", "--gallery"}) {
		return nil
	}
	res, err := gallery.ThemeGallery(cfg.Theme, cfg.NerdFont)
	if err != nil {
		return err
	}
	if !res.Applied {
		return nil
	}
	if err := applyTheme(cfg, res.Key); err != nil {
		return err
	}
	fmt.Printf("%s Tema cambiado a %s\n", ui.Style{Fg: th.Palette.Ok}.Render("✔"),
		ui.Style{Fg: ui.PaletteByName(res.Key).Accent}.B().Render(ui.PaletteByName(res.Key).Name))
	return nil
}

// themeFromWallpaper deriva un tema de los colores dominantes del fondo.
func themeFromWallpaper(cfg *config.Config, th *ui.Theme) error {
	path := cfg.WallpaperPath
	if path == "" {
		fmt.Println("No hay una imagen de fondo configurada.")
		fmt.Printf("  %s\n", ui.Dim("Prueba: swaydesk wallpaper set ~/Imágenes/fondo.png --scenes"))
		return nil
	}
	pal, err := adaptive.FromFile(path, "")
	if err != nil {
		return err
	}
	ui.RegisterPalette("adaptativo", pal)
	if err := adaptive.Save(pal, filepath.Join(cfg.Dir, "themes", "adaptativo.toml")); err != nil {
		return err
	}
	if err := applyTheme(cfg, "adaptativo"); err != nil {
		return err
	}
	fmt.Printf("%s Tema adaptativo desde %s\n", ui.Style{Fg: th.Palette.Ok}.Render("✔"), filepath.Base(path))
	fmt.Printf("  %s\n", gallery.DescribeTheme(pal))
	return nil
}

// ───────────────────────── status ─────────────────────────

// statusData es la estructura que se serializa con --json.
type statusData struct {
	Host      string     `json:"host"`
	User      string     `json:"user"`
	Distro    string     `json:"distro"`
	Theme     string     `json:"theme"`
	Uptime    string     `json:"uptime"`
	Sway      *swayInfo  `json:"sway,omitempty"`
	System    systemInfo `json:"system"`
	Timestamp time.Time  `json:"timestamp"`
}

type swayInfo struct {
	Version    string   `json:"version"`
	Config     string   `json:"config"`
	Mode       string   `json:"mode"`
	Focused    string   `json:"focused"`
	AppID      string   `json:"app_id"`
	Layout     string   `json:"layout"`
	Outputs    []string `json:"outputs"`
	Workspaces []string `json:"workspaces"`
	Windows    int      `json:"windows"`
}

type systemInfo struct {
	CPU      float64    `json:"cpu_percent"`
	MemUsed  uint64     `json:"mem_used"`
	MemTotal uint64     `json:"mem_total"`
	DiskUsed uint64     `json:"disk_used"`
	DiskTot  uint64     `json:"disk_total"`
	Temp     float64    `json:"temp_c"`
	LoadAvg  [3]float64 `json:"load_avg"`
	Volume   int        `json:"volume"`
	Muted    bool       `json:"muted"`
	Wifi     string     `json:"wifi,omitempty"`
	Signal   int        `json:"wifi_signal,omitempty"`
	Battery  *battery   `json:"battery,omitempty"`
}

type battery struct {
	Percent  int    `json:"percent"`
	Status   string `json:"status"`
	TimeLeft string `json:"time_left,omitempty"`
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "salida en JSON")
	module := fs.String("module", "", "salida de una sola línea para waybar")
	_ = fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
	}
	data := collectStatus(cfg)

	switch {
	case *module != "":
		fmt.Print(moduleOutput(*module, &data, cfg))
	case *asJSON:
		out, err := json.MarshalIndent(data, "", "  ")
		must(err)
		fmt.Println(string(out))
	default:
		printStatus(&data, cfg)
	}
}

func collectStatus(cfg *config.Config) statusData {
	col := host.NewCollector()
	s := col.Snapshot()
	data := statusData{
		Host: host.Hostname(), User: host.User(), Distro: host.Distro(),
		Theme:  ui.PaletteByName(cfg.Theme).Name,
		Uptime: formatDuration(s.Uptime),
	}
	data.System = systemInfo{
		CPU: s.CPU, MemUsed: s.MemUsed, MemTotal: s.MemTotal,
		DiskUsed: s.DiskUsed, DiskTot: s.DiskTotal, Temp: s.Temp,
		LoadAvg: s.LoadAvg, Volume: s.Volume, Muted: s.Muted,
		Wifi: s.WifiName, Signal: s.WifiSignal,
	}
	_ = cfg
	if s.Battery != nil && s.Battery.Present {
		data.System.Battery = &battery{Percent: s.Battery.Percent, Status: s.Battery.Status}
		if s.Battery.TimeLeft > 0 {
			data.System.Battery.TimeLeft = formatDuration(s.Battery.TimeLeft)
		}
	}
	data.Sway = collectSway()
	return data
}

func collectSway() *swayInfo {
	c, err := swayipc.Dial()
	if err != nil {
		return nil
	}
	defer c.Close()
	info := &swayInfo{}
	if v, err := c.Version(); err == nil {
		info.Version = v.HumanReadable
		info.Config = v.LoadedConfig
	}
	if tree, err := c.Tree(); err == nil {
		if f := tree.FindFocused(); f != nil {
			info.Focused = f.Title()
			info.AppID = f.App()
			info.Layout = f.Layout
		}
		info.Windows = len(tree.Leaves())
	}
	if ws, err := c.Workspaces(); err == nil {
		for _, w := range ws {
			mark := " "
			if w.Focused {
				mark = "*"
			}
			info.Workspaces = append(info.Workspaces, fmt.Sprintf("%s%s (%s)", mark, w.Name, w.Output))
		}
	}
	if outs, err := c.Outputs(); err == nil {
		for _, o := range outs {
			if o.Active {
				info.Outputs = append(info.Outputs, fmt.Sprintf("%s %dx%d", o.Name, o.Rect.Width, o.Rect.Height))
			}
		}
	}
	return info
}

func printStatus(d *statusData, cfg *config.Config) {
	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	p := th.Palette
	title := ui.Style{Fg: p.Accent, Bg: p.Bg}.B()
	label := ui.Style{Fg: p.Muted, Bg: p.Bg}

	fmt.Println()
	fmt.Printf("  %s %s\n", title.Render("swaydesk"), ui.Dim("· "+d.Host))
	fmt.Printf("  %s\n", ui.Dim(d.Distro+" · "+d.User+" · en marcha desde hace "+d.Uptime))
	fmt.Println()

	row := func(name, value string) {
		fmt.Printf("  %-14s %s\n", label.Render(name), value)
	}

	if d.Sway != nil {
		fmt.Printf("  %s\n", label.Render("sway"))
		row("  versión", d.Sway.Version)
		row("  sesión", strings.Join(d.Sway.Outputs, " · "))
		row("  ventanas", fmt.Sprintf("%d abiertas", d.Sway.Windows))
		row("  enfocada", fmt.Sprintf("%s %s", d.Sway.Focused, ui.Dim("("+d.Sway.AppID+")")))
		row("  workspaces", strings.Join(d.Sway.Workspaces, "  "))
		fmt.Println()
	} else {
		fmt.Printf("  %s\n\n", ui.Style{Fg: p.Warn}.Render("  sway no está en ejecución (o SWAYSOCK no es accesible)"))
	}

	fmt.Printf("  %s\n", label.Render("sistema"))
	row("  CPU", bar(d.System.CPU, 100, 24, th, colorFor(d.System.CPU, 70, 90))+
		fmt.Sprintf(" %4.1f%%  %s", d.System.CPU, ui.Dim(fmt.Sprintf("carga %.2f %.2f %.2f", d.System.LoadAvg[0], d.System.LoadAvg[1], d.System.LoadAvg[2]))))
	memPct := percent(d.System.MemUsed, d.System.MemTotal)
	row("  memoria", bar(memPct, 100, 24, th, colorFor(memPct, 70, 90))+
		fmt.Sprintf(" %s", ui.Dim(fmt.Sprintf("%s / %s", humanBytes(d.System.MemUsed), humanBytes(d.System.MemTotal)))))
	diskPct := percent(d.System.DiskUsed, d.System.DiskTot)
	row("  disco /", bar(diskPct, 100, 24, th, colorFor(diskPct, 80, 95))+
		fmt.Sprintf(" %s", ui.Dim(fmt.Sprintf("%s / %s", humanBytes(d.System.DiskUsed), humanBytes(d.System.DiskTot)))))
	if d.System.Temp > 0 {
		row("  temperatura", fmt.Sprintf("%.1f °C", d.System.Temp))
	}
	if d.System.Battery != nil {
		b := d.System.Battery
		extra := b.Status
		if b.TimeLeft != "" {
			extra += " · " + b.TimeLeft
		}
		row("  batería", bar(float64(b.Percent), 100, 24, th, colorFor(float64(b.Percent), 101, 20))+
			fmt.Sprintf(" %d%% %s", b.Percent, ui.Dim(extra)))
	}
	vol := fmt.Sprintf("%d%%", d.System.Volume)
	if d.System.Muted {
		vol = "silencio"
	}
	row("  audio", vol)
	dst := ui.Dim("sin conexión")
	if d.System.Wifi != "" {
		dst = d.System.Wifi
		if d.System.Signal > 0 {
			dst += fmt.Sprintf(" (%d%%)", d.System.Signal)
		}
	}
	row("  red", dst)
	fmt.Println()
	fmt.Printf("  %s %s\n\n", label.Render("tema"), d.Theme)

	kind := cfg.WallpaperKind
	if kind == "" {
		kind = "scene"
	}
	what := cfg.WallpaperName
	if what == "" {
		what = cfg.WallpaperPath
	}
	if what == "" {
		what = "(según el tema)"
	}
	daemon := "detenido"
	if wallpaper.RunningPID(cfg.Cache) != 0 {
		daemon = "activo"
	}
	fmt.Printf("  %s %s %s\n", label.Render("fondo"), ui.Style{Fg: p.Accent}.Render(kind+":"+what),
		ui.Dim("· demonio "+daemon))
	fmt.Println()
}

func moduleOutput(name string, d *statusData, cfg *config.Config) string {
	ic := ui.IconsFor(cfg.NerdFont)
	switch name {
	case "logo":
		return ic.Workspace + " swaydesk"
	case "recording":
		if processRunning("wf-recorder") || processRunning("wl-screenrec") {
			return ic.Dot + " REC"
		}
		return ""
	case "clock":
		return time.Now().Format("15:04")
	case "cpu":
		return fmt.Sprintf("%s %.0f%%", ic.CPU, d.System.CPU)
	case "ram":
		return fmt.Sprintf("%s %.0f%%", ic.RAM, percent(d.System.MemUsed, d.System.MemTotal))
	case "battery":
		if d.System.Battery == nil {
			return ""
		}
		return fmt.Sprintf("%s %d%%", ic.Battery, d.System.Battery.Percent)
	case "volume":
		if d.System.Muted {
			return ic.Muted
		}
		return fmt.Sprintf("%s %d%%", ic.Volume, d.System.Volume)
	case "uptime":
		return d.Uptime
	case "sway":
		if d.Sway == nil {
			return "sin sway"
		}
		return fmt.Sprintf("%d ventanas", d.Sway.Windows)
	}
	return ""
}

func processRunning(name string) bool {
	if !launcher.HasCommand("pgrep") {
		return false
	}
	return exec.Command("pgrep", "-x", name).Run() == nil
}

func autostartStatus(cfg *config.Config) (bool, string) {
	swayPath := filepath.Join(configDir(), "sway", "config")
	swayConfig, err := os.ReadFile(swayPath)
	if err != nil || !strings.Contains(string(swayConfig), "autostart.sh") {
		return false, "la configuración Sway no invoca swaydesk/autostart.sh"
	}
	path := filepath.Join(configDir(), "swaydesk", "autostart.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "no se pudo leer " + path
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, "no se pudo consultar " + path
	}
	if info.Mode().Perm()&0o111 == 0 {
		return false, "autostart.sh no es ejecutable"
	}
	checks := []struct{ label, marker string }{
		{"animación de arranque", "swaydesk boot"},
		{"notificaciones", "swaynotify daemon"},
		{"OSD", "swayosd watch"},
		{"fondos por workspace", "swaydesk wallpaper daemon"},
		{"reintentos de inicio de barra", "start_bar waybar"},
		{"reintentos de inicio de swaybar", "start_bar swaybar"},
	}
	var missing []string
	for _, check := range checks {
		if !strings.Contains(string(data), check.marker) {
			missing = append(missing, check.label)
		}
	}
	if len(missing) > 0 {
		return false, "autostart antiguo; faltan: " + strings.Join(missing, ", ")
	}
	return true, path + " · registro: " + filepath.Join(cfg.Cache, "autostart.log")
}

func notifyDaemonStatus(cfg *config.Config) (bool, string) {
	if cfg.NotifyEnabled && launcher.HasCommand("swaynotify") && hasNotifyDaemon() {
		return true, "swaynotify activo y registrado en D-Bus"
	}
	if processRunning("mako") {
		return true, "mako activo (reserva)"
	}
	if !cfg.NotifyEnabled {
		return false, "notify.enabled = false y no hay mako activo"
	}
	if !launcher.HasCommand("swaynotify") {
		return false, "falta swaynotify y no hay mako activo"
	}
	return false, "swaynotify no responde; comprueba D-Bus y el registro de autostart"
}

func osdWatchStatus(cfg *config.Config) (bool, string) {
	if !cfg.OSDEnabled && !cfg.OSDMediaEnabled {
		return false, "osd.enabled y osd.media_enabled están desactivados"
	}
	if !launcher.HasCommand("swayosd") {
		return false, "falta el binario swayosd"
	}
	if !processRunning("swayosd") {
		return false, "no está activo swayosd watch"
	}
	return true, fmt.Sprintf("watch activo · volumen=%v · MPRIS=%v", cfg.OSDEnabled, cfg.OSDMediaEnabled)
}

// ───────────────────────── doctor ─────────────────────────

type check struct {
	name     string
	ok       bool
	required bool
	detail   string
	fix      string
}

// wallpaperDaemonNeeded indica si el fondo actual lo tiene que dibujar el
// demonio (escenas, GIF, vídeo o fondos por workspace).
func wallpaperDaemonNeeded(cfg *config.Config) bool {
	kind := cfg.WallpaperKind
	if kind == "" {
		kind = "scene"
	}
	switch kind {
	case "scene", "gif", "video":
		return true
	}
	return len(cfg.WallpaperWorkspaces) > 0
}

func wallpaperDaemonOK(cfg *config.Config) bool {
	if !wallpaperDaemonNeeded(cfg) {
		return true
	}
	return wallpaper.RunningPID(cfg.Cache) != 0
}

func wallpaperDaemonDetail(cfg *config.Config) string {
	rules := len(cfg.WallpaperWorkspaces)
	if !wallpaperDaemonNeeded(cfg) {
		if rules == 0 {
			return "imagen fija global · no hay reglas por workspace"
		}
	}
	if pid := wallpaper.RunningPID(cfg.Cache); pid != 0 {
		return fmt.Sprintf("activo (pid %d) · %d regla(s) por workspace · registro: %s", pid, rules, filepath.Join(cfg.Cache, "wallpaper.log"))
	}
	if rules > 0 {
		return fmt.Sprintf("detenido con %d regla(s) por workspace", rules)
	}
	return "detenido: el fondo animado no se dibuja"
}

// binInPath indica si el binario se encuentra por el PATH actual.
func binInPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// pathDetail describe dónde se encontraron los binarios (o dónde deberían
// estar si no están en el PATH).
func pathDetail() string {
	if p, err := exec.LookPath("swaydesk"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		cand := filepath.Join(home, ".local", "bin", "swaydesk")
		if fileExists(cand) {
			return cand + " (falta en el PATH)"
		}
	}
	return "swaydesk no está en el PATH"
}

// cmdKeys lista los atajos de sway ya expandidos y señala los que no pueden
// funcionar (tecla repetida o programa inexistente). Es el diagnóstico rápido
// cuando «el atajo no hace nada»: sway falla en silencio.
func cmdKeys(args []string) {
	cfg, err := config.Load()
	must(err)
	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	p := th.Palette

	live := func() (string, error) {
		c, err := swayipc.Dial()
		if err != nil {
			return "", err
		}
		defer c.Close()
		return c.Config()
	}
	text, origin := setup.BindingSource(live)
	if text == "" {
		fmt.Fprintf(os.Stderr, "swaydesk: no se encontró la configuración de sway (%s)\n", origin)
		os.Exit(1)
	}
	// ¿La sesión va por detrás de lo que hay en disco? (editar sin recargar)
	if strings.Contains(origin, "vivo") {
		if disk, err := os.ReadFile(filepath.Join(configDir(), "sway", "config")); err == nil &&
			strings.TrimSpace(string(disk)) != strings.TrimSpace(text) {
			fmt.Printf("\n  %s\n", ui.Style{Fg: p.Warn}.Render("• sway sigue con la configuración anterior: ejecuta  swaymsg reload"))
		}
	}
	binds := setup.ParseBindings(text)
	bad := setup.CheckBindings(binds)

	filter := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			filter = strings.ToLower(a)
		}
	}
	execOnly := hasFlagArg(args, "exec")
	show := make([]setup.Binding, 0, len(binds))
	for _, b := range binds {
		if filter != "" && !strings.Contains(strings.ToLower(b.Sym+" "+b.Command), filter) {
			continue
		}
		if execOnly && b.Exec == "" {
			continue
		}
		show = append(show, b)
	}
	if !hasFlagArg(args, "sin-ordenar") && !hasFlagArg(args, "unsorted") {
		setup.SortBindings(show)
	}

	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("atajos de sway"), ui.Dim("· "+origin))
	fmt.Printf("  %d atajos · %d con problemas\n\n", len(binds), bad)

	ok := ui.Style{Fg: p.Ok}.Render("✔")
	mal := ui.Style{Fg: p.Err}.Render("✘")
	for _, b := range show {
		mark := ok
		if b.Error != "" {
			mark = mal
		}
		etiqueta := b.Sym
		if b.Mode != "" && b.Mode != "default" {
			etiqueta = b.Sym + " [" + b.Mode + "]"
		}
		sym := ui.Style{Fg: p.Accent}.Render(padRight(etiqueta, 26))
		cmd := b.Command
		if b.Exec != "" {
			cmd = ui.Dim(fmt.Sprintf("%s %s", b.Exec, strings.Join(b.Args, " ")))
		}
		fmt.Printf("  %s %s %s\n", mark, sym, ui.Truncate(cmd, 96))
		if b.Error != "" {
			fmt.Printf("      %s %s\n", ui.Style{Fg: p.Warn}.Render("→"), ui.Dim(b.Error))
		}
	}
	if len(show) == 0 {
		fmt.Printf("  %s\n", ui.Dim("ningún atajo coincide con el filtro"))
	}
	if bad > 0 {
		// Programas que faltan y teclas repetidas, en dos listas cortas.
		faltan := map[string]bool{}
		repetidas := 0
		for _, b := range binds {
			if b.Missing != "" {
				faltan[b.Missing] = true
			} else if strings.Contains(b.Error, "repetida") {
				repetidas++
			}
		}
		if len(faltan) > 0 {
			nombres := make([]string, 0, len(faltan))
			for p := range faltan {
				nombres = append(nombres, p)
			}
			sort.Strings(nombres)
			fmt.Printf("\n  %s %s\n", ui.Style{Fg: p.Warn}.Render("programas que faltan:"), strings.Join(nombres, ", "))
			fmt.Printf("  %s\n", ui.Dim("instálalos (p. ej. sudo pacman -S "+strings.Split(nombres[0], " ")[0]+") o cambia la app con: swaydesk config"))
		}
		if repetidas > 0 {
			fmt.Printf("  %s %d atajos comparten tecla: deliberado en un override o sobra uno\n",
				ui.Style{Fg: p.Warn}.Render("teclas repetidas:"), repetidas)
		}
		fmt.Printf("\n  %s\n", ui.Dim("regenera la configuración con: swaydesk init --force --only sway   ·   recarga: swaymsg reload"))
	}
	fmt.Println()
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// footIssueDetail describe el problema del foot.ini instalado, si lo hay.
func footIssueDetail() string {
	if issue := setup.FootConfigIssue(); issue != "" {
		return issue
	}
	return "colores y secciones al día"
}

// swayBindDetail avisa de teclas asignadas dos veces en la configuración.
func swayBindDetail() string {
	if tecla := setup.SwayDuplicateBinding(); tecla != "" {
		return tecla + " está asignada dos veces"
	}
	return "sin teclas repetidas"
}

func cmdDoctor(args []string) {
	cfg, err := config.Load()
	must(err)
	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	autostartOK, autostartDetail := autostartStatus(cfg)
	notifyOK, notifyDetail := notifyDaemonStatus(cfg)
	osdOK, osdDetail := osdWatchStatus(cfg)

	checks := []check{
		{name: "sway", required: true, ok: launcher.HasCommand("sway") || swayipc.SwaySock() != "",
			detail: "compositor", fix: "sudo pacman -S sway"},
		{name: "socket IPC de sway", required: true, ok: swayipc.SwaySock() != "",
			detail: swayipc.SwaySock(), fix: "exporta SWAYSOCK dentro de la sesión de sway"},
		{name: "autostart swaydesk", required: false, ok: autostartOK,
			detail: autostartDetail, fix: "swaydesk init --force --only swaydesk/autostart.sh (crea respaldo)"},
		{name: "animación de arranque", required: false, ok: cfg.BootEnabled,
			detail: fmt.Sprintf("boot.enabled = %v", cfg.BootEnabled), fix: "actívala en [boot] de config.toml"},
		{name: "waybar", required: false, ok: launcher.HasCommand("waybar"),
			detail: "barra de estado", fix: "sudo pacman -S waybar"},
		{name: "foot", required: false, ok: launcher.HasCommand("foot"),
			detail: "terminal", fix: "sudo pacman -S foot"},
		{name: "mako", required: false, ok: launcher.HasCommand("mako"),
			detail: "notificaciones", fix: "sudo pacman -S mako"},
		{name: "swaylock", required: false, ok: launcher.HasCommand("swaylock"),
			detail: "bloqueo de pantalla", fix: "sudo pacman -S swaylock"},
		{name: "grim", required: false, ok: launcher.HasCommand("grim"),
			detail: "capturas de pantalla", fix: "sudo pacman -S grim slurp"},
		{name: "slurp", required: false, ok: launcher.HasCommand("slurp"),
			detail: "selección de región", fix: "sudo pacman -S slurp"},
		{name: "wl-clipboard", required: false, ok: launcher.HasCommand("wl-copy"),
			detail: "portapapeles", fix: "sudo pacman -S wl-clipboard"},
		{name: "pavucontrol/wpctl", required: false, ok: launcher.HasCommand("wpctl") || launcher.HasCommand("pactl"),
			detail: "audio", fix: "sudo pacman -S pipewire wireplumber"},
		{name: "swayidle", required: false, ok: launcher.HasCommand("swayidle"),
			detail: "suspensión por inactividad", fix: "sudo pacman -S swayidle"},
		{name: "Nerd Font", required: false, ok: setup.HasNerdFont(),
			detail: "iconos de la barra", fix: "sudo pacman -S ttf-jetbrains-mono-nerd"},
		{name: "portal wlr", required: false, ok: launcher.HasCommand("xdg-desktop-portal-wlr") ||
			fileExists("/usr/lib/xdg-desktop-portal-wlr"),
			detail: "compartir pantalla", fix: "sudo pacman -S xdg-desktop-portal-wlr"},
		{name: "ffmpeg", required: false, ok: launcher.HasCommand("ffmpeg"),
			detail: "fondos de vídeo (mp4, webm)", fix: "sudo pacman -S ffmpeg"},
		{name: "wlr-layer-shell", required: false, ok: wallpaper.DetectDisplayServer().LayerShell,
			detail: "fondo animado detrás de las ventanas", fix: "usa sway (incluye wlr-layer-shell)"},
		{name: "demonio de fondo", required: false, ok: wallpaperDaemonOK(cfg),
			detail: wallpaperDaemonDetail(cfg),
			fix:    "swaydesk wallpaper daemon · si falla: swaydesk wallpaper logs"},
		{name: "reglas por workspace", required: false, ok: len(cfg.WallpaperWorkspaces) > 0,
			detail: fmt.Sprintf("%d regla(s) guardadas en config.toml", len(cfg.WallpaperWorkspaces)),
			fix:    "swaydesk wallpaper workspaces set <workspace> <fondo>"},
		{name: "swaybar (propia)", required: false, ok: launcher.HasCommand("swaybar") && cfg.BarEnabled,
			detail: "barra de estado de swaydesk", fix: "make build && make install"},
		{name: "fuentes de consola", required: false, ok: font.Available(),
			detail: "texto de swaybar (PSF)", fix: "sudo pacman -S kbd"},
		{name: "notificaciones", required: false,
			ok: notifyOK, detail: notifyDetail,
			fix: "revisa D-Bus y ~/.cache/swaydesk/autostart.log"},
		{name: "OSD volumen/MPRIS", required: false,
			ok: osdOK, detail: osdDetail,
			fix: "swaydesk osd show · revisa ~/.cache/swaydesk/autostart.log"},
		{name: "bus de sesión (D-Bus)", required: false, ok: dbus.SessionAvailable(),
			detail: "necesario para las notificaciones", fix: "arranca la sesión con dbus"},
		{name: "swayfx", required: false, ok: setup.IsSwayfx(),
			detail: "desenfoque y esquinas redondeadas", fix: "opcional: swayfx en el AUR"},
		{name: "PATH", required: false, ok: binInPath("swaydesk"),
			detail: pathDetail(), fix: "añade ~/.local/bin al PATH (o reejecuta scripts/install.sh)"},
		{name: "foot.ini", required: false, ok: setup.FootConfigIssue() == "",
			detail: footIssueDetail(), fix: "swaydesk init --force --only foot"},
		{name: "atajos de sway", required: false, ok: setup.SwayDuplicateBinding() == "",
			detail: swayBindDetail(), fix: "swaydesk init --force --only sway"},
	}

	okMark := ui.Style{Fg: th.Palette.Ok}.B().Render("✔")
	badMark := ui.Style{Fg: th.Palette.Err}.B().Render("✘")
	warnMark := ui.Style{Fg: th.Palette.Warn}.B().Render("•")

	fmt.Println()
	fmt.Printf("  %s %s\n\n", ui.Bold("swaydesk doctor"), ui.Dim("· comprobando el entorno"))
	failures := 0
	for _, c := range checks {
		mark := okMark
		status := ui.Dim(c.detail)
		if !c.ok {
			if c.required {
				mark = badMark
				failures++
				status = ui.Style{Fg: th.Palette.Err}.Render(c.detail + " — falta")
			} else {
				mark = warnMark
				status = ui.Dim(c.detail + " — opcional")
			}
		}
		fmt.Printf("  %s %-22s %s\n", mark, c.name, status)
		if !c.ok && c.fix != "" {
			fmt.Printf("    %s %s\n", ui.Dim("→"), ui.Dim(c.fix))
		}
	}

	// Comprobación de archivos de configuración.
	fmt.Println()
	for _, f := range []struct {
		path string
		name string
	}{
		{filepath.Join(configDir(), "sway", "config"), "sway"},
		{filepath.Join(configDir(), "waybar", "config.json"), "waybar"},
		{filepath.Join(configDir(), "waybar", "style.css"), "waybar (estilo)"},
		{filepath.Join(configDir(), "foot", "foot.ini"), "foot"},
		{filepath.Join(configDir(), "mako", "config"), "mako"},
		{cfg.Path, "swaydesk"},
	} {
		if fileExists(f.path) {
			fmt.Printf("  %s %-22s %s\n", okMark, "config "+f.name, ui.Dim(f.path))
		} else {
			fmt.Printf("  %s %-22s %s\n", warnMark, "config "+f.name, ui.Dim("no existe · ejecuta swaydesk init"))
		}
	}

	fmt.Println()
	if failures > 0 {
		fmt.Printf("  %s faltan %d dependencias obligatorias\n\n", badMark, failures)
		os.Exit(1)
	}
	fmt.Printf("  %s todo listo\n\n", okMark)
}

// ───────────────────────── notify / volume / power ─────────────────────────

func cmdNotify(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: swaydesk notify <título> [cuerpo] [--urgency low|normal|critical]")
		os.Exit(2)
	}
	urgency := "normal"
	var words []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--urgency" && i+1 < len(args) {
			urgency = args[i+1]
			i++
			continue
		}
		words = append(words, args[i])
	}
	title := words[0]
	body := strings.Join(words[1:], " ")

	// Primero se intenta el daemon propio (permite acciones y estilo del tema).
	n := notify.Notification{
		App: "swaydesk", Summary: title, Body: body, Progress: -1,
		Icon: optValue(args, "icon"),
	}
	switch strings.ToLower(urgency) {
	case "low", "baja":
		n.Urgency = notify.Low
	case "critical", "urgente", "crítica", "critica":
		n.Urgency = notify.Critical
	}
	if t := optValue(args, "timeout"); t != "" {
		if ms, err := time.ParseDuration(t + "ms"); err == nil {
			n.Timeout = int(ms / time.Millisecond)
		}
	}
	if notifySend(n) {
		return
	}
	actions.Notification(title, body, urgency)
}

func cmdVolume(args []string) {
	what := flagValue(args, "mute")
	// Con swayosd instalado, el propio cambio dibuja el aviso en pantalla.
	if launcher.HasCommand("swayosd") {
		orden := map[string]string{"up": "up", "+": "up", "raise": "up",
			"down": "down", "-": "down", "lower": "down",
			"mute": "mute", "toggle": "mute"}[what]
		if orden != "" {
			if err := launcher.ExecCommand("swayosd", "volume", orden); err == nil {
				return
			}
		}
	}
	switch what {
	case "up", "+", "raise":
		must(actions.Volume(5, false))
	case "down", "-", "lower":
		must(actions.Volume(-5, false))
	case "mute", "toggle":
		must(actions.Volume(0, true))
	default:
		fmt.Fprintf(os.Stderr, "uso: swaydesk volume <up|down|mute>\n")
		os.Exit(2)
	}
}

func cmdPower(args []string) {
	cfg, err := config.Load()
	must(err)
	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	if len(args) > 0 {
		must(actions.RunPowerAction(args[0], cfg.ConfirmPower, th))
		return
	}
	key, _, ok, err := actions.Pick(th, "Energía · sesión", actions.PowerMenuItems(th), actions.SelectorOptions{})
	must(err)
	if !ok {
		return
	}
	must(actions.RunPowerAction(key, cfg.ConfirmPower, th))
}

func cmdConfig(args []string) {
	cfg, err := config.Load()
	must(err)
	fmt.Printf("archivo    %s\n", cfg.Path)
	fmt.Printf("caché      %s\n", cfg.Cache)
	fmt.Printf("tema       %s (%s)\n", cfg.Theme, ui.PaletteByName(cfg.Theme).Name)
	fmt.Printf("terminal   %s\n", cfg.Terminal)
	fmt.Printf("lanzador   %s\n", cfg.Launcher)
	fmt.Printf("editor     %s\n", cfg.Editor)
	fmt.Printf("navegador  %s\n", cfg.Browser)
	fmt.Printf("archivos   %s\n", cfg.FileManager)
	fmt.Printf("fuente     %s\n", setup.DetectFont(cfg.Font, cfg.NerdFont))
	fmt.Printf("nerd font  %v (instalada: %v)\n", cfg.NerdFont, setup.HasNerdFont())
	fmt.Printf("teclado    %s\n", cfg.KeyboardLayout)
}

// ───────────────────────── utilidades ─────────────────────────

func reloadSway() error {
	c, err := swayipc.Dial()
	if err != nil {
		if launcher.HasCommand("swaymsg") {
			return launcher.StartDetached("swaymsg", "reload")
		}
		return err
	}
	defer c.Close()
	return c.CommandOK("reload")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}

func colorFor(v, warn, crit float64) ui.Color {
	switch {
	case v >= crit:
		return "#f38ba8"
	case v >= warn:
		return "#f9e2af"
	default:
		return "#a6e3a1"
	}
}

// bar dibuja una barra de progreso de width celdas.
func bar(value, total float64, width int, th *ui.Theme, color ui.Color) string {
	if total <= 0 {
		total = 100
	}
	ratio := value / total
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	fill := ui.Style{Fg: color}.Render(strings.Repeat("█", filled))
	rest := ui.Style{Fg: th.Palette.BgSel}.Render(strings.Repeat("░", width-filled))
	return fill + rest
}
