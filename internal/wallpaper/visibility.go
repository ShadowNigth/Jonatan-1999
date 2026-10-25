package wallpaper

import (
	"time"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

const visibilityPollInterval = 500 * time.Millisecond

// wallpaperVisibilityStream publica cambios cuando todas las salidas activas
// quedan cubiertas por ventanas visibles a pantalla completa.
func wallpaperVisibilityStream(stop <-chan struct{}) <-chan bool {
	out := make(chan bool, 1)
	go func() {
		defer close(out)
		ticker := time.NewTicker(visibilityPollInterval)
		defer ticker.Stop()

		hidden := fullscreenCoversAllOutputs()
		out <- hidden
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				next := fullscreenCoversAllOutputs()
				if next == hidden {
					continue
				}
				hidden = next
				select {
				case out <- hidden:
				default:
					// Si el consumidor está ocupado, conserva el estado más
					// reciente y descarta el cambio obsoleto.
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
	if tree == nil {
		return false
	}
	activeOutputs := make(map[string]struct{})
	for _, output := range outputs {
		if output.Active && output.Name != "" {
			activeOutputs[output.Name] = struct{}{}
		}
	}
	if len(activeOutputs) == 0 {
		return false
	}

	visibleWorkspaceOutput := make(map[string]string)
	visibleOutputs := make(map[string]struct{})
	for _, workspace := range workspaces {
		if !workspace.Visible || workspace.Name == "" {
			continue
		}
		if _, active := activeOutputs[workspace.Output]; !active {
			continue
		}
		visibleWorkspaceOutput[workspace.Name] = workspace.Output
		visibleOutputs[workspace.Output] = struct{}{}
	}
	// Si falta información de un monitor activo, no asumir que el fondo quedó
	// cubierto: el wallpaper puede seguir visible allí.
	if len(visibleOutputs) != len(activeOutputs) {
		return false
	}

	coveredOutputs := make(map[string]struct{})
	globalFullscreen := false
	tree.Walk(func(node *swayipc.Node) bool {
		if node.Type != "workspace" {
			return true
		}
		output, visible := visibleWorkspaceOutput[node.Name]
		if !visible {
			return true
		}
		node.Walk(func(child *swayipc.Node) bool {
			if !isFullscreenWindow(child) {
				return true
			}
			if child.FullscreenMode > 1 {
				globalFullscreen = true
				return false
			}
			coveredOutputs[output] = struct{}{}
			return false
		})
		return !globalFullscreen
	})
	if globalFullscreen {
		return true
	}
	for output := range activeOutputs {
		if _, covered := coveredOutputs[output]; !covered {
			return false
		}
	}
	return true
}

// isFullscreenWindow excluye los nodos de workspace y los contenedores de
// distribución: solo una ventana hoja visible en fullscreen debe contar.
func isFullscreenWindow(node *swayipc.Node) bool {
	if node == nil || !node.IsWindow() || node.FullscreenMode <= 0 ||
		len(node.Nodes) != 0 || len(node.FloatingNodes) != 0 {
		return false
	}
	return node.Visible == nil || *node.Visible
}
