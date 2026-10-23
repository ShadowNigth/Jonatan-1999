package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/host"
	"github.com/ShadowNigth/swaydesk/internal/osd"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// rendererFor monta el dibujante del OSD a partir de la configuración.
func rendererFor(cfg *config.Config, scale, width int, themeName string) *osd.Renderer {
	if themeName == "" {
		themeName = cfg.Theme
	}
	if width <= 0 {
		width = cfg.OSDWidth
	}
	return osd.New(osd.Options{
		Width:    width,
		Padding:  cfg.OSDPadding,
		Radius:   cfg.OSDRadius,
		Opacity:  cfg.OSDOpacity,
		FontPath: cfg.OSDFont,
		Face:     font.Best(cfg.OSDFont),
		Theme:    ui.PaletteByName(themeName),
		Scale:    scale,
		Shadow:   true,
		Segments: cfg.OSDSegments,
	})
}

// currentVolume lee el volumen del altavoz por defecto.
func currentVolume() (osd.State, bool) {
	vol, muted, ok := host.Audio()
	if !ok {
		return osd.State{}, false
	}
	return osd.Volume(vol, muted, host.AudioDevice()), true
}

// runShow dibuja el aviso y termina. Se puede invocar desde un atajo:
//
//	swayosd show            (lee el volumen actual)
//	swayosd show --value 45 --muted
func runShow(args []string) {
	vals, _ := parseFlags(args, nil)
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	if !cfg.OSDEnabled {
		return
	}

	var st osd.State
	if v := last(vals, "value"); v != "" {
		n, err := atoi(v)
		if err != nil {
			fail(fmt.Errorf("valor de volumen inválido: %q", v))
		}
		st = osd.Volume(n, has(vals, "muted"), last(vals, "device"))
	} else {
		cur, ok := currentVolume()
		if !ok {
			fail(fmt.Errorf("no se pudo leer el volumen (¿wpctl, pactl o amixer?)"))
		}
		st = cur
		if has(vals, "muted") {
			st.Muted = true
		}
	}
	if d := last(vals, "device"); d != "" {
		st.Device = d
	}

	scale := scaleFor(cfg, last(vals, "scale"))
	r := rendererFor(cfg, scale, 0, last(vals, "theme"))
	hold := time.Duration(cfg.OSDTimeout) * time.Millisecond
	if v := last(vals, "timeout"); v != "" {
		if n, err := atoi(v); err == nil && n >= 100 {
			hold = time.Duration(n) * time.Millisecond
		}
	}
	if err := showOverlay(cfg, r, st, hold); err != nil {
		fail(err)
	}
}

// runVolume cambia el volumen y muestra el OSD en el mismo gesto, para que los
// atajos del teclado solo necesiten un comando.
//
//	swayosd volume up|down|mute
func runVolume(args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf("uso: swayosd volume <up|down|mute> [--step N]"))
	}
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	step := 5
	if v := last(mustFlags(args[1:]), "step"); v != "" {
		if n, err := atoi(v); err == nil && n > 0 && n <= 30 {
			step = n
		}
	}
	var delta int
	mute := false
	switch strings.ToLower(args[0]) {
	case "up", "+", "subir":
		delta = step
	case "down", "-", "bajar":
		delta = -step
	case "mute", "toggle", "silenciar":
		mute = true
	default:
		fail(fmt.Errorf("orden desconocida: %q (usa up, down o mute)", args[0]))
	}
	if err := changeVolume(delta, mute); err != nil {
		fail(err)
	}
	if !cfg.OSDEnabled {
		return
	}
	// El demonio de audio tarda unos milisegundos en aplicar el cambio; se
	// reintenta la lectura para no mostrar el valor anterior.
	var st osd.State
	var ok bool
	for i := 0; i < 6; i++ {
		if st, ok = currentVolume(); ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ok {
		fail(fmt.Errorf("no se pudo leer el volumen"))
	}
	st.Step = delta
	scale := scaleFor(cfg, last(mustFlags(args[1:]), "scale"))
	r := rendererFor(cfg, scale, 0, last(mustFlags(args[1:]), "theme"))
	if err := showOverlay(cfg, r, st, time.Duration(cfg.OSDTimeout)*time.Millisecond); err != nil {
		fail(err)
	}
}

// runPreview guarda una imagen del OSD, sin sesión gráfica.
func runPreview(args []string) {
	vals, _ := parseFlags(args, nil)
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	out := last(vals, "out")
	if out == "" {
		out = "osd.png"
	}
	value := 45
	if v := last(vals, "value"); v != "" {
		if n, err := atoi(v); err == nil {
			value = n
		}
	}
	st := osd.Volume(value, has(vals, "muted"), orDefault(last(vals, "device"), "Auriculares"))
	r := rendererFor(cfg, 2, 0, last(vals, "theme"))
	canvas := r.Render(st, 1)
	if err := savePNG(canvas, out); err != nil {
		fail(err)
	}
	fmt.Println(out)
}

// fail imprime el error y sale.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "swayosd: %v\n", err)
	os.Exit(1)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
