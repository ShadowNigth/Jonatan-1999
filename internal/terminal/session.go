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
	"github.com/ShadowNigth/Jonatan-1999/internal/swayipc"
	"github.com/ShadowNigth/Jonatan-1999/internal/sysinfo"
	"github.com/ShadowNigth/Jonatan-1999/internal/ws"
)

type Handler struct {
	store     *config.Store
	collector *sysinfo.Collector
	sway      *swayipc.Client
	hub       *ws.Hub
	startDir  string
}

func NewHandler(store *config.Store, collector *sysinfo.Collector, sway *swayipc.Client, hub *ws.Hub, startDir string) *Handler {
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	return &Handler{
		store:     store,
		collector: collector,
		sway:      sway,
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
	welcome := h.renderFastfetch() + "\n\x1b[38;5;245mEscribe \x1b[1;36mhelp\x1b[0m\x1b[38;5;245m para ver comandos de Sway/SwayFX & KitsuneDE (\x1b[36mswaymsg\x1b[38;5;245m, \x1b[36mkitsune-ctl\x1b[38;5;245m, \x1b[36myay -S swayfx\x1b[38;5;245m).\x1b[0m\n"
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

	case "swaymsg":
		return h.handleSwayMsg(args), cwd

	case "kitsune-ctl":
		return h.handleKitsuneCtl(cwd, args), cwd

	case "pacman", "yay", "paru":
		return h.handleArchPkg(head, args, cwd), cwd
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", cmdLine)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"PATH=/usr/local/bin:/usr/bin:/bin:"+filepath.Join(h.startDir, "build"),
	)
	output, err := cmd.CombinedOutput()
	text := string(output)
	if ctx.Err() == context.DeadlineExceeded {
		text += "\n\x1b[33m[comando finalizado tras límite de 12s]\x1b[0m\n"
	} else if err != nil && text == "" {
		text = fmt.Sprintf("\x1b[31m%s\x1b[0m\n", err.Error())
	}
	return text, cwd
}

