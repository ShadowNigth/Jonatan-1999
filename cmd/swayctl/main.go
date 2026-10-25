// swayctl es el panel de control del escritorio: estado del sistema, gestión
// de workspaces y ventanas, cambio de tema y chuleta de atajos.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

var tabs = []string{"Estado", "Workspaces", "Ventanas", "Apariencia", "Atajos"}

type app struct {
	theme   *ui.Theme
	cfg     *config.Config
	tab     int
	cursor  []int // cursor por pestaña
	offset  []int
	col     *host.Collector
	state   host.State
	sway    *swayipc.Conn
	tree    *swayipc.Node
	ws      []swayipc.Workspace
	outs    []swayipc.Output
	status  string
	statusT time.Time
	preview string
	lastRef time.Time
}

func main() {
	tabFlag := flag.Int("tab", 1, "pestaña inicial (1-5)")
	themeFlag := flag.String("theme-palette", "", "paleta a usar")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swayctl: %v\n", err)
	}
	if relaunched, launchErr := launcher.RelaunchInTerminalIfNeeded(
		ui.IsInteractive(), cfg.Terminal, "swaydesk-ctl", "swaydesk · panel", 0, 0, os.Args[1:],
	); relaunched {
		if launchErr != nil {
			message := "no se pudo abrir el panel en una terminal: " + launchErr.Error()
			fmt.Fprintf(os.Stderr, "swayctl: %s\n", message)
			actions.Notification("swayctl", message, "critical")
			os.Exit(1)
		}
		return
	}
	if *themeFlag != "" {
		cfg.Theme = *themeFlag
	}
	a := &app{
		theme:  ui.NewTheme(cfg.Theme, cfg.NerdFont),
		cfg:    cfg,
		col:    host.NewCollector(),
		cursor: make([]int, len(tabs)),
		offset: make([]int, len(tabs)),
	}
	a.tab = clamp(*tabFlag-1, 0, len(tabs)-1)
	a.refresh()

	sc, err := ui.OpenScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swayctl: %v\n", err)
		os.Exit(1)
	}
	defer sc.Close()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	_ = sc.Loop(
		func(w, h int) string { return a.view(w, h) },
		func(k ui.Key) bool { return a.handle(k) },
		ticker.C,
	)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// refresh recarga el estado del sistema y de sway.
func (a *app) refresh() {
	a.state = a.col.Snapshot()
	if a.sway == nil {
		if c, err := swayipc.Dial(); err == nil {
			a.sway = c
		}
	}
	if a.sway != nil {
		if tree, err := a.sway.Tree(); err == nil {
			a.tree = tree
		} else {
			a.sway.Close()
			a.sway = nil
		}
	}
	if a.sway != nil {
		a.ws, _ = a.sway.Workspaces()
		a.outs, _ = a.sway.Outputs()
	}
	a.lastRef = time.Now()
}

func (a *app) setStatus(msg string) {
	a.status = msg
	a.statusT = time.Now()
}

func (a *app) handle(k ui.Key) bool {
	t := a.theme
	_ = t
	switch {
	case k.Is("esc") || k.IsRune('q'):
		return true
	case k.Is("tab"):
		a.tab = (a.tab + 1) % len(tabs)
	case k.Is("backtab"):
		a.tab = (a.tab + len(tabs) - 1) % len(tabs)
	case k.Is("left"):
		a.tab = (a.tab + len(tabs) - 1) % len(tabs)
	case k.Is("right"):
		a.tab = (a.tab + 1) % len(tabs)
	case k.IsRune('r'):
		a.refresh()
		a.setStatus("datos actualizados")
	case k.Is("down") || k.IsRune('j'):
		a.move(1)
	case k.Is("up") || k.IsRune('k'):
		a.move(-1)
	case k.Is("pgdown"):
		a.move(8)
	case k.Is("pgup"):
		a.move(-8)
	case k.Is("home"):
		a.cursor[a.tab] = 0
	case k.Is("end"):
		a.cursor[a.tab] = 1 << 20
		a.move(0)
	case k.Is("enter"):
		a.activate()
	case k.IsRune('x'):
		a.closeWindow()
	case k.IsRune('f'):
		a.toggleFloating()
	case k.IsRune('m'):
		a.maximizeWindow()
	case k.IsRune('n'):
		a.quickNote()
	}
	return false
}

