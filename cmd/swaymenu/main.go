// swaymenu es el lanzador de aplicaciones de swaydesk. Busca en las entradas
// .desktop, en el PATH, incluye acciones del escritorio y una calculadora.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/actions"
	"github.com/ShadowNigth/swaydesk/internal/calc"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/gallery"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/setup"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

type payload struct {
	kind   string // app, cmd, action, calc
	entry  launcher.Entry
	action string
}

func main() {
	var (
		float    = flag.Bool("float", false, "abrir el menú en una ventana flotante")
		noFloat  = flag.Bool("no-float", false, "ejecutar en la terminal actual (uso interno)")
		power    = flag.Bool("power", false, "mostrar directamente el menú de energía")
		themes   = flag.Bool("themes", false, "mostrar directamente el selector de temas")
		filter   = flag.String("filter", "", "texto inicial del filtro")
		withCmds = flag.Bool("commands", false, "incluir comandos del PATH")
		width    = flag.Int("width", 0, "ancho de la ventana flotante en columnas")
		height   = flag.Int("height", 0, "alto de la ventana flotante en filas")
		themeArg = flag.String("theme-palette", "", "paleta a usar (sobrescribe la configuración)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "swaymenu · lanzador de aplicaciones para sway\n\n")
		fmt.Fprintf(os.Stderr, "Uso: swaymenu [opciones]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaymenu: %v\n", err)
	}
	if *themeArg != "" {
		cfg.Theme = *themeArg
	}
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	if *power {
		// Waybar, swaybar y los atajos de sway no tienen TTY. En esos casos
		// abre el menú dentro de una terminal flotante; desde una terminal
		// interactiva conserva el comportamiento directo. El hijo relanzado
		// lleva --no-float para impedir que vuelva a abrir otra terminal.
		interactive := ui.IsInteractive()
		if powerMenuNeedsTerminal(*float, *noFloat, interactive) {
			runFloating(cfg, 58, 14)
			return
		}
		runPowerMenu(theme, cfg)
		return
	}
	if *themes {
		// Desde sway no hay terminal: se relanza en una ventana flotante.
		if !ui.IsInteractive() {
			runFloating(cfg, 96, 34)
			return
		}
		runThemePicker(theme, cfg)
		return
	}
	if *float && !*noFloat {
		runFloating(cfg, *width, *height)
		return
	}

	runMenu(theme, cfg, *filter, *withCmds || cfg.Bool("launcher.commands", false))
}

func powerMenuNeedsTerminal(forceFloat, noFloat, interactive bool) bool {
	return forceFloat || (!noFloat && !interactive)
}

// runFloating lanza una copia del menú dentro de una ventana de terminal
// flotante (sway coloca la ventana según la regla app_id="swaydesk-menu").
func runFloating(cfg *config.Config, width, height int) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaymenu: %v\n", err)
		os.Exit(1)
	}
	w, h := width, height
	if w <= 0 {
		w = cfg.Width
	}
	if h <= 0 {
		h = cfg.Height
	}
	args := []string{"--no-float"}
	// Se conservan los argumentos relevantes de la invocación original.
	for _, a := range os.Args[1:] {
		if a == "--float" || strings.HasPrefix(a, "--width") || strings.HasPrefix(a, "--height") {
			continue
		}
		args = append(args, a)
	}
	if err := launcher.LaunchTerminal(cfg.Terminal, "swaydesk-menu", "swaydesk", w, h, append([]string{exe}, args...)); err != nil {
		fmt.Fprintf(os.Stderr, "swaymenu: no se pudo abrir la terminal del menú: %v\n", err)
		actions.Notification("swaydesk", "No se pudo abrir la terminal del menú: "+err.Error(), "critical")
		os.Exit(1)
	}
}

