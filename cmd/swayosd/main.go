// swayosd muestra avisos en pantalla (OSD) sobre las ventanas: el nivel de
// volumen aparece en una tarjeta flotante unos instantes y se va solo.
//
// Se puede usar de tres maneras:
//
//	swayosd volume up        cambia el volumen y muestra el aviso
//	swayosd show             muestra el volumen actual (para scripts y atajos)
//	swayosd watch            vigila el volumen y avisa de cualquier cambio
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/actions"
	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
)

// Version de swayosd (sigue la de swaydesk).
const Version = "1.0.0"

func main() {
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		switch {
		case strings.HasPrefix(args[0], "-"):
			// --help y --version también valen sin la orden delante.
			switch strings.TrimLeft(args[0], "-") {
			case "help", "h", "version", "v":
				cmd = strings.TrimLeft(args[0], "-")
				args = args[1:]
			}
		default:
			cmd = args[0]
			args = args[1:]
		}
	}

	switch cmd {
	case "show", "mostrar", "volume-osd":
		runShow(args)
	case "volume", "volumen":
		runVolume(args)
	case "watch", "vigilar":
		runWatch(args)
	case "preview":
		runPreview(args)
	case "version", "v":
		fmt.Printf("swayosd %s (swaydesk)\n", Version)
	case "help", "h", "":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "swayosd: orden desconocida: %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`swayosd · avisos en pantalla para sway, en Go

Uso: swayosd <orden> [opciones]

  volume up|down|mute [--step N]
                             cambia el volumen y muestra el aviso
  show [--value N] [--muted] [--device NOMBRE] [--timeout MS]
                             muestra el volumen actual (o el indicado)
  watch [--intervalo MS]     vigila el volumen y avisa de cada cambio
  preview --out RUTA         PNG del aviso, sin sesión gráfica
  version                    versión

Opciones comunes
  --theme NOMBRE             paleta (por defecto la del tema activo)
  --scale N                  escala de la fuente (1-3)
`)
}

// ─── lectura del estado ────────────────────────────────────────────────────

// scaleFor decide la escala de la fuente. El OSD se dibuja por defecto al
// doble que la barra: se mira de reojo y tiene que leerse de un vistazo.
func scaleFor(cfg *config.Config, valor string) int {
	if valor != "" {
		if n, err := atoi(valor); err == nil && n >= 1 && n <= 3 {
			return n
		}
	}
	if cfg.OSDFontScale > 0 && cfg.OSDFontScale <= 3 {
		return cfg.OSDFontScale
	}
	if env := os.Getenv("SWAYDESK_OSD_SCALE"); env != "" {
		if n, err := atoi(env); err == nil && n >= 1 && n <= 3 {
			return n
		}
	}
	return 2
}

// changeVolume aplica el cambio con la herramienta disponible.
func changeVolume(delta int, mute bool) error {
	if !mute && delta == 0 {
		return nil
	}
	return actions.Volume(delta, mute)
}

// watch vigila el volumen y lanza un OSD por cada cambio, venga de donde venga
// (teclas multimedia, auriculares Bluetooth, otro programa…).
func runWatch(args []string) {
	vals, _ := parseFlags(args, nil)
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	intervalo := 300
	if v := last(vals, "intervalo"); v != "" {
		if n, err := atoi(v); err == nil && n >= 100 {
			intervalo = n
		}
	}
	scale := scaleFor(cfg, last(vals, "scale"))
	themeName := last(vals, "theme")
	// El OSD de un vistazo necesita un proceso por aviso para poder
	// desaparecer solo; este demonio solo vigila y lo lanza.
	exe, err := os.Executable()
	if err != nil {
		exe = "swayosd"
	}
	ultimo := ""
	primera := true
	for {
		st, ok := currentVolume()
		if ok {
			key := fmt.Sprintf("%d|%v|%s", st.Value, st.Muted, st.Device)
			if key != ultimo {
				ultimo = key
				// Arrancar no debe dibujar nada: solo los cambios.
				if !primera && cfg.OSDEnabled {
					args := []string{"show", "--value", strconv.Itoa(st.Value)}
					if st.Muted {
						args = append(args, "--muted")
					}
					if st.Device != "" {
						args = append(args, "--device", st.Device)
					}
					args = append(args, "--scale", strconv.Itoa(scale))
					if themeName != "" {
						args = append(args, "--theme", themeName)
					}
					if err := launcher.StartDetached(exe, args...); err != nil {
						fmt.Fprintf(os.Stderr, "swayosd: %v\n", err)
					}
				}
				primera = false
			}
		}
		time.Sleep(time.Duration(intervalo) * time.Millisecond)
	}
}

// savePNG escribe un lienzo como PNG.
func savePNG(c *bar.Canvas, path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, c.Image())
}

// ─── opciones de línea de órdenes ──────────────────────────────────────────

// parseFlags interpreta --clave valor y --clave=valor; lo demás es posicional.
func parseFlags(args []string, repeat map[string]bool) (map[string][]string, []string) {
	vals := map[string][]string{}
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		value := "true"
		if j := strings.IndexByte(name, '='); j >= 0 {
			name, value = name[:j], name[j+1:]
		} else if i+1 < len(args) && takesValue(name) {
			value = args[i+1]
			i++
		}
		if repeat[name] {
			vals[name] = append(vals[name], value)
		} else {
			vals[name] = []string{value}
		}
	}
	return vals, pos
}

// takesValue enumera las opciones que consumen el argumento siguiente.
func takesValue(name string) bool {
	switch name {
	case "value", "device", "timeout", "scale", "theme", "step", "out", "intervalo":
		return true
	}
	return false
}

// last devuelve el último valor de una opción.
func last(vals map[string][]string, name string) string {
	if v, ok := vals[name]; ok && len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

// has indica si la opción apareció.
func has(vals map[string][]string, name string) bool {
	_, ok := vals[name]
	return ok
}

// mustFlags es un atajo para leer opciones cuando ya se validó la orden.
func mustFlags(args []string) map[string][]string {
	vals, _ := parseFlags(args, nil)
	return vals
}

// atoi convierte sin arrastrar el paquete strconv por todas partes.
func atoi(s string) (int, error) { return strconv.Atoi(strings.TrimSpace(s)) }
