package wallpaper

import (
	"time"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

const visibilityPollInterval = 500 * time.Millisecond

// wallpaperVisibilityStream publica cambios cuando una ventana visible pasa a
// pantalla completa o deja de estarlo.
func wallpaperVisibilityStream(stop <-chan struct{}) <-chan bool {
	out := make(chan bool, 1)
	go func() {
		defer close(out)
		ticker := time.NewTicker(visibilityPollInterval)
		defer ticker.Stop()

		hidden := fullscreenWindowVisible()
		out <- hidden
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				next := fullscreenWindowVisible()
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

func fullscreenWindowVisible() bool {
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
	return anyVisibleFullscreen(outputs, workspaces, tree)
}

func anyVisibleFullscreen(outputs []swayipc.Output, workspaces []swayipc.Workspace, tree *swayipc.Node) bool {
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

	visibleWorkspaces := make(map[string]struct{})
	for _, workspace := range workspaces {
		if !workspace.Visible || workspace.Name == "" {
			continue
		}
		if _, active := activeOutputs[workspace.Output]; active {
			visibleWorkspaces[workspace.Name] = struct{}{}
		}
	}
	if len(visibleWorkspaces) == 0 {
		return false
	}

	fullscreen := false
	tree.Walk(func(node *swayipc.Node) bool {
		if node.Type != "workspace" {
			return true
		}
		if _, visible := visibleWorkspaces[node.Name]; !visible {
			return true
		}
		node.Walk(func(child *swayipc.Node) bool {
			if child.FullscreenMode > 0 {
				fullscreen = true
				return false
			}
			return true
		})
		return !fullscreen
	})
	return fullscreen
}
