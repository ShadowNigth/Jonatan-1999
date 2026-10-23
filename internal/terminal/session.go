package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
	"github.com/ShadowNigth/Jonatan-1999/internal/ricer"
	"github.com/ShadowNigth/Jonatan-1999/internal/sysinfo"
	"github.com/ShadowNigth/Jonatan-1999/internal/ws"
)

type Handler struct {
	store     *config.Store
	collector *sysinfo.Collector
	hub       *ws.Hub
	startDir  string
}

func NewHandler(store *config.Store, collector *sysinfo.Collector, hub *ws.Hub, startDir string) *Handler {
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	return &Handler{
		store:     store,
		collector: collector,
		hub:       hub,
		startDir:  startDir,
	}
}

type clientMsg struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type serverMsg struct {
	Type   string `json:"type"`
	Output string `json:"output,omitempty"`
	Cwd    string `json:"cwd,omitempty"`
	Clear  bool   `json:"clear,omitempty"`
}

func (h *Handler) ServeWS(conn *ws.Conn) {
	defer conn.Close()

	cwd := h.startDir
	// Send welcome fastfetch on connect
	welcome := h.renderFastfetch() + "\n\x1b[38;5;245mEscribe \x1b[1;36mhelp\x1b[0m\x1b[38;5;245m para ver comandos de KitsuneDE & Arch Linux (\x1b[36mkitsune-ctl\x1b[38;5;245m, \x1b[36mpacman -Syu\x1b[38;5;245m, \x1b[36mfastfetch\x1b[38;5;245m).\x1b[0m\n"
	_ = conn.WriteJSON(serverMsg{
		Type:   "output",
		Output: welcome,
		Cwd:    prettyCwd(cwd),
	})

	for {
		raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg clientMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type == "ping" {
			_ = conn.WriteJSON(serverMsg{Type: "pong", Cwd: prettyCwd(cwd)})
			continue
		}
		cmdLine := strings.TrimSpace(msg.Command)
		if cmdLine == "" {
			_ = conn.WriteJSON(serverMsg{Type: "output", Output: "", Cwd: prettyCwd(cwd)})
			continue
		}
		if cmdLine == "clear" || cmdLine == "cls" {
			_ = conn.WriteJSON(serverMsg{Type: "output", Clear: true, Cwd: prettyCwd(cwd)})
			continue
		}

		out, nextCwd := h.executeCommand(cwd, cmdLine)
		cwd = nextCwd
		_ = conn.WriteJSON(serverMsg{
			Type:   "output",
			Output: out,
			Cwd:    prettyCwd(cwd),
		})
	}
}

func (h *Handler) executeCommand(cwd, cmdLine string) (string, string) {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return "", cwd
	}
	head := fields[0]
	args := fields[1:]
	if head == "sudo" && len(args) > 0 {
		if args[0] == "pacman" || args[0] == "kitsune-ctl" {
			head = args[0]
			args = args[1:]
		}
	}

	switch head {
	case "cd":
		target := h.startDir
		if len(args) > 0 {
			arg := args[0]
			if arg == "~" {
				if home, err := os.UserHomeDir(); err == nil {
					target = home
				}
			} else if strings.HasPrefix(arg, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					target = filepath.Join(home, arg[2:])
				}
			} else if filepath.IsAbs(arg) {
				target = arg
			} else {
				target = filepath.Join(cwd, arg)
			}
		}
		info, err := os.Stat(target)
		if err != nil || !info.IsDir() {
			return fmt.Sprintf("\x1b[31mcd: no existe el directorio: %s\x1b[0m\n", target), cwd
		}
		abs, _ := filepath.Abs(target)
		return "", abs

	case "fastfetch", "neofetch":
		return h.renderFastfetch(), cwd

	case "help", "ayuda":
		return renderHelp(), cwd

	case "kitsune-ctl":
		return h.handleKitsuneCtl(cwd, args), cwd

	case "pacman", "yay", "paru":
		return h.handleArchPkg(head, args, cwd), cwd
	}

	// Execute real Linux shell command
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", cmdLine)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"PATH=/usr/local/bin:/usr/bin:/bin:"+filepath.Join(h.startDir, "bin"),
	)
	output, err := cmd.CombinedOutput()
	text := string(output)
	if ctx.Err() == context.DeadlineExceeded {
		text += "\n\x1b[33m[comando finalizado tras límite de 12s — usa procesos en segundo plano si es interactivo]\x1b[0m\n"
	} else if err != nil && text == "" {
		text = fmt.Sprintf("\x1b[31m%s\x1b[0m\n", err.Error())
	}
	return text, cwd
}

