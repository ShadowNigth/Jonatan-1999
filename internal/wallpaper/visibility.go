package wallpaper

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

const visibilityPollInterval = 500 * time.Millisecond

var screenLockerProcesses = map[string]struct{}{
	"swaylock":        {},
	"swaylock-effects": {},
	"swaylock-effect":  {}, // nombre truncado a 15 caracteres en /proc/<pid>/comm
	"gtklock":          {},
	"waylock":          {},
	"hyprlock":         {},
	"i3lock":           {},
	"i3lock-color":     {},
	"xsecurelock":      {},
	"physlock":         {},
}

// wallpaperVisibilityStream publica cambios cuando el fondo deja de verse en
// todas las salidas (fullscreen) o la sesión queda bloqueada.
func wallpaperVisibilityStream(stop <-chan struct{}) <-chan bool {
	out := make(chan bool, 1)
	go func() {
		defer close(out)
		ticker := time.NewTicker(visibilityPollInterval)
		defer ticker.Stop()

		hidden := wallpaperObscured()
		out <- hidden
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				next := wallpaperObscured()
				if next == hidden {
					continue
				}
				hidden = next
				select {
				case out <- hidden:
				default:
					// Si el consumidor está ocupado, conserva siempre el estado más
					// reciente y descarta el cambio ya obsoleto.
					select {
					case <-out:
					default:
					}
					select {
					case out <- hidden:
					default:
					}
				}
			}
		}
	}()
	return out
}

func wallpaperObscured() bool {
	if screenLocked() {
		return true
	}
	return fullscreenCoversAllOutputs()
}

func screenLocked() bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		proc := filepath.Join("/proc", entry.Name())
		comm, err := os.ReadFile(filepath.Join(proc, "comm"))
		if err != nil {
			continue
		}
		if isScreenLockerProcess(string(comm)) {
			return true
		}
		cmdline, err := os.ReadFile(filepath.Join(proc, "cmdline"))
		if err != nil {
			continue
		}
		argv0, _, _ := strings.Cut(string(cmdline), "\x00")
		if isScreenLockerProcess(argv0) {
			return true
		}
	}
	return false
}

func isScreenLockerProcess(name string) bool {
	name = strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	_, ok := screenLockerProcesses[name]
	return ok
}

func fullscreenCoversAllOutputs() bool {
	conn, err := swayipc.Dial()
	if err != nil {
		return false
	}
	defer conn.Close()

	outputs, err := conn.Outputs()
	if err != nil {
		return false
	}
	workspaces, err := conn.Workspaces()
	if err != nil {
		return false
	}
	tree, err := conn.Tree()
	if err != nil {
		return false
	}
	return allActiveOutputsFullscreen(outputs, workspaces, tree)
}

func allActiveOutputsFullscreen(outputs []swayipc.Output, workspaces []swayipc.Workspace, tree *swayipc.Node) bool {
	active := make(map[string]struct{})
	for _, output := range outputs {
		if output.Active && output.Name != "" {
			active[output.Name] = struct{}{}
		}
	}
	if len(active) == 0 || tree == nil {
		return false
	}

	visibleWorkspaceOutput := make(map[string]string)
	visibleOutputs := make(map[string]struct{})
	for _, workspace := range workspaces {
		if !workspace.Visible || workspace.Name == "" {
			continue
		}
		if _, ok := active[workspace.Output]; !ok {
			continue
		}
		visibleWorkspaceOutput[workspace.Name] = workspace.Output
		visibleOutputs[workspace.Output] = struct{}{}
	}
	covered := make(map[string]struct{})
	globalFullscreen := false
	tree.Walk(func(node *swayipc.Node) bool {
		if node.Type != "workspace" {
			return true
		}
		output, visible := visibleWorkspaceOutput[node.Name]
		if !visible {
			return true
		}
		fullscreen := false
		node.Walk(func(child *swayipc.Node) bool {
			if child.FullscreenMode > 0 {
				fullscreen = true
				if child.FullscreenMode > 1 {
					globalFullscreen = true
					return false
				}
			}
			return true
		})
		if fullscreen {
			covered[output] = struct{}{}
		}
		return !globalFullscreen
	})
	if globalFullscreen {
		return true
	}
	// Si no podemos asociar un workspace visible a cada salida, no pausamos:
	// es preferible gastar algo de CPU a congelar un fondo que aún se ve.
	if len(visibleOutputs) != len(active) {
		return false
	}
	for output := range active {
		if _, ok := covered[output]; !ok {
			return false
		}
	}
	return true
}
