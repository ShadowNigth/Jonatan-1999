// swaybar es la barra de estado de swaydesk: una superficie de wlr-layer-shell
// dibujada píxel a píxel en Go, con islas redondeadas, translúcidas y
// adaptadas a la paleta activa.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/host"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Version de swaybar (sigue la de swaydesk).
const Version = "1.0.0"

func main() {
	var (
		preview   = flag.Bool("preview", false, "dibuja una vista previa en PNG en lugar de abrir la barra")
		out       = flag.String("out", "", "ruta del PNG de vista previa (por defecto ~/.cache/swaydesk/barra.png)")
		width     = flag.Int("width", 1920, "ancho de la vista previa en píxeles")
		enabled   = flag.Bool("enabled", false, "sale con 0 si la barra está activada en la configuración (para scripts)")
		dump      = flag.Bool("dump", false, "imprime los módulos y la fuente detectada")
		fontPath  = flag.String("font", "", "fuente PSF de consola a usar")
		themeName = flag.String("theme", "", "paleta a usar (por defecto la del tema activo)")
		noHidpi   = flag.Bool("no-hidpi", false, "ignora el factor de escala de la pantalla")
		demo      = flag.Bool("demo", false, "datos de ejemplo en la vista previa (workspaces, batería, música)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `swaybar · barra de estado para sway en Go

Uso: swaybar [opciones]

  --preview          genera un PNG de la barra con los datos actuales
  --out RUTA         archivo del PNG (--preview)
  --width N          ancho de la vista previa (por defecto 1920)
  --enabled          código 0 si la barra está activada en config.toml
  --dump             muestra los módulos y la fuente en uso
  --font RUTA        fuente PSF de consola concreta
  --theme NOMBRE     paleta a usar en la vista previa

Sin opciones arranca la barra en una capa wlr-layer-shell (capa top).
`)
	}
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaybar: %v\n", err)
		os.Exit(1)
	}
	if *enabled {
		if cfg.BarEnabled {
			return
		}
		os.Exit(1)
	}
	themeNameResolved := cfg.Theme
	if *themeName != "" {
		themeNameResolved = *themeName
	}
	pal := ui.PaletteByName(themeNameResolved)

	opts := bar.Options{
		Height:    cfg.BarHeight,
		Margin:    cfg.BarMargin,
		Padding:   cfg.BarPadding,
		Gap:       cfg.BarGap,
		Radius:    cfg.BarRadius,
		Opacity:   cfg.BarBackgroundOpacity,
		Islands:   cfg.BarIslands,
		Modules:   cfg.BarModules,
		ShowLogo:  cfg.BarLogo,
		FontPath:  cfg.BarFont,
		FontScale: cfg.BarFontScale,
		Shadow:    cfg.BarShadow,
		Theme:     pal,
	}
	if *fontPath != "" {
		opts.FontPath = *fontPath
	}
	renderer := bar.New(opts)

	if *dump {
		fmt.Printf("barra: %d px de alto, islas=%v, opacidad=%.2f\n", opts.Height, opts.Islands, opts.Opacity)
		fmt.Printf("fuente: %s (%dx%d, escala x%d)\n", renderer.Face().Name, renderer.Face().W, renderer.Face().H, renderer.Scale())
		fmt.Printf("módulos: %s\n", strings.Join(opts.Modules, ", "))
		return
	}

	if *preview {
		path := *out
		if path == "" {
			path = filepath.Join(cfg.Cache, "barra.png")
		}
		if err := renderPreview(renderer, cfg, *width, path, *demo); err != nil {
			fmt.Fprintf(os.Stderr, "swaybar: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(path)
		return
	}

	scale := 1
	if !*noHidpi {
		scale = outputScale(cfg)
	}
	if err := runBar(cfg, renderer, scale); err != nil {
		fmt.Fprintf(os.Stderr, "swaybar: %v\n", err)
		os.Exit(1)
	}
}

// renderPreview dibuja la barra sobre un fondo de escritorio simulado y la
// guarda como PNG.
func renderPreview(r *bar.Renderer, cfg *config.Config, width int, path string, demo bool) error {
	snap := collect(cfg)
	if demo {
		snap = demoSnapshot(snap)
	}
	canvas := r.Render(snap, width)
	// Se compone sobre un degradado vertical del tema para que la translucidez
	// y las esquinas se aprecien en la captura.
	bg := bar.NewCanvas(width, r.Opt.Height)
	p := ui.PaletteByName(cfg.Theme)
	top := bar.ParseColor(string(p.BgAlt), 1)
	bottom := bar.ParseColor(string(p.Bg), 1)
	for y := 0; y < bg.H; y++ {
		t := 0.0
		if bg.H > 1 {
			t = float64(y) / float64(bg.H-1)
		}
		for x := 0; x < width; x++ {
			bg.Set(x, y, bar.Blend(top, bottom, t))
		}
	}
	for y := 0; y < canvas.H; y++ {
		for x := 0; x < canvas.W; x++ {
			i := (y*canvas.W + x) * 4
			if canvas.Pix[i+3] == 0 {
				continue
			}
			bg.Over(x, y, bar.RGBA{
				R: canvas.Pix[i], G: canvas.Pix[i+1], B: canvas.Pix[i+2], A: canvas.Pix[i+3],
			})
		}
	}
	return savePNG(bg, path)
}

// demoSnapshot rellena los campos que solo existen dentro de una sesión de sway
// para poder ver el diseño completo en una vista previa.
func demoSnapshot(base bar.Snapshot) bar.Snapshot {
	base.Workspaces = []bar.Workspace{
		{Name: "1", Focused: true, Visible: true, Windows: 2},
		{Name: "2", Visible: true, Windows: 1},
		{Name: "3"},
		{Name: "4", Urgent: true},
		{Name: "5", Windows: 3},
	}
	base.Title = "nvim · internal/bar/render.go"
	base.AppID = "foot"
	base.Media = "Lofi Girl · Midnight Study"
	base.HasBattery = true
	base.Battery = 74
	base.Charging = true
	base.Wifi = "Casa"
	base.Signal = 82
	base.Connected = true
	base.Volumen = 45
	base.Notifications = 3
	base.NotifReplies = 1
	if base.CPU < 5 {
		base.CPU = 23
	}
	if base.RAM < 5 {
		base.RAM = 61
	}
	base.Clock = "14:32"
	base.Date = "vie 23 oct"
	return base
}

// collect reúne los datos que muestra la barra.
func collect(cfg *config.Config) bar.Snapshot {
	return snapshot(cfg, host.NewCollector())
}

// outputScale consulta el factor de escala de la salida para HiDPI.
func outputScale(cfg *config.Config) int {
	if v := os.Getenv("SWAYDESK_BAR_SCALE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 3 {
			return n
		}
	}
	return 1
}

func savePNG(c *bar.Canvas, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	img := c.Image()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return encodePNG(f, img)
}

var _ = time.Now
var _ = font.Builtin
