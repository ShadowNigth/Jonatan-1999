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
	"github.com/ShadowNigth/swaydesk/internal/ui"
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

// DaemonOptions ajusta el demonio de fondo animado.
type DaemonOptions struct {
	// Engine es el fondo con el que arranca (el de la configuración).
	Engine *Engine
	// Workspaces resuelve el fondo de cada workspace (puede ser nil).
	Workspaces *WorkspaceMap
	// Transition anima el cambio de workspace.
	Transition Transition
	// Palette es la paleta con la que se interpretan los fondos nuevos.
	Palette ui.Palette
	// FPS es el valor de reserva para los fondos sin FPS propio.
	FPS int
	// Scale reduce la resolución interna (0 = automática).
	Scale int
}

// daemonState guarda el fondo activo y la transición en curso.
type daemonState struct {
	opt   DaemonOptions
	bg    *wayland.Background
	scale int

	eng  *Engine
	from *Canvas // último fotograma del fondo anterior, congelado
	last *Canvas // último fotograma dibujado

	source string
	start  time.Time
	moving bool
}

// runDaemonOptions implementa el bucle del fondo animado sobre wlr-layer-shell.
func runDaemonOptions(opt DaemonOptions, stop <-chan struct{}, sig <-chan os.Signal) error {
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

	if opt.Scale < 1 {
		opt.Scale = ScaleFor(w, h)
	}
	if err := bg.InitShm(2); err != nil {
		return err
	}
	writePID()
	defer removePID()

	d := &daemonState{opt: opt, bg: bg, scale: opt.Scale, eng: opt.Engine}
	defer d.closeEngines()
	d.source = d.eng.Spec().Source()

	// Si el workspace enfocado tiene su propio fondo, se aplica ya al arrancar.
	if opt.Workspaces != nil {
		if num, name, ok := FocusedWorkspace(); ok {
			if source, _ := opt.Workspaces.SourceFor(num, name); source != "" && source != d.source {
				if err := d.install(source, false); err != nil {
					fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
				}
			}
		}
	}

	// Los eventos de sway llegan por un canal aparte para no bloquear el dibujo.
	events := workspaceStream(stop)

	for {
		wait := d.eng.Interval()
		drawFrame := d.last == nil || d.moving || (d.eng.Kind() != KindImage && d.eng.Kind() != KindNone)
		if d.moving {
			wait = 16 * time.Millisecond
		} else if !drawFrame {
			// Un fondo fijo solo se vuelve a dibujar al quedar sucio (inicio,
			// cambio de workspace o resize); el loop sigue atendiendo Wayland.
			wait = 250 * time.Millisecond
		}
		select {
		case <-sig:
			return nil
		case <-stop:
			return nil
		case ev, ok := <-events:
			if !ok {
				// Sin canal de eventos: se sigue dibujando el fondo actual.
				events = nil
				continue
			}
			d.onWorkspace(ev)
		case <-time.After(wait):
			if bg.Closed() {
				return nil
			}
			if bg.NeedsResize() {
				if err := bg.ReinitShm(2); err != nil {
					return err
				}
				// Una transición a medias con otro tamaño se descarta.
				d.moving = false
				d.from = nil
				d.last = nil
				drawFrame = true
			}
			if drawFrame {
				if err := d.frame(); err != nil {
					return err
				}
			}
			// Se procesan los eventos pendientes (release de búferes, configure).
			_ = bg.Dispatch(0)
		}
	}
}

func (d *daemonState) closeEngines() {
	if d.eng != nil {
		d.eng.Close()
		d.eng = nil
	}
}

// dims calcula el tamaño del lienzo para un tipo de fondo. Las escenas usan
// media altura: se dibujan a resolución de terminal (2 píxeles por fila).
func (d *daemonState) dims(kind Kind) (int, int) {
	sw, sh := d.bg.Size()
	w, h := sw/d.scale, sh/d.scale
	if w < 16 {
		w = 16
	}
	if h < 16 {
		h = 16
	}
	if kind == KindScene {
		h /= 2
		if h < 16 {
			h = 16
		}
	}
	return w, h
}

// expandHomePath expande el prefijo de ruta ~/ sin alterar las rutas relativas
// ni las referencias de escena.
func expandHomePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path, err
	}
	return filepath.Join(home, path[2:]), nil
}