// buildItems reúne acciones, aplicaciones y comandos ordenados por uso.
func buildItems(cfg *config.Config, theme *ui.Theme, withCommands bool) ([]ui.Item, map[string]payload) {
	items := make([]ui.Item, 0, 256)
	byKey := make(map[string]payload, 256)
	add := func(it ui.Item, p payload) {
		if _, exists := byKey[it.Key]; exists {
			return
		}
		byKey[it.Key] = p
		items = append(items, it)
	}

	// --- Acciones integradas del escritorio ---
	builtin := []struct {
		id, label, desc, icon string
		requires              string
	}{
		{"terminal", "Terminal", cfg.Terminal, theme.Icons.Terminal, cfg.Terminal},
		{"files", "Archivos", cfg.FileManager, theme.Icons.Files, cfg.FileManager},
		{"browser", "Navegador", cfg.Browser, theme.Icons.Browser, cfg.Browser},
		{"swayctl", "Panel de control", "estado, temas, atajos", theme.Icons.Layout, "swayctl"},
		{"swayclip", "Portapapeles", "historial de wl-clipboard", theme.Icons.Scroll, "swayclip"},
		{"screenshot", "Captura de pantalla", "grim + slurp", theme.Icons.Camera, "grim"},
		{"lock", "Bloquear pantalla", "swaylock", theme.Icons.Lock, "swaylock"},
		{"power", "Energía", "apagar, reiniciar, suspender", theme.Icons.Power, ""},
		{"edit-sway", "Editar configuración de sway", "~/.config/sway/config", theme.Icons.Gear, cfg.Editor},
		{"edit-swaydesk", "Editar configuración de swaydesk", cfg.Path, theme.Icons.Doc, cfg.Editor},
	}
	for _, b := range builtin {
		if b.requires != "" && !launcher.HasCommand(b.requires) {
			continue
		}
		add(ui.Item{Key: "act:" + b.id, Label: b.label, Hint: b.desc, Icon: b.icon},
			payload{kind: "action", action: b.id})
	}

	// --- Aplicaciones .desktop ---
	hist := launcher.LoadHistory(filepath.Join(cfg.Cache, "history"), cfg.HistorySize)
	now := time.Now().Unix()
	apps := launcher.DesktopEntries()
	sort.SliceStable(apps, func(i, j int) bool {
		si, sj := hist.Score(apps[i].Key, now), hist.Score(apps[j].Key, now)
		if si != sj {
			return si > sj
		}
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	for _, app := range apps {
		hint := app.Comment
		if hint == "" {
			hint = firstCategory(app.Categories)
		}
		add(ui.Item{Key: app.Key, Label: app.Name, Hint: hint, Icon: iconFor(app, theme.Icons)},
			payload{kind: "app", entry: app})
	}

	// --- Comandos del PATH ---
	if withCommands {
		for _, cmd := range launcher.PathCommands() {
			add(ui.Item{Key: cmd.Key, Label: cmd.Name, Hint: "comando", Icon: theme.Icons.Terminal},
				payload{kind: "cmd", entry: cmd})
		}
	}
	return items, byKey
}

// iconFor elige un icono según la categoría de la aplicación.
func iconFor(app launcher.Entry, ic ui.Icons) string {
	cats := strings.ToLower(app.Categories + " " + app.Name + " " + app.Exec)
	switch {
	case strings.Contains(cats, "web") && strings.Contains(cats, "browser"),
		strings.Contains(cats, "chromium"), strings.Contains(cats, "chrome"), strings.Contains(cats, "firefox"):
		return ic.Browser
	case strings.Contains(cats, "terminal"), strings.Contains(cats, "shell"):
		return ic.Terminal
	case strings.Contains(cats, "filemanager"), strings.Contains(cats, "files"):
		return ic.Files
	case strings.Contains(cats, "development"), strings.Contains(cats, "ide"),
		strings.Contains(cats, "code"), strings.Contains(cats, "editor"):
		return ic.Code
	case strings.Contains(cats, "audio"), strings.Contains(cats, "video"), strings.Contains(cats, "player"):
		return ic.Music
	case strings.Contains(cats, "graphics"), strings.Contains(cats, "photo"):
		return ic.Camera
	case strings.Contains(cats, "office"), strings.Contains(cats, "word"), strings.Contains(cats, "text"):
		return ic.Doc
	case strings.Contains(cats, "game"):
		return ic.Game
	case strings.Contains(cats, "chat"), strings.Contains(cats, "instantmessaging"):
		return ic.Chat
	case strings.Contains(cats, "mail"):
		return ic.Mail
	case strings.Contains(cats, "calculator"):
		return ic.Calc
	case strings.Contains(cats, "settings"), strings.Contains(cats, "system"):
		return ic.Gear
	}
	return ic.Window
}

func firstCategory(cats string) string {
	for _, c := range strings.Split(cats, ";") {
		if c != "" && c != "GTK" && c != "Qt" {
			return strings.ToLower(c)
		}
	}
	return ""
}

func runMenu(theme *ui.Theme, cfg *config.Config, filter string, withCommands bool) {
	sc, err := ui.OpenScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaymenu: %v\n", err)
		os.Exit(1)
	}
	defer sc.Close()

	sel := ui.NewSelector(theme, "swaydesk · lanzador", nil)
	sel.SetFilter(filter)
	sel.Placeholder = "escribe para buscar ·  = para calcular"
	sel.Empty = "cargando aplicaciones…"

	var (
		calcResult string
		calcErr    error
		base       []ui.Item
		byKey      map[string]payload
		loaded     bool
	)
	refresh := func() {
		f := strings.TrimSpace(sel.Filter)
		sel.SetItems(base)
		sel.SetBypassFilter(false)
		calcResult, calcErr = "", nil
		if strings.HasPrefix(f, "=") {
			expr := strings.TrimSpace(strings.TrimPrefix(f, "="))
			if expr != "" {
				v, err := calc.Eval(expr)
				if err != nil {
					calcErr = err
				} else {
					calcResult = calc.Format(v)
					sel.SetItems([]ui.Item{{
						Key:   "calc",
						Label: calcResult,
						Hint:  "copiar resultado",
						Icon:  theme.Icons.Calc,
					}})
					sel.SetBypassFilter(true)
					sel.Cursor = 0
				}
			}
		}
	}
	type buildResult struct {
		items []ui.Item
		byKey map[string]payload
	}
	ready := make(chan buildResult, 1)
	wake := make(chan time.Time, 1)
	go func() {
		items, payloads := buildItems(cfg, theme, withCommands)
		ready <- buildResult{items: items, byKey: payloads}
		wake <- time.Now()
	}()
	consumeReady := func() {
		if loaded {
			return
		}
		select {
		case result := <-ready:
			loaded = true
			base, byKey = result.items, result.byKey
			sel.Empty = "sin resultados"
			refresh()
		default:
		}
	}
	editFilterWhileLoading := func(k ui.Key) bool {
		switch {
		case k.Is("esc"):
			if sel.Filter != "" {
				sel.ClearFilter()
				return false
			}
			return true
		case k.Is("enter"):
			// Evita cerrar el lanzador si la búsqueda inicial sigue cargando.
			return false
		case k.Is("backspace"):
			sel.Backspace()
		case k.Ctrl && k.IsRune('u'):
			sel.ClearFilter()
		case k.Ctrl && k.IsRune('w'):
			f := strings.TrimRight(sel.Filter, " ")
			if i := strings.LastIndexByte(f, ' '); i >= 0 {
				sel.SetFilter(f[:i+1])
			} else {
				sel.SetFilter("")
			}
		case k.Rune >= 32 && !k.Ctrl && !k.Alt:
			sel.Type(k.Rune)
		}
		return false
	}

	_ = sc.Loop(
		func(w, h int) string {
			consumeReady()
			view := sel.View(w, h)
			if calcErr != nil && strings.HasPrefix(sel.Filter, "=") {
				// Sustituye la línea de pistas por el error de cálculo.
				lines := strings.Split(view, "\n")
				if len(lines) >= 2 {
					inner := w - 4
					if inner > 0 {
						msg := theme.StyleOn(theme.Palette.Err, theme.Palette.Bg).Render(
							" " + ui.Truncate(fmt.Sprintf("%s: %v", "error", calcErr), inner-2) + " ")
						lines[len(lines)-2] = ui.PadRight(msg, w)
					}
				}
				return strings.Join(lines, "\n")
			}
			return view
		},
		func(k ui.Key) bool {
			consumeReady()
			if !loaded {
				return editFilterWhileLoading(k)
			}
			switch {
			case k.Is("esc"):
				if sel.Filter != "" {
					sel.ClearFilter()
					refresh()
					return false
				}
				return true
			case k.Is("enter"):
				it, ok := sel.Selected()
				if !ok {
					return true
				}
				execute(it, byKey, theme, cfg, calcResult)
				return true
			case k.Is("up") || (k.Ctrl && k.IsRune('p')):
				sel.Move(-1)
			case k.Is("down") || (k.Ctrl && k.IsRune('n')):
				sel.Move(1)
			case k.Is("pgup"):
				sel.Move(-8)
			case k.Is("pgdn"):
				sel.Move(8)
			case k.Is("home"):
				sel.Cursor = 0
			case k.Is("end"):
				sel.Cursor = len(sel.Filtered()) - 1
			case k.Is("backspace"):
				sel.Backspace()
				refresh()
			case k.Ctrl && k.IsRune('u'):
				sel.ClearFilter()
				refresh()
			case k.Ctrl && k.IsRune('w'):
				f := strings.TrimRight(sel.Filter, " ")
				if i := strings.LastIndexByte(f, ' '); i >= 0 {
					sel.SetFilter(f[:i+1])
				} else {
					sel.SetFilter("")
				}
				refresh()
			case k.Rune >= 32 && !k.Ctrl && !k.Alt:
				sel.Type(k.Rune)
				refresh()
			}
			return false
		}, wake)
}

// execute lanza la entrada seleccionada.
func execute(it ui.Item, byKey map[string]payload, theme *ui.Theme, cfg *config.Config, calcResult string) {
	p, ok := byKey[it.Key]
	if !ok {
		if it.Key == "calc" && calcResult != "" {
			copyToClipboard(calcResult)
			actions.Notification("Calculadora", "Resultado copiado: "+calcResult, "low")
		}
		return
	}
	hist := launcher.LoadHistory(filepath.Join(cfg.Cache, "history"), cfg.HistorySize)
	switch p.kind {
	case "app", "cmd":
		hist.Record(p.entry.Key, time.Now().Unix())
		if err := launcher.Launch(p.entry, cfg.Terminal); err != nil {
			actions.Notification("swaymenu", fmt.Sprintf("No se pudo lanzar %s: %v", it.Label, err), "critical")
		}
	case "action":
		switch p.action {
		case "power":
			runPowerMenu(theme, cfg)
		default:
			runAction(p.action, theme, cfg)
		}
	}
}

// runAction ejecuta una acción integrada por nombre.
func runAction(id string, theme *ui.Theme, cfg *config.Config) {
	fail := func(err error) {
		if err != nil {
			actions.Notification("swaydesk", err.Error(), "critical")
		}
	}
	switch id {
	case "terminal":
		fail(launcher.StartDetached(cfg.Terminal))
	case "files":
		fail(launcher.StartDetached(cfg.FileManager))
	case "browser":
		fail(launcher.StartDetached(cfg.Browser))
	case "lock":
		fail(actions.Lock())
	case "screenshot":
		fail(actions.Screenshot("region"))
	case "reload":
		fail(reloadSway())
	case "edit-sway", "edit-swaydesk":
		home, _ := os.UserHomeDir()
		path := cfg.Path
		if id == "edit-sway" {
			path = filepath.Join(home, ".config/sway/config")
		}
		fail(launcher.StartInTerminal(cfg.Terminal, []string{cfg.Editor, path}))
	case "swayctl":
		fail(launcher.LaunchTerminal(cfg.Terminal, "swaydesk-ctl", "swaydesk · panel", 0, 0, []string{"swayctl"}))
	case "swayclip":
		fail(launcher.LaunchTerminal(cfg.Terminal, "swaydesk-clip", "swaydesk · portapapeles", 0, 0, []string{"swayclip"}))
	case "swaymenu-commands":
		fail(launcher.LaunchTerminal(cfg.Terminal, "swaydesk-menu", "swaydesk", cfg.Width, cfg.Height,
			[]string{"swaymenu", "--no-float", "--commands"}))
	}
}

func reloadSway() error {
	c, err := swayipc.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	return c.CommandOK("reload")
}

func runPowerMenu(theme *ui.Theme, cfg *config.Config) {
	items := actions.PowerMenuItems(theme)
	key, _, ok, err := actions.Pick(theme, "Energía · sesión", items, actions.SelectorOptions{})
	if err != nil {
		message := "No se pudo abrir el menú de energía: " + err.Error()
		fmt.Fprintf(os.Stderr, "swaymenu: %s\n", message)
		actions.Notification("swaydesk", message, "critical")
		return
	}
	if !ok {
		return
	}
	if err := actions.RunPowerAction(key, cfg.ConfirmPower, theme); err != nil {
		actions.Notification("swaydesk", err.Error(), "critical")
	}
}

func runThemePicker(theme *ui.Theme, cfg *config.Config) {
	// Se usa la misma galería que `swaydesk theme --gallery`.
	res, err := gallery.ThemeGallery(cfg.Theme, cfg.NerdFont)
	if err != nil || !res.Applied || res.Key == "" {
		return
	}
	if err := applyTheme(res.Key); err != nil {
		actions.Notification("swaydesk", err.Error(), "critical")
	}
}

// applyTheme cambia la paleta activa: guarda la configuración, regenera los
// archivos dependientes del tema y avisa a waybar y sway.
func applyTheme(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.SaveTheme(name); err != nil {
		return err
	}
	dest, err := configDir()
	if err != nil {
		return err
	}
	if _, err := setup.Apply(cfg, dest); err != nil {
		return err
	}
	actions.Notification("swaydesk", "Tema aplicado: "+ui.PaletteByName(name).Name, "low")
	return nil
}

// configDir devuelve ~/.config (o $XDG_CONFIG_HOME).
func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

func copyToClipboard(text string) {
	for _, tool := range [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}} {
		if launcher.HasCommand(tool[0]) {
			if err := launcher.WriteToCommand(text, tool...); err == nil {
				return
			}
		}
	}
}
