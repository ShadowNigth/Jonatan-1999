package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/host"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/wayland"
)

// runBar crea la superficie y entra en el bucle de refresco.
func runBar(cfg *config.Config, r *bar.Renderer, scale int) error {
	conn, err := wayland.NewConn()
	if err != nil {
		return fmt.Errorf("no se pudo conectar con el compositor: %w", err)
	}
	if _, err := conn.GetRegistry(); err != nil {
		conn.Close()
		return fmt.Errorf("no se pudieron leer los globals de Wayland: %w", err)
	}

	anchors := uint32(wayland.AnchorTop | wayland.AnchorLeft | wayland.AnchorRight)
	margins := wayland.PanelSpec{}
	if cfg.BarPosition == "bottom" {
		anchors = wayland.AnchorBottom | wayland.AnchorLeft | wayland.AnchorRight
		margins.MarginBottom = cfg.BarMargin
	} else {
		margins.MarginTop = cfg.BarMargin
	}
	// El alto reservado incluye el margen para que las ventanas no se solapen.
	exclusive := cfg.BarHeight
	if cfg.BarExclusive {
		exclusive = cfg.BarHeight + cfg.BarMargin
	} else {
		exclusive = 0
	}
	margins.Namespace = "swaydesk-bar"
	margins.Layer = wayland.LayerTop
	margins.Height = cfg.BarHeight
	margins.Anchors = anchors
	margins.ExclusiveZone = exclusive

	panel, err := wayland.NewPanel(conn, margins)
	if err != nil {
		conn.Close()
		return err
	}
	defer panel.Close()
	if err := panel.InitShm(2); err != nil {
		return err
	}

	// Ratón: el indicador de notificaciones abre el centro en una terminal.
	clicks := make(chan string, 4)
	if p, err := wayland.NewPointer(conn, wayland.PointerHandler{
		Button: func(x, y float64, button uint32, pressed bool) {
			if !pressed || button != 0x110 {
				return
			}
			if reg, ok := r.RegionAt(int(x), int(y)); ok {
				select {
				case clicks <- reg.Name:
				default:
				}
			}
		},
	}); err == nil {
		defer p.Close()
	}

	stop := make(chan struct{})
	defer close(stop)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	defer signal.Stop(sig)

	collector := host.NewCollector()
	refresh := make(chan struct{}, 1)
	updates := make(chan bar.Snapshot, 1)
	requestRefresh := func() {
		select {
		case refresh <- struct{}{}:
		default:
		}
	}

	// La recolección puede consultar el bus, sway y herramientas del sistema.
	// Mantenerla fuera del bucle de Wayland evita que una orden lenta congele el
	// repintado o el despacho de eventos del compositor.
	go func() {
		tick := time.NewTicker(900 * time.Millisecond)
		defer tick.Stop()
		publish := func() {
			snap := snapshot(cfg, collector)
			select {
			case updates <- snap:
			default:
				// Si el dibujante está ocupado, conserva solo la foto más reciente.
				select {
				case <-updates:
				default:
				}
				select {
				case updates <- snap:
				default:
				}
			}
		}
		publish()
		for {
			select {
			case <-stop:
				return
			case <-refresh:
				publish()
			case <-tick.C:
				publish()
			}
		}
	}()

	var latest bar.Snapshot
	last := ""
	draw := func() error {
		w, h := panel.Size()
		if w <= 0 || h <= 0 {
			return nil
		}
		snap := latest
		snap.Clock = time.Now().Format("15:04")
		snap.Date = shortDate()
		// Se evita repintar si nada cambió (menos CPU y parpadeos).
		key := snapshotKey(w, snap)
		if key == last {
			return nil
		}
		canvas := r.Render(snap, w/scale)
		if scale > 1 {
			canvas = canvas.ScaleCanvas(scale)
		}
		if err := panel.Draw(canvas.Pix, canvas.W, canvas.H); err != nil {
			return err
		}
		last = key
		return nil
	}

	// Evita una barra vacía al inicio si la primera foto llega enseguida, pero
	// no retrasa el primer fotograma si una consulta del sistema está lenta.
	select {
	case latest = <-updates:
	case <-time.After(150 * time.Millisecond):
	}
	if err := draw(); err != nil {
		return err
	}

	// Eventos de sway: solicita una foto nueva, pero nunca dibuja desde el
	// goroutine de eventos ni espera al IPC en el bucle Wayland.
	events := make(chan struct{}, 1)
	go watchSway(stop, events)

	tick := time.NewTicker(900 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-sig:
			return nil
		case <-events:
			requestRefresh()
		case latest = <-updates:
			if err := draw(); err != nil {
				return err
			}
		case name := <-clicks:
			handleClick(cfg, name)
		case <-tick.C:
			if panel.NeedsResize() {
				_ = panel.Resize(2)
			}
			if err := draw(); err != nil {
				return err
			}
		}
		// Se procesan los eventos pendientes del compositor sin bloquear.
		_ = panel.Dispatch(0)
	}
}