func (a *app) move(delta int) {
	n := a.listLen()
	if n == 0 {
		return
	}
	c := a.cursor[a.tab] + delta
	if c < 0 {
		c = 0
	}
	if c >= n {
		c = n - 1
	}
	a.cursor[a.tab] = c
}

func (a *app) listLen() int {
	switch a.tab {
	case 1:
		return len(a.ws)
	case 2:
		return len(a.windows())
	case 3:
		return len(ui.PaletteNames())
	}
	return 0
}

// activate ejecuta la acción principal del elemento seleccionado.
func (a *app) activate() {
	switch a.tab {
	case 1:
		if a.cursor[1] < len(a.ws) {
			a.command("workspace "+a.ws[a.cursor[1]].Name, "workspace "+a.ws[a.cursor[1]].Name)
		}
	case 2:
		if f := a.selectedWindow(); f != nil {
			a.command(fmt.Sprintf(`[con_id=%d] focus`, f.ID), "ventana enfocada")
		}
	case 3:
		names := ui.PaletteNames()
		if a.cursor[2] < len(names) {
			name := names[a.cursor[2]]
			if err := a.cfg.SaveTheme(name); err != nil {
				a.setStatus("error: " + err.Error())
				return
			}
			dest := configRoot()
			if _, err := setup.Apply(a.cfg, dest); err != nil {
				a.setStatus("error: " + err.Error())
				return
			}
			a.theme = ui.NewTheme(name, a.cfg.NerdFont)
			a.setStatus("tema aplicado: " + a.theme.Palette.Name)
		}
	}
}

func configRoot() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func (a *app) selectedWindow() *swayipc.Node {
	windows := a.windows()
	if len(windows) == 0 {
		return nil
	}
	c := a.cursor[2]
	if c >= len(windows) {
		c = len(windows) - 1
	}
	if c < 0 {
		c = 0
	}
	return windows[c]
}

// windows devuelve las ventanas en orden de aparición en el árbol.
func (a *app) windows() []*swayipc.Node {
	if a.tree == nil {
		return nil
	}
	return a.tree.Leaves()
}

func (a *app) command(cmd, okMsg string) {
	if a.sway == nil {
		a.setStatus("sin conexión con sway")
		return
	}
	if err := a.sway.CommandOK(cmd); err != nil {
		a.setStatus("error: " + err.Error())
		return
	}
	a.setStatus(okMsg)
	a.refresh()
}

func (a *app) closeWindow() {
	if a.tab != 2 {
		return
	}
	w := a.selectedWindow()
	if w == nil {
		a.setStatus("no hay ventana seleccionada")
		return
	}
	a.command(fmt.Sprintf(`[con_id=%d] kill`, w.ID), "ventana cerrada")
}

func (a *app) toggleFloating() {
	if a.tab != 2 {
		return
	}
	w := a.selectedWindow()
	if w == nil {
		return
	}
	a.command(fmt.Sprintf(`[con_id=%d] floating toggle`, w.ID), "ventana flotante alternada")
}

func (a *app) maximizeWindow() {
	if a.tab != 2 {
		return
	}
	w := a.selectedWindow()
	if w == nil {
		return
	}
	a.command(fmt.Sprintf(`[con_id=%d] fullscreen toggle`, w.ID), "pantalla completa alternada")
}

func (a *app) quickNote() {
	if a.tab == 2 {
		w := a.selectedWindow()
		if w == nil {
			return
		}
		a.command(fmt.Sprintf(`[con_id=%d] move scratchpad`, w.ID), "ventana enviada al scratchpad")
	}
}

// ───────────────────────── vista ─────────────────────────

func (a *app) view(w, h int) string {
	t := a.theme
	innerW := w - 2

	// Cabecera con reloj y host.
	clock := ui.FechaHora(time.Now())
	left := " " + t.Icons.Workspace + " " + t.Palette.Name
	right := host.Hostname() + " · " + clock + " "
	head := t.Accent().Render(left)
	pad := innerW - ui.Width(head) - ui.Width(t.Muted().Render(right))
	if pad < 0 {
		pad = 0
	}
	head += strings.Repeat(" ", pad) + t.Muted().Render(right)

	// Pestañas.
	var tabParts []string
	for i, name := range tabs {
		label := fmt.Sprintf(" %s ", name)
		if i == a.tab {
			tabParts = append(tabParts, ui.Style{Fg: t.Palette.Bg, Bg: t.Palette.Accent}.B().Render(label))
		} else {
			tabParts = append(tabParts, ui.Style{Fg: t.Palette.Muted, Bg: t.Palette.Bg}.Render(label))
		}
	}
	tabBar := strings.Join(tabParts, t.Muted().Render("│"))

	body := []string{head, tabBar, t.Muted().Render(strings.Repeat("─", innerW))}
	contentH := h - 2 /*bordes*/ - len(body) - 1 /*pie*/
	if contentH < 3 {
		contentH = 3
	}
	content := a.content(innerW, contentH)
	body = append(body, content...)
	for len(body) < h-2-1 {
		body = append(body, "")
	}

	footer := a.footer()
	return t.Frame(w, h, "swaydesk · panel de control", footer, body)
}

