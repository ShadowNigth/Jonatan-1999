package wallpaper

import (
	"time"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

const (
	visibilityPollInterval = 5 * time.Second
	visibilityDebounce     = 75 * time.Millisecond
)

// wallpaperVisibilityStream publica cambios cuando todas las salidas activas
// quedan cubiertas por ventanas visibles a pantalla completa. Sway avisa de los
// cambios relevantes; un sondeo lento sirve de respaldo si se pierde una señal.
func wallpaperVisibilityStream(stop <-chan struct{}) <-chan bool {
	out := make(chan bool, 1)
	go func() {
		defer close(out)
		ticker := time.NewTicker(visibilityPollInterval)
		defer ticker.Stop()

		last, initialized := false, false
		publishState := func(next bool) {
			if initialized && next == last {
				return
			}
			last, initialized = next, true
			select {
			case out <- next:
			default:
				// Conserva el estado más reciente si el consumidor aún está
				// aplicando la transición anterior.
				select {
				case <-out:
				default:
				}
				select {
				case out <- next:
				default:
				}
			}
		}
		publish := func() { publishState(fullscreenCoversAllOutputs()) }
		publish()

		for {
			select {
			case <-stop:
				return
			default:
			}
			conn, err := swayipc.Dial()
			if err != nil {
				// Sin respuesta del compositor, no se asume que todas las salidas
				// estén cubiertas: el wallpaper debe seguir visible.
				publishState(false)
				if waitUntilStop(stop, 2*time.Second) {
					return
				}
				continue
			}
			if err := conn.Subscribe("workspace", "window", "output"); err != nil {
				conn.Close()
				publish()
				if waitUntilStop(stop, 2*time.Second) {
					return
				}
				continue
			}
			// Reconsulta tras suscribirse para cubrir cualquier cambio ocurrido
			// entre la primera lectura y la instalación del listener.
			publish()
			events := conn.Events(32)
			debounce := time.NewTimer(time.Hour)
			if !debounce.Stop() {
				<-debounce.C
			}
			var debounceC <-chan time.Time
			connected := true
			for connected {
				select {
				case <-stop:
					conn.Close()
					debounce.Stop()
					return
				case event, ok := <-events:
					if !ok {
						conn.Close()
						publish()
						connected = false
						continue
					}
					switch event.Type {
					case swayipc.EventWorkspace, swayipc.EventWindow, swayipc.EventOutput:
						if !debounce.Stop() {
							select {
							case <-debounce.C:
							default:
							}
						}
						debounce.Reset(visibilityDebounce)
						debounceC = debounce.C
					}
				case <-debounceC:
					debounceC = nil
					publish()
				case <-ticker.C:
					publish()
				}
			}
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			if waitUntilStop(stop, time.Second) {
				return
			}
		}
	}()
	return out
}

func waitUntilStop(stop <-chan struct{}, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-stop:
		return true
	case <-timer.C:
		return false
	}
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
