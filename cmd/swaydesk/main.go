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
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/host"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/setup"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
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
  theme [nombre] Lista las paletas disponibles o cambia la activa
  config         Muestra las rutas y los valores de configuración

Sesión
  status [--json|--module M]  Estado del escritorio (sway, CPU, RAM, batería...)
  doctor                      Diagnóstico de dependencias y configuración
  notify <título> [cuerpo]    Envía una notificación de escritorio
  volume <up|down|mute>       Controla el volumen
  screenshot [region|full]    Captura de pantalla con grim
  wallpaper <imagen> [modo]   Fondo de pantalla (y lo guarda en config.d)
  lock                        Bloquea la pantalla
  reload                      Recarga la configuración de sway
  power [acción]              Menú de energía (lock, suspend, logout, reboot, poweroff)
  version                     Muestra la versión

Ejemplos:
  swaydesk init --force && swaymsg reload
  swaydesk theme gruvbox
  swaydesk status --module clock

Atajos por defecto (Super = Mod4):
  Super+Return  terminal        Super+d       lanzador
  Super+p       energía         Super+t       temas
  Super+c       portapapeles    Super+i       panel de control
  Super+Shift+l bloquear        Super+q       cerrar ventana
  Super+r       redimensionar    Super+w       pestañas
`)
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		os.Exit(1)
	}
}

func flagValue(args []string, def string) string {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0]
	}
	return def
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
	if len(args) == 0 {
		th := ui.NewTheme(cfg.Theme, cfg.NerdFont)
		fmt.Println(ui.Bold("Paletas disponibles:"))
		for _, name := range ui.PaletteNames() {
			p := ui.PaletteByName(name)
			mark := " "
			if name == cfg.Theme {
				mark = ui.Style{Fg: th.Palette.Ok}.Render("●")
			}
			swatch := ""
			for _, c := range []ui.Color{p.Bg, p.BgAlt, p.BgSel, p.Accent, p.Accent2, p.Ok, p.Warn, p.Err} {
				swatch += ui.Style{Fg: c, Bg: c}.Render("  ")
			}
			fmt.Printf(" %s %-12s %-18s %s\n", mark, name, p.Name, swatch)
		}
		fmt.Println()
		fmt.Println(ui.Dim("Cambiar tema: swaydesk theme <nombre>   ·   menú interactivo: swaymenu --themes"))
		return
	}
	name := strings.ToLower(args[0])
	if _, ok := ui.BuiltinPalettes[name]; !ok {
		fmt.Fprintf(os.Stderr, "swaydesk: paleta desconocida %q\n", name)
		fmt.Fprintf(os.Stderr, "disponibles: %s\n", strings.Join(ui.PaletteNames(), ", "))
		os.Exit(1)
	}
	must(cfg.SaveTheme(name))
	rest, err := setup.Apply(cfg, configDir())
	must(err)
	th := ui.NewTheme(name, cfg.NerdFont)
	fmt.Printf("%s Tema cambiado a %s\n", ui.Style{Fg: th.Palette.Ok}.Render("✔"), th.Palette.Name)
	for _, r := range rest {
		fmt.Printf("  %s\n", ui.Dim(r.Path))
	}
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

// ───────────────────────── doctor ─────────────────────────

type check struct {
	name     string
	ok       bool
	required bool
	detail   string
	fix      string
}

func cmdDoctor(args []string) {
	cfg, err := config.Load()
	must(err)
	th := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	checks := []check{
		{name: "sway", required: true, ok: launcher.HasCommand("sway") || swayipc.SwaySock() != "",
			detail: "compositor", fix: "sudo pacman -S sway"},
		{name: "socket IPC de sway", required: true, ok: swayipc.SwaySock() != "",
			detail: swayipc.SwaySock(), fix: "exporta SWAYSOCK dentro de la sesión de sway"},
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
		{filepath.Join(configDir(), "waybar", "config"), "waybar"},
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
	actions.Notification(title, body, urgency)
}

func cmdVolume(args []string) {
	what := flagValue(args, "mute")
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

// ───────────────────────── wallpaper ─────────────────────────

// cmdWallpaper aplica un fondo de pantalla a todas las salidas y lo guarda en
// un archivo dentro de config.d para que se restaure al recargar sway.
func cmdWallpaper(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: swaydesk wallpaper <imagen> [fill|fit|stretch|center|tile]")
		fmt.Fprintln(os.Stderr, "     swaydesk wallpaper --list")
		os.Exit(2)
	}
	if args[0] == "--list" || args[0] == "-l" {
		listWallpapers()
		return
	}
	path := args[0]
	mode := "fill"
	if len(args) > 1 {
		mode = args[1]
	}
	switch mode {
	case "fill", "fit", "stretch", "center", "tile":
	default:
		fmt.Fprintf(os.Stderr, "swaydesk: modo de fondo inválido %q\n", mode)
		os.Exit(2)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		must(err)
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: no se puede leer %s: %v\n", abs, err)
		os.Exit(1)
	}

	// 1. Aplicar en caliente mediante IPC (sway dibuja el fondo él mismo).
	applied := false
	if c, err := swayipc.Dial(); err == nil {
		err = c.CommandOK(fmt.Sprintf(`output * bg "%s" %s`, abs, mode))
		c.Close()
		if err == nil {
			applied = true
		}
	}
	// 2. Alternativa: swaybg en segundo plano.
	if !applied && launcher.HasCommand("swaybg") {
		must(launcher.StartDetached("swaybg", "-m", mode, "-i", abs))
		applied = true
	}
	if !applied {
		fmt.Fprintln(os.Stderr, "swaydesk: no hay sesión de sway activa; el fondo se guardará igualmente")
	}

	// 3. Persistir el ajuste.
	confDir := filepath.Join(configDir(), "sway", "config.d")
	must(os.MkdirAll(confDir, 0o755))
	conf := fmt.Sprintf("# Fondo de pantalla establecido con `swaydesk wallpaper`\noutput * bg \"%s\" %s\n", abs, mode)
	must(os.WriteFile(filepath.Join(confDir, "50-wallpaper.conf"), []byte(conf), 0o644))

	th := ui.NewTheme("catppuccin", false)
	fmt.Printf("%s fondo aplicado: %s %s\n", ui.Style{Fg: th.Palette.Ok}.Render("✔"),
		abs, ui.Dim("("+mode+")"))
}

// listWallpapers muestra las imágenes de los directorios habituales.
func listWallpapers() {
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, "Imágenes", "Fondos"),
		filepath.Join(home, "Imágenes"),
		filepath.Join(home, "Pictures", "Wallpapers"),
		filepath.Join(home, "Pictures"),
		"/usr/share/backgrounds",
	}
	exts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".bmp": true}
	seen := map[string]bool{}
	fmt.Println(ui.Bold("Fondos de pantalla encontrados:"))
	found := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !exts[strings.ToLower(filepath.Ext(e.Name()))] {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if seen[path] {
				continue
			}
			seen[path] = true
			found++
			fmt.Printf("  %s\n", path)
		}
	}
	if found == 0 {
		fmt.Println(ui.Dim("  no se encontraron imágenes en ~/Imágenes ni /usr/share/backgrounds"))
	}
	fmt.Println()
	fmt.Println(ui.Dim("Aplicar: swaydesk wallpaper <ruta> [fill|fit|stretch|center|tile]"))
}