func (a *app) content(w, h int) []string {
	switch a.tab {
	case 0:
		return a.viewStatus(w, h)
	case 1:
		return a.viewWorkspaces(w, h)
	case 2:
		return a.viewWindows(w, h)
	case 3:
		return a.viewThemes(w, h)
	default:
		return a.viewKeys(w, h)
	}
}

func (a *app) viewStatus(w, h int) []string {
	t := a.theme
	p := t.Palette
	var out []string
	line := func(label, value string) {
		out = append(out, " "+t.Muted().Render(ui.PadRight(label, 14))+value)
	}

	if a.sway != nil {
		v := ""
		if ver, err := a.sway.Version(); err == nil {
			v = ver.HumanReadable
		}
		focused := ui.Dim("—")
		if a.tree != nil {
			if f := a.tree.FindFocused(); f != nil {
				focused = t.Text().Render(ui.Truncate(f.Title(), 44)) + " " + t.Muted().Render("("+f.App()+")")
			}
		}
		out = append(out, " "+t.Accent().Render("sway "+v))
		line("ventanas", fmt.Sprintf("%d abiertas", len(a.windows())))
		line("enfocada", focused)
		var wsNames []string
		for _, ws := range a.ws {
			if ws.Focused {
				wsNames = append(wsNames, t.Selected().Render(" "+ws.Name+" "))
			} else if ws.Visible {
				wsNames = append(wsNames, ui.Style{Fg: p.Fg, Bg: p.BgSel}.Render(" "+ws.Name+" "))
			} else {
				wsNames = append(wsNames, t.Muted().Render(" "+ws.Name+" "))
			}
		}
		line("workspaces", strings.Join(wsNames, " "))
		var outs []string
		for _, o := range a.outs {
			if o.Active {
				outs = append(outs, fmt.Sprintf("%s %dx%d", o.Name, o.Rect.Width, o.Rect.Height))
			}
		}
		line("salidas", t.Faint().Render(strings.Join(outs, " · ")))
	} else {
		out = append(out, " "+ui.Style{Fg: p.Warn}.Render("sway no está en ejecución"))
	}

	out = append(out, "")
	out = append(out, " "+t.Accent().Render("sistema"))
	s := a.state
	memPct := 0.0
	if s.MemTotal > 0 {
		memPct = float64(s.MemUsed) * 100 / float64(s.MemTotal)
	}
	out = append(out, " "+t.Muted().Render(ui.PadRight("cpu", 14))+
		bar(s.CPU, 24, t, stateColor(s.CPU, 70, 90))+fmt.Sprintf(" %.0f%%", s.CPU))
	out = append(out, " "+t.Muted().Render(ui.PadRight("memoria", 14))+
		bar(memPct, 24, t, stateColor(memPct, 70, 90))+
		t.Faint().Render(fmt.Sprintf(" %s / %s", humanBytes(s.MemUsed), humanBytes(s.MemTotal))))
	diskPct := 0.0
	if s.DiskTotal > 0 {
		diskPct = float64(s.DiskUsed) * 100 / float64(s.DiskTotal)
	}
	out = append(out, " "+t.Muted().Render(ui.PadRight("disco", 14))+
		bar(diskPct, 24, t, stateColor(diskPct, 80, 95))+
		t.Faint().Render(fmt.Sprintf(" %s / %s", humanBytes(s.DiskUsed), humanBytes(s.DiskTotal))))
	if s.Temp > 0 {
		out = append(out, " "+t.Muted().Render(ui.PadRight("temperatura", 14))+
			t.Text().Render(fmt.Sprintf("%.1f °C", s.Temp)))
	}
	if s.Battery != nil && s.Battery.Present {
		mark := t.Icons.Battery
		if s.Battery.Charging() {
			mark = t.Icons.Charging
		}
		out = append(out, " "+t.Muted().Render(ui.PadRight("batería", 14))+
			t.Accent().Render(mark+" ")+fmt.Sprintf("%d%% ", s.Battery.Percent)+t.Faint().Render(s.Battery.Status))
	}
	vol := fmt.Sprintf("%d%%", s.Volume)
	if s.Muted {
		vol = "silencio"
	}
	out = append(out, " "+t.Muted().Render(ui.PadRight("audio", 14))+t.Text().Render(vol))
	net := "sin conexión"
	if s.WifiName != "" {
		net = s.WifiName
		if s.WifiSignal > 0 {
			net += fmt.Sprintf(" (%d%%)", s.WifiSignal)
		}
	}
	out = append(out, " "+t.Muted().Render(ui.PadRight("red", 14))+t.Text().Render(net))
	out = append(out, " "+t.Muted().Render(ui.PadRight("encendido", 14))+
		t.Faint().Render(formatUptime(s.Uptime)))

	return clampLines(out, h)
}