func (h *Handler) handleSwayMsg(args []string) string {
	// If running inside real Sway/SwayFX on Arch Linux, forward over $SWAYSOCK!
	if h.sway != nil && h.sway.Available() {
		if len(args) >= 2 && args[0] == "-t" {
			switch args[1] {
			case "get_version":
				b, err := h.sway.Send(swayipc.MsgGetVersion, "")
				if err == nil {
					return string(b) + "\n"
				}
			case "get_workspaces":
				b, err := h.sway.Send(swayipc.MsgGetWorkspaces, "")
				if err == nil {
					return string(b) + "\n"
				}
			}
		} else if len(args) > 0 {
			b, err := h.sway.RunCommand(strings.Join(args, " "))
			if err == nil {
				return string(b) + "\n"
			}
		}
	}

	// Interactive Sway / SwayFX IPC bridge in KitsuneDE
	s := h.store.Settings()
	if len(args) == 0 {
		return "\x1b[1;36mswaymsg (KitsuneDE Go sway-ipc bridge)\x1b[0m\n" +
			"  Prueba:\n" +
			"    \x1b[33mswaymsg -t get_version\x1b[0m\n" +
			"    \x1b[33mswaymsg -t get_workspaces\x1b[0m\n" +
			"    \x1b[33mswaymsg corner_radius 18\x1b[0m         (SwayFX)\n" +
			"    \x1b[33mswaymsg blur toggle\x1b[0m              (SwayFX)\n" +
			"    \x1b[33mswaymsg gaps inner all set 12\x1b[0m    (Sway/SwayFX)\n" +
			"    \x1b[33mswaymsg layout tabbed\x1b[0m            (Contenedor con pestañas Sway)\n"
	}

	if len(args) >= 2 && args[0] == "-t" {
		switch args[1] {
		case "get_version":
			variant := "swayfx"
			human := "swayfx version 0.4.0 (based on sway 1.10, KitsuneDE Go IPC)"
			if s.Compositor == "sway" {
				variant = "sway"
				human = "sway version 1.10 (KitsuneDE Go IPC)"
			}
			out, _ := json.MarshalIndent(map[string]any{
				"major":                   1,
				"minor":                   10,
				"patch":                   0,
				"human_readable":          human,
				"variant":                 variant,
				"loaded_config_file_name": "~/.config/sway/config",
			}, "", "  ")
			return string(out) + "\n"

		case "get_workspaces":
			kanji := []string{"一", "二", "三", "四", "五"}
			var list []map[string]any
			for i := 1; i <= 5; i++ {
				list = append(list, map[string]any{
					"num":     i,
					"name":    fmt.Sprintf("%d:%s", i, kanji[i-1]),
					"focused": i == s.ActiveWorkspace,
					"visible": i == s.ActiveWorkspace,
					"layout":  s.LayoutMode,
					"output":  "WL-1",
				})
			}
			out, _ := json.MarshalIndent(list, "", "  ")
			return string(out) + "\n"
		}
	}

	joined := strings.ToLower(strings.Join(args, " "))
	switch {
	case strings.HasPrefix(joined, "corner_radius"):
		n, _ := strconv.Atoi(args[len(args)-1])
		s.BorderRadius = n
		updated, _ := h.store.UpdateSettings(s)
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": updated, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("[{\"success\": true, \"swayfx\": \"corner_radius %d\"}]\n", updated.BorderRadius)

	case strings.HasPrefix(joined, "blur"):
		if strings.Contains(joined, "disable") {
			s.BlurEnabled = false
		} else if strings.Contains(joined, "enable") {
			s.BlurEnabled = true
		} else {
			s.BlurEnabled = !s.BlurEnabled
		}
		updated, _ := h.store.UpdateSettings(s)
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": updated, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("[{\"success\": true, \"swayfx_blur\": %v}]\n", updated.BlurEnabled)

	case strings.HasPrefix(joined, "gaps inner"):
		n, _ := strconv.Atoi(args[len(args)-1])
		s.GapsIn = n
		updated, _ := h.store.UpdateSettings(s)
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": updated, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("[{\"success\": true, \"gaps_inner\": %d}]\n", updated.GapsIn)

	case strings.HasPrefix(joined, "gaps outer"):
		n, _ := strconv.Atoi(args[len(args)-1])
		s.GapsOut = n
		updated, _ := h.store.UpdateSettings(s)
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": updated, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("[{\"success\": true, \"gaps_outer\": %d}]\n", updated.GapsOut)

	case strings.HasPrefix(joined, "layout"):
		mode := "tiling"
		if strings.Contains(joined, "tabbed") {
			mode = "tabbed"
		} else if strings.Contains(joined, "toggle") {
			mode = "toggle"
		}
		updated, _ := h.store.SetLayout(mode)
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": updated, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("[{\"success\": true, \"layout\": %q}]\n", updated.LayoutMode)

	case strings.HasPrefix(joined, "workspace"):
		n, _ := strconv.Atoi(args[len(args)-1])
		updated, err := h.store.SetWorkspace(n)
		if err != nil {
			return fmt.Sprintf("[{\"success\": false, \"error\": %q}]\n", err.Error())
		}
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": updated, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("[{\"success\": true, \"workspace\": %d}]\n", updated.ActiveWorkspace)
	}

	return "[{\"success\": true}]\n"
}

func (h *Handler) handleKitsuneCtl(cwd string, args []string) string {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		return "\x1b[1;36mKitsuneDE Control CLI (Go + Sway/SwayFX IPC)\x1b[0m\n" +
			"  \x1b[33mkitsune-ctl status\x1b[0m                              Ver estado actual y compositor activo\n" +
			"  \x1b[33mkitsune-ctl compositor <swayfx|sway|hyprland>\x1b[0m   Elegir perfil de compositor\n" +
			"  \x1b[33mkitsune-ctl theme [list|next|set <id>]\x1b[0m          Gestionar temas anime\n" +
			"  \x1b[33mkitsune-ctl layout <tiling|tabbed|floating|monocle|toggle>\x1b[0m\n" +
			"  \x1b[33mkitsune-ctl workspace <1..5>\x1b[0m                    Cambiar al workspace 1..5\n" +
			"  \x1b[33mkitsune-ctl notify <mensaje>\x1b[0m                    Enviar notificación estilo SwayNC\n" +
			"  \x1b[33mkitsune-ctl export [directorio]\x1b[0m                 Exportar configs Sway/SwayFX/Waybar\n"
	}

	switch args[0] {
	case "status":
		s := h.store.Settings()
		t := h.store.ActiveTheme()
		return fmt.Sprintf(
			"\x1b[1;35m狐 KitsuneDE Daemon Status\x1b[0m\n"+
				"  Compositor  : \x1b[1;36m%s\x1b[0m ($SWAYSOCK activo: %v)\n"+
				"  Tema Activo : \x1b[1;36m%s\x1b[0m (%s)\n"+
				"  Layout WM   : \x1b[1;32m%s\x1b[0m (gaps inner=%d, outer=%d, radius=%d)\n"+
				"  Workspace   : \x1b[1;33m%d\x1b[0m\n",
			strings.ToUpper(s.Compositor), h.sway != nil && h.sway.Available(),
			t.Name, t.ID, s.LayoutMode, s.GapsIn, s.GapsOut, s.BorderRadius, s.ActiveWorkspace,
		)

	case "compositor", "comp":
		comp := "toggle"
		if len(args) >= 2 {
			comp = strings.ToLower(args[1])
		}
		settings, err := h.store.SetCompositor(comp)
		if err != nil {
			return fmt.Sprintf("\x1b[31mError: %s\x1b[0m\n", err.Error())
		}
		if h.sway != nil && h.sway.Available() {
			_ = h.sway.ApplyLiveState(h.store.ActiveTheme(), settings)
		}
		h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": settings, "theme": h.store.ActiveTheme()}})
		return fmt.Sprintf("\x1b[1;32m✓ Perfil de compositor cambiado a: %s\x1b[0m\n", strings.ToUpper(settings.Compositor))

	case "theme":
		if len(args) < 2 || args[1] == "list" {
			var b strings.Builder
			cur := h.store.Settings().ActiveTheme
			b.WriteString("\x1b[1;36mTemas Anime disponibles para Sway / SwayFX:\x1b[0m\n")
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
			if h.sway != nil && h.sway.Available() {
				h.sway.SyncNativeDesktop(theme, settings)
			}
			h.hub.Broadcast(ws.Event{Type: "state_updated", Payload: map[string]any{"settings": settings, "theme": theme}})
			return fmt.Sprintf("\x1b[1;32m✓ Tema cambiado a %s (%s)\x1b[0m\n", theme.Name, theme.JapaneseName)
		}
		if args[1] == "set" && len(args) >= 3 {
			settings, theme, err := h.store.SetTheme(args[2])
			if err != nil {
				return fmt.Sprintf("\x1b[31mError: %s\x1b[0m\n", err.Error())
			}
			if h.sway != nil && h.sway.Available() {
				h.sway.SyncNativeDesktop(theme, settings)
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
		return fmt.Sprintf("\x1b[1;32m✓ Modo de ventanas Sway cambiado a: %s\x1b[0m\n", strings.ToUpper(settings.LayoutMode))

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
		msg := "Notificación SwayNC desde KitsuneDE"
		if len(args) >= 2 {
			msg = strings.Join(args[1:], " ")
		}
		h.hub.Broadcast(ws.Event{
			Type: "notification",
			Payload: map[string]any{
				"title": "狐 Kitsune SwayNC",
				"body":  msg,
				"icon":  "sparkles",
			},
		})
		return fmt.Sprintf("\x1b[1;36m✓ Notificación enviada: %q\x1b[0m\n", msg)

	case "export":
		outDir := filepath.Join(cwd, "dotfiles-sway-export")
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
		b.WriteString(fmt.Sprintf("\x1b[1;32m✓ %d archivos Sway / SwayFX / Waybar exportados a %s:\x1b[0m\n", len(files), outDir))
		for _, f := range files {
			b.WriteString("  • " + f + "\n")
		}
		return b.String()
	}

	return "\x1b[31mSubcomando desconocido. Ejecuta: kitsune-ctl help\x1b[0m\n"
}

func (h *Handler) handleArchPkg(bin string, args []string, cwd string) string {
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
			" \x1b[35mchaotic-aur\x1b[0m             2.1 MiB   9.4 MiB/s 00:00 [\x1b[35m########################\x1b[0m] 100%\n" +
			"\x1b[1;34m::\x1b[0m \x1b[1mIniciando actualización completa del sistema...\x1b[0m\n" +
			" nada que hacer — \x1b[1;32mtu entorno KitsuneDE + SwayFX (Go) está actualizado 狐\x1b[0m\n"
	}

	if args[0] == "-Ss" {
		q := ""
		if len(args) > 1 {
			q = strings.ToLower(args[1])
		}
		pkgs := []struct{ repo, name, ver, desc string }{
			{"kitsune", "kitsune-de", "1.1.0-1", "Escritorio y demonio sway-ipc estilo Anime en Go para Sway y SwayFX"},
			{"aur", "swayfx", "0.4.0-2", "Sway con corner_radius, blur, shadows y layer_effects estilo anime"},
			{"extra", "sway", "1.10-1", "Compositor Wayland ligero compatible con i3 e integrado con $SWAYSOCK"},
			{"extra", "swaybg", "1.2.1-1", "Herramienta de fondos de pantalla para compositores Wayland / Sway"},
			{"aur", "swaylock-effects-git", "1.7.0-1", "Pantalla de bloqueo para Sway/SwayFX con blur e indicador circular"},
			{"extra", "autotiling", "2.1.2-1", "División automática espiral splith/splitv para Sway y SwayFX"},
			{"extra", "waybar", "0.11.0-2", "Barra superior con soporte nativo para sway/workspaces y sway/mode"},
			{"aur", "swaync", "0.10.1-2", "Centro de notificaciones para Sway y SwayFX"},
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
				"Paquetes (1) \x1b[1m%s-latest\x1b[0m\n\n"+
				"\x1b[1;32m(1/1) instalando %s\x1b[0m                              [\x1b[36m########################\x1b[0m] 100%%\n"+
				"\x1b[1;35m✓ Listo en KitsuneDE (en tu Arch Linux real ejecuta ./INSTALAR_EN_ARCH.sh --compositor swayfx)\x1b[0m\n",
			pkg, pkg, pkg,
		)
	}

	return "\x1b[1;36mArch Pacman / Yay Wrapper\x1b[0m — prueba: \x1b[33myay -S swayfx\x1b[0m, \x1b[33mpacman -Ss sway\x1b[0m, \x1b[33mpacman -Syu\x1b[0m\n"
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

	compLabel := strings.ToUpper(settings.Compositor)
	if compLabel == "" {
		compLabel = "SWAYFX"
	}

	info := []string{
		fmt.Sprintf("\x1b[1;36m%s\x1b[0m@\x1b[1;35m%s\x1b[0m", snap.Username, snap.Hostname),
		"\x1b[38;5;240m──────────────────────────────────────────────\x1b[0m",
		fmt.Sprintf("\x1b[1;36m󰣇 OS         \x1b[0m ➜  %s", snap.Distro),
		fmt.Sprintf("\x1b[1;36m󰌽 Kernel     \x1b[0m ➜  %s (%s)", snap.Kernel, snap.Arch),
		fmt.Sprintf("\x1b[1;35m󰖲 Compositor \x1b[0m ➜  KitsuneDE + %s [%s]", compLabel, strings.ToUpper(settings.LayoutMode)),
		fmt.Sprintf("\x1b[1;35m🎨 Tema Anime \x1b[0m ➜  %s (%s)", theme.Name, theme.JapaneseName),
		fmt.Sprintf("\x1b[1;34m🐹 Motor IPC  \x1b[0m ➜  %s (sway-ipc + WebSocket)", snap.GoVersion),
		fmt.Sprintf("\x1b[1;34m󰥔 Uptime     \x1b[0m ➜  %s", snap.UptimeHuman),
		fmt.Sprintf("\x1b[1;32m󰻠 CPU        \x1b[0m ➜  %s (%d cores @ %.1f%%)", snap.CPUModel, snap.CPUCores, snap.CPUPercent),
		fmt.Sprintf("\x1b[1;32m󰍛 Memoria    \x1b[0m ➜  %d MiB / %d MiB (%.1f%%)", snap.MemUsedMB, snap.MemTotalMB, snap.MemPercent),
		fmt.Sprintf("\x1b[1;33m󰋊 Disco      \x1b[0m ➜  %.1f GiB / %.1f GiB (%.1f%%)", snap.DiskUsedGB, snap.DiskTotalGB, snap.DiskPercent),
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
	return "\x1b[1;35m╭─── 狐 KitsuneDE // Terminal Arch Linux Sway & SwayFX (Go) ───────────╮\x1b[0m\n" +
		"  \x1b[1;36mswaymsg -t get_version\x1b[0m          Consultar versión IPC de Sway / SwayFX\n" +
		"  \x1b[1;36mswaymsg corner_radius 18\x1b[0m        Ajustar redondeo SwayFX en tiempo real\n" +
		"  \x1b[1;36mswaymsg layout tabbed\x1b[0m           Activar contenedor con pestañas de Sway\n" +
		"  \x1b[1;36mkitsune-ctl compositor swayfx\x1b[0m   Cambiar perfil (swayfx | sway | hyprland)\n" +
		"  \x1b[1;36mkitsune-ctl theme next\x1b[0m          Cambiar de tema anime en vivo\n" +
		"  \x1b[1;36mkitsune-ctl export ./dotfiles\x1b[0m   Generar configs de Sway, SwayFX y Waybar\n" +
		"  \x1b[1;36myay -S swayfx\x1b[0m / \x1b[1;36mpacman -Ss sway\x1b[0m Gestor de paquetes Arch Linux\n" +
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