// install cambia el fondo activo, con transición si animate lo permite.
func (d *daemonState) install(source string, animate bool) error {
	spec, err := ParseSpec(source, d.opt.Palette)
	if err != nil {
		return err
	}
	if spec.Path != "" {
		spec.Path, err = expandHomePath(spec.Path)
		if err != nil {
			return err
		}
	}
	if spec.FPS == 0 {
		spec.FPS = d.opt.FPS
	}
	next, err := NewEngine(spec)
	if err != nil {
		return err
	}
	if !animate || !d.opt.Transition.Active() || d.last == nil || d.eng == nil {
		if d.eng != nil {
			d.eng.Close()
		}
		d.eng = next
		d.from = nil
		d.last = nil // el siguiente ciclo debe dibujar el fondo nuevo
		d.moving = false
		d.source = source
		return nil
	}
	// El motor anterior ya no se dibuja: su último fotograma queda congelado
	// en d.from y el vídeo puede cerrar su tubería de ffmpeg.
	if d.eng != nil {
		d.eng.Close()
	}
	d.eng = next
	d.from = d.last
	d.start = time.Now()
	d.moving = true
	d.source = source
	return nil
}

// frame dibuja el siguiente fotograma (con transición si hace falta).
func (d *daemonState) frame() error {
	w, h := d.dims(d.eng.Kind())
	c := d.eng.Frame(w, h)
	if d.moving {
		p := d.opt.Transition.Progress(time.Since(d.start))
		out := Blend(d.from, c, EaseInOut(p), d.opt.Transition.Kind)
		d.last = out
		if err := d.bg.Draw(out.Pix, out.W, out.H); err != nil {
			return err
		}
		if p >= 1 {
			d.moving = false
			d.from = nil
		}
		return nil
	}
	d.last = c
	return d.bg.Draw(c.Pix, c.W, c.H)
}

// onWorkspace reacciona a un cambio de workspace de sway.
func (d *daemonState) onWorkspace(ev swayipc.WorkspaceEvent) {
	if d.opt.Workspaces == nil {
		return
	}
	switch ev.Change {
	case "", "focus", "init":
	default:
		return
	}
	if ev.Current.Name == "" {
		return
	}
	source, _ := d.opt.Workspaces.SourceFor(ev.Current.Num, ev.Current.Name)
	if source == "" || source == d.source {
		return
	}
	prev := d.source
	d.source = source
	if err := d.install(source, true); err != nil {
		d.source = prev
		fmt.Fprintf(os.Stderr, "swaydesk: fondo de %q: %v\n", ev.Current.Name, err)
	}
}

// workspaceStream escucha los cambios de workspace de sway y los entrega por
// un canal. Si sway no está, reintenta cada dos segundos sin bloquear el fondo.
func workspaceStream(stop <-chan struct{}) <-chan swayipc.WorkspaceEvent {
	out := make(chan swayipc.WorkspaceEvent, 8)
	go func() {
		defer close(out)
		for {
			select {
			case <-stop:
				return
			default:
			}
			c, err := swayipc.Dial()
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			if err := c.Subscribe("workspace"); err != nil {
				c.Close()
				time.Sleep(2 * time.Second)
				continue
			}
			// Una conexión tardía debe aplicar el workspace que ya está enfocado,
			// no esperar al siguiente cambio del usuario.
			if num, name, ok := FocusedWorkspace(); ok {
				select {
				case out <- swayipc.WorkspaceEvent{Change: "focus", Current: swayipc.Workspace{Num: num, Name: name, Focused: true}}:
				case <-stop:
					c.Close()
					return
				}
			}
			for ev := range c.Events(8) {
				if ev.Type != swayipc.EventWorkspace {
					continue
				}
				var we swayipc.WorkspaceEvent
				if err := ev.Decode(&we); err != nil {
					continue
				}
				select {
				case out <- we:
				case <-stop:
					c.Close()
					return
				}
			}
			c.Close()
			time.Sleep(time.Second)
		}
	}()
	return out
}

// FocusedWorkspace devuelve el número y el nombre del workspace enfocado.
func FocusedWorkspace() (int, string, bool) {
	c, err := swayipc.Dial()
	if err != nil {
		return 0, "", false
	}
	defer c.Close()
	ws, err := c.Workspaces()
	if err != nil {
		return 0, "", false
	}
	for _, x := range ws {
		if x.Focused {
			return x.Num, x.Name, true
		}
	}
	return 0, "", false
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