func (a *app) viewWorkspaces(w, h int) []string {
	t := a.theme
	var out []string
	out = append(out, " "+t.Muted().Render("Enter entra al workspace · n ventanas"))
	out = append(out, "")
	for i, ws := range a.ws {
		selected := i == a.cursor[1]
		bg := t.Palette.Bg
		if selected {
			bg = t.Palette.BgSel
		}
		marker := "  "
		if ws.Focused {
			marker = t.Accent().Render("▸ ")
		}
		name := fmt.Sprintf(" %s %s", ws.Name, ws.Output)
		vis := "oculto"
		if ws.Visible {
			vis = "visible"
		}
		hint := fmt.Sprintf("%d ventanas · %s", countWorkspaceWindows(a.tree, ws.Name), vis)
		row := ui.Style{Fg: t.Palette.Fg, Bg: bg}.Render(ui.Truncate(name, 30))
		pad := w - 2 - ui.Width(marker) - ui.Width(row) - len(hint) - 2
		if pad < 0 {
			pad = 0
		}
		out = append(out, " "+marker+row+ui.Style{Bg: bg}.Render(strings.Repeat(" ", pad))+
			t.Muted().Render(hint)+" ")
	}
	return clampLines(out, h)
}

func (a *app) viewWindows(w, h int) []string {
	t := a.theme
	var out []string
	out = append(out, " "+t.Muted().Render("Enter enfocar · x cerrar · f flotante · m pantalla completa"))
	out = append(out, "")
	windows := a.windows()
	if len(windows) == 0 {
		out = append(out, " "+t.Muted().Render("no hay ventanas abiertas"))
	}
	for i, win := range windows {
		selected := i == a.cursor[2]
		bg := t.Palette.Bg
		if selected {
			bg = t.Palette.BgSel
		}
		marker := "  "
		if win.Focused {
			marker = t.Accent().Render("▸ ")
		}
		float := ""
		if win.Floating != "" && win.Floating != "auto_off" {
			float = "flotante"
		}
		title := ui.Style{Fg: t.Palette.Fg, Bg: bg}.B().Render(ui.Truncate(ui.Sanitize(win.Title()), 46))
		app := t.Muted().Render(ui.Truncate(win.App(), 18))
		pad := w - 2 - ui.Width(marker) - ui.Width(title) - ui.Width(app) - len(float) - 3
		if pad < 0 {
			pad = 0
		}
		out = append(out, " "+marker+title+ui.Style{Bg: bg}.Render(strings.Repeat(" ", pad))+
			app+ui.Style{Bg: t.Palette.Bg}.Render(" ")+t.Faint().Render(float))
	}
	return clampLines(out, h)
}

