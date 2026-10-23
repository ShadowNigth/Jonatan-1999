package wallpaper

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Options ajusta la ejecución de un fondo animado.
type Options struct {
	// FPS limita los fotogramas por segundo (0 = los del motor).
	FPS int
	// Protocol es el protocolo de imágenes del terminal (solo modo terminal).
	Protocol ImageProtocol
	// ShowInfo dibuja un cartel con la ayuda de teclas (modo terminal).
	ShowInfo bool
	// Scale reduce la resolución interna (1 = nativa, 2 = mitad).
	Scale int
}

// ANSIFrame convierte un lienzo en un bloque de texto con medios bloques.
func ANSIFrame(c *Canvas) string {
	var b strings.Builder
	b.Grow(c.W * c.H * 8)
	for row := 0; row < c.H/2; row++ {
		for col := 0; col < c.W; col++ {
			i1 := (row*2*c.W + col) * 3
			i2 := ((row*2+1)*c.W + col) * 3
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				c.Pix[i1], c.Pix[i1+1], c.Pix[i1+2],
				c.Pix[i2], c.Pix[i2+1], c.Pix[i2+2])
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String()
}

// PreviewBlock devuelve el fondo dibujado en exactamente w×h celdas, listo para
// insertarlo en un panel de la interfaz.
func PreviewBlock(eng *Engine, w, h int) []string {
	if w < 2 || h < 2 {
		return nil
	}
	c := eng.Frame(w, h*2)
	lines := make([]string, 0, h)
	for row := 0; row < h; row++ {
		var b strings.Builder
		for col := 0; col < w; col++ {
			i1 := (row*2*c.W + col) * 3
			i2 := ((row*2+1)*c.W + col) * 3
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				c.Pix[i1], c.Pix[i1+1], c.Pix[i1+2],
				c.Pix[i2], c.Pix[i2+1], c.Pix[i2+2])
		}
		b.WriteString("\x1b[0m")
		lines = append(lines, b.String())
	}
	return lines
}

// RunTerminal reproduce el fondo a pantalla completa en la terminal actual.
//
// Teclas: q/Esc salir, espacio pausar, + y − ajustar FPS, i ocultar la ayuda.
func RunTerminal(eng *Engine, opts Options) error {
	sc, err := ui.OpenScreen()
	if err != nil {
		return err
	}
	defer sc.Close()

	fps := opts.FPS
	if fps <= 0 {
		fps = int(time.Second / eng.Interval())
	}
	if fps <= 0 {
		fps = 24
	}
	paused := false
	showInfo := opts.ShowInfo
	scale := opts.Scale
	if scale < 1 {
		scale = 1
	}
	keys := sc.Keys.Keys()

	render := func() string {
		w, h := sc.W, sc.H
		if showInfo && h > 4 {
			h -= 2
		}
		if scale > 1 {
			w = w / scale
			h = h / scale
		}
		cw, ch := eng.Size(w, h)
		frame := ANSIFrame(eng.Frame(cw, ch))
		if !showInfo {
			return frame
		}
		t := eng.Spec()
		kind := string(t.Kind)
		detail := eng.Describe()
		bar := ui.Style{Fg: eng.Spec().Pal.Accent, Bg: eng.Spec().Pal.Bg}.Render("▎")
		info := " " + bar + " " + ui.Style{Fg: eng.Spec().Pal.Fg, Bg: eng.Spec().Pal.Bg}.B().Render(detail) +
			ui.Style{Fg: eng.Spec().Pal.Muted, Bg: eng.Spec().Pal.Bg}.Render(
				fmt.Sprintf("  ·  %s · %d fps · %s", kind, fps, pauseLabel(paused)))
		help := ui.Style{Fg: eng.Spec().Pal.Muted, Bg: eng.Spec().Pal.Bg}.Render(
			" q salir · espacio pausar · +/− velocidad · i ocultar ")
		return frame + ui.PadRight(info, sc.W) + "\n" + ui.PadRight(help, sc.W)
	}

	tick := time.NewTicker(time.Second / time.Duration(fps))
	defer tick.Stop()

	for {
		sc.Draw(render())
		select {
		case key, ok := <-keys:
			if !ok {
				return nil
			}
			switch {
			case key.Is("esc") || key.IsRune('q') || (key.Ctrl && key.IsRune('c')):
				return nil
			case key.IsRune(' '):
				paused = !paused
			case key.IsRune('i'):
				showInfo = !showInfo
			case key.IsRune('+') || key.IsRune('='):
				if fps < 60 {
					fps += 5
					tick = resetTicker(tick, fps)
				}
			case key.IsRune('-') || key.IsRune('_'):
				if fps > 5 {
					fps -= 5
					tick = resetTicker(tick, fps)
				}
			case key.IsRune('s'):
				path := fmt.Sprintf("%s/swaydesk-fondo-%s.png", os.TempDir(), time.Now().Format("150405"))
				cw, ch := eng.Size(sc.W, (sc.H-2)*2)
				if err := SaveCanvas(eng.Frame(cw, ch), path); err == nil {
					fmt.Printf("\033[?25h\n fondo guardado en %s\n", path)
					time.Sleep(700 * time.Millisecond)
				}
			}
		case <-sc.Resize:
			sc.UpdateSize()
		case <-tick.C:
			if paused {
				<-time.After(30 * time.Millisecond)
			}
		}
	}
}

func pauseLabel(paused bool) string {
	if paused {
		return "pausado"
	}
	return "reproduciendo"
}

func resetTicker(old *time.Ticker, fps int) *time.Ticker {
	old.Stop()
	if fps < 1 {
		fps = 1
	}
	return time.NewTicker(time.Second / time.Duration(fps))
}

// ─── Demonio Wayland ───────────────────────────────────────────────────────

// RunDaemon publica el fondo animado como superficie wlr-layer-shell en la capa
// "background". Si no hay compositor compatible devuelve un error explicativo.
//
// El bucle termina al recibir una señal o cuando stop se cierra.
func RunDaemon(eng *Engine, stop <-chan struct{}, scale int) error {
	return RunDaemonOptions(DaemonOptions{Engine: eng, Scale: scale}, stop)
}

// RunDaemonOptions arranca el demonio con fondos por workspace y transiciones.
func RunDaemonOptions(opt DaemonOptions, stop <-chan struct{}) error {
	if !depsAvailable() {
		return fmt.Errorf("no se encontró un compositor Wayland con wlr-layer-shell")
	}
	if opt.Engine == nil {
		return fmt.Errorf("falta el fondo inicial")
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sig)
	return runDaemonOptions(opt, stop, sig)
}

// WaitForSignal permite ejecutar el demonio en primer plano desde un servicio.
func WaitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

// ScaleFor calcula la resolución interna recomendada para la salida.
func ScaleFor(width, height int) int {
	pixels := width * height
	switch {
	case pixels > 3840*2160:
		return 4
	case pixels > 2560*1440:
		return 3
	case pixels > 1920*1080:
		return 3
	case pixels > 1280*720:
		return 2
	default:
		return 2
	}
}

// clamp limita v al rango [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
