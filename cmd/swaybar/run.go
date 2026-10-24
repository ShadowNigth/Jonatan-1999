package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

	// Ratón: el indicador de notificaciones abre el centro y el botón OFF
	// abre el menú de energía; ambos usan terminales flotantes.
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

	stop := make(chan struct{}, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

	collector := host.NewCollector()
	// El primer muestreo de CPU necesita dos lecturas separadas.
	_ = collector.Snapshot()

	tick := time.NewTicker(900 * time.Millisecond)
	defer tick.Stop()

	last := ""
	draw := func() error {
		w, h := panel.Size()
		if w <= 0 || h <= 0 {
			return nil
		}
		snap := snapshot(cfg, collector)
		snap.Clock = time.Now().Format("15:04")
		snap.Date = shortDate()
		canvas := r.Render(snap, w/scale)
		if scale > 1 {
			canvas = canvas.ScaleCanvas(scale)
		}
		// Se evita repintar si nada cambió (menos CPU y parpadeos).
		key := fmt.Sprintf("%d|%s|%s|%s|%.0f|%.0f|%d|%v|%d|%d|%v|%d",
			w, snap.Clock, snap.Title, snap.AppID, snap.CPU, snap.RAM, snap.Volumen, snap.Muted,
			len(snap.Workspaces), snap.Notifications, snap.NotifDND, snap.NotifReplies)
		if key == last {
			return nil
		}
		last = key
		return panel.Draw(canvas.Pix, canvas.W, canvas.H)
	}

	if err := draw(); err != nil {
		return err
	}

	// Eventos de sway: se refresca al instante al cambiar de workspace o foco.
	events := make(chan struct{}, 1)
	go watchSway(stop, events)

	for {
		select {
		case <-stop:
			return nil
		case <-events:
			_ = draw()
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

// handleClick ejecuta la acción de una zona pulsable de la barra.
func handleClick(cfg *config.Config, name string) {
	switch name {
	case "notify":
		// El centro necesita teclado: se abre en una terminal flotante.
		_ = launcher.LaunchTerminal(cfg.Terminal, "swaydesk-notify", "Notificaciones", 84, 24,
			[]string{siblingExecutable("swaydesk"), "notifications", "center"})
	case "power":
		// Fuera de una terminal, swaymenu se relanza automáticamente en una
		// terminal flotante para que el selector pueda leer el teclado.
		_ = launcher.StartDetached(siblingExecutable("swaymenu"), "--power", "--float")
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
			time.Sleep(3 * time.Second)
			continue
		}
		if err := c.Subscribe("workspace", "window"); err != nil {
			c.Close()
			time.Sleep(3 * time.Second)
			continue
		}
		ch := c.Events(16)
		if err != nil {
			c.Close()
			time.Sleep(3 * time.Second)
			continue
		}
		for {
			select {
			case <-stop:
				c.Close()
				return
			case _, ok := <-ch:
				if !ok {
					c.Close()
					goto reconnect
				}
				select {
				case notify <- struct{}{}:
				default:
				}
			}
		}
	reconnect:
		time.Sleep(time.Second)
	}
}

// shortDate devuelve la fecha corta en español: "vie 23 oct".
func shortDate() string {
	dias := [...]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"}
	meses := [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	now := time.Now()
	return fmt.Sprintf("%s %d %s", dias[int(now.Weekday())], now.Day(), meses[int(now.Month())-1])
}