func (a *app) viewThemes(w, h int) []string {
	t := a.theme
	var out []string
	out = append(out, " "+t.Muted().Render("Enter aplica el tema a waybar, sway, foot, mako y swaylock"))
	out = append(out, "")
	for i, name := range ui.PaletteNames() {
		p := ui.PaletteByName(name)
		selected := i == a.cursor[2]
		bg := t.Palette.Bg
		if selected {
			bg = t.Palette.BgSel
		}
		swatch := ""
		for _, c := range []ui.Color{p.Bg, p.BgAlt, p.BgSel, p.Accent, p.Accent2, p.Ok, p.Warn, p.Err} {
			swatch += ui.Style{Fg: c, Bg: c}.Render("   ")
		}
		mark := "  "
		if name == a.cfg.Theme {
			mark = t.Accent().Render(t.Icons.Check + " ")
		}
		label := ui.Style{Fg: t.Palette.Fg, Bg: bg}.Render(ui.PadRight(" "+p.Name, 22))
		out = append(out, " "+mark+label+ui.Style{Bg: bg}.Render(" ")+swatch+
			ui.Style{Bg: bg}.Render(" ")+t.Muted().Render(name))
	}
	return clampLines(out, h)
}

func (a *app) viewKeys(w, h int) []string {
	t := a.theme
	groups := []struct {
		title string
		keys  [][2]string
	}{
		{"aplicaciones", [][2]string{
			{"Super+Return", "terminal"}, {"Super+d", "lanzador"},
			{"Super+Shift+d", "lanzador (comandos)"}, {"Super+b", "navegador"},
			{"Super+Shift+f", "archivos"}, {"Super+q", "cerrar ventana"},
		}},
		{"swaydesk", [][2]string{
			{"Super+p", "menú de energía"}, {"Super+t", "cambiar tema"},
			{"Super+c", "portapapeles"}, {"Super+i", "este panel"},
			{"Super+Shift+l", "bloquear"}, {"Print", "captura de pantalla"},
		}},
		{"ventanas", [][2]string{
			{"Super+h/j/k/l", "foco"}, {"Super+Shift+hjkl", "mover"},
			{"Super+r", "redimensionar"}, {"Super+f", "pantalla completa"},
			{"Super+Shift+space", "flotante"}, {"Super+e", "alternar split"},
			{"Super+w", "pestañas"}, {"Super+s", "apilado"},
		}},
		{"workspaces", [][2]string{
			{"Super+1..0", "ir a workspace"}, {"Super+Shift+1..0", "mover al workspace"},
			{"Super+Tab", "workspace anterior"}, {"Super+Shift+minus", "a scratchpad"},
			{"Super+minus", "mostrar scratchpad"}, {"Super+Shift+c", "recargar sway"},
		}},
	}
	var out []string
	for _, g := range groups {
		out = append(out, " "+t.Accent().Render(strings.ToUpper(g.title)))
		for _, k := range g.keys {
			out = append(out, "   "+ui.PadRight(t.Faint().Render(k[0]), 20)+t.Text().Render(k[1]))
		}
		out = append(out, "")
	}
	return clampLines(out, h)
}

func (a *app) footer() string {
	t := a.theme
	hints := [][2]string{
		{"Tab", "sección"}, {"↑↓", "navegar"},
	}
	switch a.tab {
	case 0:
		hints = append(hints, [2]string{"r", "actualizar"})
	case 1:
		hints = append(hints, [2]string{"↵", "entrar"})
	case 2:
		hints = append(hints, [2]string{"↵", "enfocar"}, [2]string{"x", "cerrar"}, [2]string{"f", "flotar"})
	case 3:
		hints = append(hints, [2]string{"↵", "aplicar"})
	}
	hints = append(hints, [2]string{"q", "salir"})
	f := t.HintBar(hints...)
	if a.status != "" && time.Since(a.statusT) < 4*time.Second {
		f = t.HintBar([2]string{"•", a.status})
	}
	return f
}

// ───────────────────────── utilidades de dibujo ─────────────────────────

func clampLines(lines []string, h int) []string {
	if len(lines) > h {
		return lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

func bar(value float64, width int, t *ui.Theme, color ui.Color) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	filled := int(value * float64(width) / 100)
	return ui.Style{Fg: color}.Render(strings.Repeat("█", filled)) +
		ui.Style{Fg: t.Palette.BgSel}.Render(strings.Repeat("░", width-filled))
}

func stateColor(v, warn, crit float64) ui.Color {
	switch {
	case v >= crit:
		return "#f38ba8"
	case v >= warn:
		return "#f9e2af"
	default:
		return "#a6e3a1"
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

func formatUptime(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func countWorkspaceWindows(root *swayipc.Node, wsName string) int {
	if root == nil {
		return 0
	}
	count := 0
	root.Walk(func(n *swayipc.Node) bool {
		if n.Type == "workspace" && n.Name == wsName {
			count = len(n.Leaves())
		}
		return true
	})
	return count
}
