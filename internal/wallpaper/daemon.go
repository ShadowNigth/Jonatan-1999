package wallpaper

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/wayland"
)

func depsAvailable() bool { return wayland.Available() }

// OutputSize devuelve el tamaño en píxeles de la salida principal según sway.
func OutputSize() (int, int) {
	w, h := 1920, 1080
	c, err := swayipc.Dial()
	if err != nil {
		return w, h
	}
	defer c.Close()
	outs, err := c.Outputs()
	if err != nil {
		return w, h
	}
	// Se prefiere la salida con el workspace enfocado; si no, la primera activa.
	focusedOutput := ""
	if ws, err := c.Workspaces(); err == nil {
		for _, x := range ws {
			if x.Focused {
				focusedOutput = x.Output
			}
		}
	}
	best := -1
	for i, o := range outs {
		if !o.Active {
			continue
		}
		if o.Name == focusedOutput {
			return o.Rect.Width, o.Rect.Height
		}
		if best < 0 || o.Rect.Width*o.Rect.Height > outs[best].Rect.Width*outs[best].Rect.Height {
			best = i
		}
	}
	if best >= 0 && outs[best].Rect.Width > 0 {
		return outs[best].Rect.Width, outs[best].Rect.Height
	}
	return w, h
}

// runDaemon implementa el bucle del fondo animado sobre wlr-layer-shell.
func runDaemon(eng *Engine, stop <-chan struct{}, sig <-chan os.Signal, scale int) error {
	conn, err := wayland.NewConn()
	if err != nil {
		return err
	}
	if _, err := conn.GetRegistry(); err != nil {
		conn.Close()
		return fmt.Errorf("leyendo los globals del compositor: %w", err)
	}
	w, h := OutputSize()
	bg, err := wayland.NewBackground(conn, w, h, "swaydesk-wallpaper")
	if err != nil {
		conn.Close()
		return err
	}
	defer bg.Close()

	if scale < 1 {
		scale = ScaleFor(w, h)
	}
	sw, sh := bg.Size()
	renderW, renderH := sw/scale, sh/scale
	if renderW < 16 {
		renderW = 16
	}
	if renderH < 16 {
		renderH = 16
	}
	// El lienzo usa dos píxeles por fila cuando la escena es procedural.
	if eng.Kind() == KindScene {
		renderH = renderH / 2
	}

	if err := bg.InitShm(2); err != nil {
		return err
	}
	writePID()

	interval := eng.Interval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Primer fotograma inmediato.
	first := eng.Frame(renderW, renderH)
	if err := bg.Draw(first.Pix, first.W, first.H); err != nil {
		return err
	}

	for {
		select {
		case s := <-sig:
			_ = s
			removePID()
			return nil
		case <-stop:
			removePID()
			return nil
		case <-ticker.C:
			if bg.Closed() {
				removePID()
				return nil
			}
			if bg.NeedsResize() {
				if err := bg.ReinitShm(2); err != nil {
					return err
				}
				sw, sh = bg.Size()
				renderW, renderH = sw/scale, sh/scale
				if renderW < 16 {
					renderW = 16
				}
				if renderH < 16 {
					renderH = 16
				}
				if eng.Kind() == KindScene {
					renderH = renderH / 2
				}
			}
			c := eng.Frame(renderW, renderH)
			if err := bg.Draw(c.Pix, c.W, c.H); err != nil {
				removePID()
				return err
			}
			// Se procesan los eventos pendientes (release de búferes, configure).
			_ = bg.Dispatch(0)
		}
	}
}

// ─── Archivo PID ───────────────────────────────────────────────────────────

// PIDPath devuelve la ruta del archivo con el PID del demonio de fondo.
func PIDPath(cacheDir string) string {
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".cache", "swaydesk")
	}
	return filepath.Join(cacheDir, "wallpaperd.pid")
}

var pidFile string

func writePID() {
	if pidFile == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(pidFile), 0o755)
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

func removePID() {
	if pidFile != "" {
		_ = os.Remove(pidFile)
	}
}

// SetPIDPath configura dónde se escribe el archivo de PID.
func SetPIDPath(path string) { pidFile = path }

// RunningPID devuelve el PID del demonio activo (0 si no hay ninguno).
func RunningPID(cacheDir string) int {
	data, err := os.ReadFile(PIDPath(cacheDir))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0
	}
	// Comprobar que el proceso sigue vivo.
	if err := syscall.Kill(pid, 0); err != nil {
		return 0
	}
	return pid
}

// StopDaemon detiene el demonio activo, si lo hay.
func StopDaemon(cacheDir string) bool {
	pid := RunningPID(cacheDir)
	if pid == 0 {
		return false
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		if err := syscall.Kill(pid, 0); err != nil {
			break
		}
	}
	_ = os.Remove(PIDPath(cacheDir))
	return true
}

// WaylandReady indica si hay un compositor Wayland utilizable.
func WaylandReady() bool { return depsAvailable() }

// DetectDisplayServer describe el compositor actual (Wayland, layer-shell).
func DetectDisplayServer() wayland.DisplayServer { return wayland.Detect() }