// snapshotKey incluye los campos visibles de la barra para evitar repintados
// cuando el sondeo devuelve exactamente el mismo estado.
func snapshotKey(width int, s bar.Snapshot) string {
	var key strings.Builder
	fmt.Fprintf(&key, "%d|%q|%q|%q|%q|%.0f|%.0f|%.0f|%.0f|%q|%d|%t|%q|%d|%t|%t|%t|%d|%q|%d|%t|%t|%d|%d",
		width, s.Clock, s.Date, s.Title, s.AppID, s.CPU, s.RAM, s.Disk, s.Temp, s.Uptime,
		s.Volumen, s.Muted, s.Wifi, s.Signal, s.Connected, s.HasBattery, s.Charging, s.Battery,
		s.Media, s.Notifications, s.NotifDND, s.Recording, s.NotifReplies, s.Updates)
	for _, ws := range s.Workspaces {
		fmt.Fprintf(&key, "|%q:%t:%t:%t:%d", ws.Name, ws.Focused, ws.Visible, ws.Urgent, ws.Windows)
	}
	return key.String()
}

// handleClick ejecuta la acción de una zona pulsable de la barra.
func handleClick(cfg *config.Config, name string) {
	switch name {
	case "notify":
		// El centro necesita teclado: se abre en una terminal flotante.
		_ = launcher.LaunchTerminal(cfg.Terminal, "swaydesk-notify", "Notificaciones", 84, 24,
			[]string{siblingExecutable("swaydesk"), "notifications", "center"})
	}
}

func siblingExecutable(name string) string {
	if self, err := os.Executable(); err == nil {
		if sibling := filepath.Join(filepath.Dir(self), name); fileExists(sibling) {
			return sibling
		}
	}
	return name
}

// fileExists indica si existe la ruta.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// watchSway se suscribe a los cambios de workspaces y de foco en sway para
// repintar la barra sin esperar al siguiente tic.
func watchSway(stop <-chan struct{}, notify chan<- struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		c, err := swayipc.Dial()
		if err != nil {
			if waitForStop(stop, 3*time.Second) {
				return
			}
			continue
		}
		if err := c.Subscribe("workspace", "window", "output"); err != nil {
			c.Close()
			if waitForStop(stop, 3*time.Second) {
				return
			}
			continue
		}
		ch := c.Events(16)
		connected := true
		for connected {
			select {
			case <-stop:
				c.Close()
				return
			case _, ok := <-ch:
				if !ok {
					c.Close()
					connected = false
					continue
				}
				select {
				case notify <- struct{}{}:
				default:
				}
			}
		}
		if waitForStop(stop, time.Second) {
			return
		}
	}
}

// waitForStop duerme hasta el plazo indicado o hasta que el bucle se cancele.
func waitForStop(stop <-chan struct{}, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-stop:
		return true
	case <-timer.C:
		return false
	}
}

// shortDate devuelve la fecha corta en español: "vie 23 oct".
func shortDate() string {
	dias := [...]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"}
	meses := [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	now := time.Now()
	return fmt.Sprintf("%s %d %s", dias[int(now.Weekday())], now.Day(), meses[int(now.Month())-1])
}