func (h *Handler) handleKitsuneCtl(cwd string, args []string) string {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		return "\x1b[1;36mKitsuneDE Control CLI (Go)\x1b[0m\n" +
			"  \x1b[33mkitsune-ctl status\x1b[0m                     Ver estado actual del escritorio\n" +
			"  \x1b[33mkitsune-ctl theme list\x1b[0m                 Listar los 4 temas anime incluidos\n" +
			"  \x1b[33mkitsune-ctl theme set <id>\x1b[0m             Cambiar tema (neon-tokyo | sakura-lofi | crimson-shonen | eva-midnight)\n" +
			"  \x1b[33mkitsune-ctl theme next\x1b[0m                 Alternar al siguiente tema anime\n" +
			"  \x1b[33mkitsune-ctl layout <tiling|floating|monocle|toggle>\x1b[0m\n" +
			"  \x1b[33mkitsune-ctl workspace <1..5>\x1b[0m           Cambiar al espacio de trabajo 1..5\n" +
			"  \x1b[33mkitsune-ctl notify <mensaje>\x1b[0m           Enviar notificación estilo SwayNC al escritorio\n" +
			"  \x1b[33mkitsune-ctl export [directorio]\x1b[0m        Exportar dotfiles reales de Arch (Hyprland/Waybar/Kitty/Rofi)\n"
	}

	switch args[0] {
	case "status":
		s := h.store.Settings()
		t := h.store.ActiveTheme()
		return fmt.Sprintf(
			"\x1b[1;35m狐 KitsuneDE Daemon Status\x1b[0m\n"+
				"  Tema Activo : \x1b[1;36m%s\x1b[0m (%s)\n"+
				"  Layout WM   : \x1b[1;32m%s\x1b[0m (gaps_in=%d, gaps_out=%d)\n"+
				"  Workspace   : \x1b[1;33m%d\x1b[0m\n"+
				"  Partículas  : %v  |  Blur: %v\n",
			t.Name, t.ID, s.LayoutMode, s.GapsIn, s.GapsOut, s.ActiveWorkspace, s.ParticlesEnabled, s.BlurEnabled,
		)

	case "theme":
		if len(args) < 2 || args[1] == "list" {
			var b strings.Builder
			cur := h.store.Settings().ActiveTheme
			b.WriteString("\x1b[1;36mTemas Anime disponibles:\x1b[0m\n")
			for _, t := range h.store.Themes() {
				mark := "  "
				if t.ID == cur {
					mark = "\x1b[1;32m➜ \x1b[0m"
				}
				b.WriteString(fmt.Sprintf("%s\x1b[1m%-16s\x1b[0m %s — \x1b[38;5;245m%s\x1b[0m\n", mark, t.ID, t.JapaneseName, t.Name))
			}
			return b.String()
		}
		if args[1] == "next" {
			themes := h.store.Themes()
			cur := h.store.Settings().ActiveTheme
			nextID := themes[0].ID
			for i, t := range themes {
				if t.ID == cur {
					nextID = themes[(i+1)%len(themes)].ID
					break
				}
			}
			settings, theme, _ := h.store.SetTheme(nextID)
			h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": settings, "theme": theme}})
			return fmt.Sprintf("\x1b[1;32m✓ Tema cambiado a %s (%s)\x1b[0m\n", theme.Name, theme.JapaneseName)
		}
		if args[1] == "set" && len(args) >= 3 {
			settings, theme, err := h.store.SetTheme(args[2])
			if err != nil {
				return fmt.Sprintf("\x1b[31mError: %s\x1b[0m\n", err.Error())
			}
			h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": settings, "theme": theme}})
			return fmt.Sprintf("\x1b[1;32m✓ Tema aplicado en vivo: %s (%s)\x1b[0m\n", theme.Name, theme.JapaneseName)
		}
		return "\x1b[31mUso: kitsune-ctl theme [list | next | set <id>]\x1b[0m\n"

	case "layout":
		mode := "toggle"
		if len(args) >= 2 {
			mode = args[1]
		}
		settings, err := h.store.SetLayout(mode)
		if err != nil {
			return fmt.Sprintf("\x1b[31mError: %s\x1b[0m\n", err.Error())
		}
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": settings, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("\x1b[1;32m✓ Modo de ventanas cambiado a: %s\x1b[0m\n", strings.ToUpper(settings.LayoutMode))

	case "workspace", "ws":
		if len(args) < 2 {
			return "\x1b[31mUso: kitsune-ctl workspace <1..5>\x1b[0m\n"
		}
		n, _ := strconv.Atoi(args[1])
		settings, err := h.store.SetWorkspace(n)
		if err != nil {
			return fmt.Sprintf("\x1b[31mError: %s\x1b[0m\n", err.Error())
		}
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": settings, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("\x1b[1;32m✓ Cambiado al Workspace %d\x1b[0m\n", settings.ActiveWorkspace)

	case "notify":
		msg := "Notificación desde terminal KitsuneDE"
		if len(args) >= 2 {
			msg = strings.Join(args[1:], " ")
		}
		h.hub.Broadcast(ws.Event{
			Type: "notification",
			Payload: map[string]any{
				"title": "狐 Kitsune CLI",
				"body":  msg,
				"icon":  "sparkles",
			},
		})
		return fmt.Sprintf("\x1b[1;36m✓ Notificación enviada: %q\x1b[0m\n", msg)

	case "export":
		outDir := filepath.Join(cwd, "dotfiles-export")
		if len(args) >= 2 {
			if filepath.IsAbs(args[1]) {
				outDir = args[1]
			} else {
				outDir = filepath.Join(cwd, args[1])
			}
		}
		files, err := ricer.ExportToDir(outDir, h.store.ActiveTheme(), h.store.Settings())
		if err != nil {
			return fmt.Sprintf("\x1b[31mError exportando dotfiles: %v\x1b[0m\n", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("\x1b[1;32m✓ %d archivos de configuración Arch exportados a %s:\x1b[0m\n", len(files), outDir))
		for _, f := range files {
			b.WriteString("  • " + f + "\n")
		}
		return b.String()
	}

	return "\x1b[31mSubcomando desconocido. Ejecuta: kitsune-ctl help\x1b[0m\n"
}

func (h *Handler) handleArchPkg(bin string, args []string, cwd string) string {
	// If running on actual Arch Linux with /usr/bin/pacman and read-only query, call real binary
	if _, err := os.Stat("/usr/bin/" + bin); err == nil && len(args) > 0 {
		if strings.HasPrefix(args[0], "-Q") || args[0] == "--version" {
			cmd := exec.Command("/usr/bin/"+bin, args...)
			cmd.Dir = cwd
			out, _ := cmd.CombinedOutput()
			return string(out)
		}
	}

	if len(args) == 0 || args[0] == "-Syu" {
		return "\x1b[1;34m::\x1b[0m \x1b[1mSincronizando bases de datos de paquetes de Arch Linux...\x1b[0m\n" +
			" \x1b[32mcore\x1b[0m                  164.2 KiB  1.42 MiB/s 00:00 [\x1b[36m########################\x1b[0m] 100%\n" +
			" \x1b[32mextra\x1b[0m                   8.4 MiB  19.8 MiB/s 00:00 [\x1b[36m########################\x1b[0m] 100%\n" +
			" \x1b[35mkitsune-anime\x1b[0m          48.0 KiB   920 KiB/s 00:00 [\x1b[35m########################\x1b[0m] 100%\n" +
			"\x1b[1;34m::\x1b[0m \x1b[1mIniciando actualización completa del sistema...\x1b[0m\n" +
			" nada que hacer — \x1b[1;32mtu entorno KitsuneDE (Go + Arch Rice) está actualizado 狐\x1b[0m\n"
	}

	if args[0] == "-Ss" {
		q := ""
		if len(args) > 1 {
			q = strings.ToLower(args[1])
		}
		pkgs := []struct{ repo, name, ver, desc string }{
			{"kitsune-anime", "kitsune-de", "1.0.0-1", "Entorno de escritorio estilo Anime para Arch Linux escrito en Go"},
			{"extra", "hyprland", "0.46.2-1", "Compositor Wayland de mosaico dinámico con bordes animados"},
			{"extra", "waybar", "0.11.0-2", "Barra superior altamente personalizable para Wayland/Hyprland"},
			{"extra", "swww", "0.9.5-1", "Demonio eficiente de wallpapers animados para Wayland"},
			{"extra", "kitty", "0.38.1-1", "Emulador de terminal acelerado por GPU con soporte de imágenes"},
			{"extra", "fastfetch", "2.33.0-1", "Herramienta ultrarrápida de información del sistema estilo neofetch"},
			{"aur", "rofi-wayland", "1.7.5+wayland3-1", "Lanzador de aplicaciones y selector de ventanas"},
			{"aur", "catppuccin-gtk-theme-mocha", "1.0.3-1", "Tema GTK acogedor pastel oscuro para escritorios anime"},
		}
		var b strings.Builder
		for _, p := range pkgs {
			if q == "" || strings.Contains(p.name, q) || strings.Contains(strings.ToLower(p.desc), q) {
				b.WriteString(fmt.Sprintf("\x1b[1;35m%s/\x1b[1;37m%s \x1b[1;32m%s\x1b[0m [instalado]\n    %s\n", p.repo, p.name, p.ver, p.desc))
			}
		}
		if b.Len() == 0 {
			return "\x1b[33mNo se encontraron paquetes que coincidan con la búsqueda.\x1b[0m\n"
		}
		return b.String()
	}

	if strings.HasPrefix(args[0], "-S") && len(args) >= 2 {
		pkg := strings.Join(args[1:], " ")
		return fmt.Sprintf(
			"\x1b[1;34m::\x1b[0m Resolviendo dependencias para \x1b[1;36m%s\x1b[0m...\n"+
				"\x1b[1;34m::\x1b[0m Buscando conflictos entre paquetes...\n\n"+
				"Paquetes (1) \x1b[1m%s-latest\x1b[0m\n\n"+
				"Tamaño total de instalación:  18.40 MiB\n"+
				"\x1b[1;32m(1/1) instalando %s\x1b[0m                              [\x1b[36m########################\x1b[0m] 100%%\n"+
				"\x1b[1;35m✓ Listo en KitsuneDE (modo simulación fuera de Arch real; en Arch Linux usa ./arch/install.sh)\x1b[0m\n",
			pkg, pkg, pkg,
		)
	}

	return "\x1b[1;36mArch Pacman / Kitsune Wrapper\x1b[0m — prueba: \x1b[33mpacman -Syu\x1b[0m, \x1b[33mpacman -Ss anime\x1b[0m, \x1b[33myay -S swww\x1b[0m\n"
}

func (h *Handler) renderFastfetch() string {
	snap := h.collector.Collect()
	theme := h.store.ActiveTheme()
	settings := h.store.Settings()

	art := []string{
		"\x1b[1;36m                   -`                 \x1b[0m",
		"\x1b[1;36m                  .o+`                \x1b[0m",
		"\x1b[1;36m                 `ooo/                \x1b[0m",
		"\x1b[1;36m                `+oooo:               \x1b[0m",
		"\x1b[1;36m               `+oooooo:              \x1b[0m",
		"\x1b[1;36m               -+oooooo+:             \x1b[0m",
		"\x1b[1;35m             `/:-:++oooo+:            \x1b[0m",
		"\x1b[1;35m            `/++++/+++++++:           \x1b[0m",
		"\x1b[1;35m           `/++++++++++++++:          \x1b[0m",
		"\x1b[1;35m          `/+++ooooooooooooo/`        \x1b[0m",
		"\x1b[1;34m         ./ooosssso++osssssso+`       \x1b[0m",
		"\x1b[1;34m        .oossssso-````/ossssss+`      \x1b[0m",
		"\x1b[1;34m       -osssssso.      :ssssssso.     \x1b[0m",
		"\x1b[1;34m      :osssssss/   🦊   osssso+++.    \x1b[0m",
	}

	info := []string{
		fmt.Sprintf("\x1b[1;36m%s\x1b[0m@\x1b[1;35m%s\x1b[0m", snap.Username, snap.Hostname),
		"\x1b[38;5;240m──────────────────────────────────────────\x1b[0m",
		fmt.Sprintf("\x1b[1;36m󰣇 OS      \x1b[0m ➜  %s", snap.Distro),
		fmt.Sprintf("\x1b[1;36m󰌽 Kernel  \x1b[0m ➜  %s (%s)", snap.Kernel, snap.Arch),
		fmt.Sprintf("\x1b[1;35m󱂬 DE / WM \x1b[0m ➜  KitsuneDE [%s] (Motor Go)", strings.ToUpper(settings.LayoutMode)),
		fmt.Sprintf("\x1b[1;35m🎨 Tema   \x1b[0m ➜  %s (%s)", theme.Name, theme.JapaneseName),
		fmt.Sprintf("\x1b[1;34m🐹 Runtime\x1b[0m ➜  %s (%d goroutines)", snap.GoVersion, snap.Goroutines),
		fmt.Sprintf("\x1b[1;34m󰥔 Uptime  \x1b[0m ➜  %s", snap.UptimeHuman),
		fmt.Sprintf("\x1b[1;32m󰻠 CPU     \x1b[0m ➜  %s (%d cores @ %.1f%%)", snap.CPUModel, snap.CPUCores, snap.CPUPercent),
		fmt.Sprintf("\x1b[1;32m󰍛 Memoria \x1b[0m ➜  %d MiB / %d MiB (%.1f%%)", snap.MemUsedMB, snap.MemTotalMB, snap.MemPercent),
		fmt.Sprintf("\x1b[1;33m󰋊 Disco   \x1b[0m ➜  %.1f GiB / %.1f GiB (%.1f%%)", snap.DiskUsedGB, snap.DiskTotalGB, snap.DiskPercent),
		"",
		"  \x1b[40m   \x1b[41m   \x1b[42m   \x1b[43m   \x1b[44m   \x1b[45m   \x1b[46m   \x1b[47m   \x1b[0m",
		"  \x1b[100m   \x1b[101m   \x1b[102m   \x1b[103m   \x1b[104m   \x1b[105m   \x1b[106m   \x1b[107m   \x1b[0m",
	}

	var b strings.Builder
	maxLines := len(art)
	if len(info) > maxLines {
		maxLines = len(info)
	}
	for i := 0; i < maxLines; i++ {
		left := "                                      "
		if i < len(art) {
			left = art[i]
		}
		right := ""
		if i < len(info) {
			right = info[i]
		}
		b.WriteString(left + "  " + right + "\n")
	}
	return b.String()
}

func renderHelp() string {
	return "\x1b[1;35m╭─── 狐 KitsuneDE // Terminal Arch Linux Anime (Go) ───────────────────╮\x1b[0m\n" +
		"  \x1b[1;36mfastfetch\x1b[0m                       Mostrar resumen del sistema e icono Arch\n" +
		"  \x1b[1;36mkitsune-ctl theme list\x1b[0m          Ver los 4 temas anime incluidos\n" +
		"  \x1b[1;36mkitsune-ctl theme next\x1b[0m          Cambiar de tema anime en tiempo real\n" +
		"  \x1b[1;36mkitsune-ctl layout toggle\x1b[0m       Alternar entre Tiling, Floating y Monocle\n" +
		"  \x1b[1;36mkitsune-ctl export ./dotfiles\x1b[0m   Generar configs reales de Hyprland/Waybar/Kitty\n" +
		"  \x1b[1;36mpacman -Syu\x1b[0m / \x1b[1;36mpacman -Ss anime\x1b[0m  Gestor de paquetes Arch Linux\n" +
		"  \x1b[1;36mls -la\x1b[0m / \x1b[1;36mgo version\x1b[0m / \x1b[1;36muname -a\x1b[0m  Comandos reales de Linux en el servidor\n" +
		"\x1b[1;35m╰──────────────────────────────────────────────────────────────────────╯\x1b[0m\n"
}

func prettyCwd(cwd string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(cwd, home) {
		rel := strings.TrimPrefix(cwd, home)
		if rel == "" {
			return "~"
		}
		return "~" + rel
	}
	return cwd
}
