// Package boot implementa la animación de arranque de la sesión de swaydesk:
// un logo que se dibuja con el degradado del tema, una lista de componentes
// que se van activando y partículas que caen, todo en la terminal.
package boot

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// Step es un componente del escritorio que aparece durante la animación.
type Step struct {
	Label  string
	Detail string
	OK     bool
}

// Options controla la animación.
type Options struct {
	Theme    *ui.Theme
	Style    string        // sakura, neon, matrix, minimal
	Duration time.Duration // duración total; 0 = automática
	Steps    []Step
	Version  string
	Intro    bool // dibujar el logo grande con partículas
}

// Styles devuelve los estilos de arranque disponibles.
func Styles() []string { return []string{"sakura", "neon", "matrix", "minimal"} }

// ValidStyle indica si el estilo existe.
func ValidStyle(s string) bool {
	for _, x := range Styles() {
		if strings.EqualFold(s, x) {
			return true
		}
	}
	return false
}

// DefaultSteps inspecciona el sistema y devuelve las comprobaciones que se
// muestran en el arranque.
func DefaultSteps(cfg *config.Config) []Step {
	pal := ui.PaletteByName(cfg.Theme)
	has := launcher.HasCommand
	steps := []Step{
		{"Compositor", "sway + Wayland", swayipc.SwaySock() != "" || has("sway")},
		{"Barra de estado", "waybar", has("waybar")},
		{"Notificaciones", "mako", has("mako")},
		{"Portapapeles", "historial listo", has("cliphist") || has("clipman") || has("wl-paste")},
		{"Tema", pal.Name, true},
	}
	kind := cfg.WallpaperKind
	if kind == "" {
		kind = "scene"
	}
	fondo := kind
	switch kind {
	case "scene":
		name := cfg.WallpaperName
		if name == "" {
			name = pal.Scene
		}
		if name != "" {
			fondo = "escena " + name
		}
	case "image", "gif":
		fondo = "imagen animada"
	case "video":
		fondo = "vídeo" + map[bool]string{true: " (ffmpeg)", false: " (sin ffmpeg)"}[has("ffmpeg")]
	}
	steps = append(steps, Step{"Fondo", fondo, true})
	if wallpaper.RunningPID(cfg.Cache) != 0 {
		steps = append(steps, Step{"Ventanas", "atajos Super activos", true})
	} else {
		steps = append(steps, Step{"Ventanas", "atajos Super activos", true})
	}
	return steps
}

// logo es la tipografía de bloques de "swaydesk" (5 filas).
var logo = map[rune][5]string{
	's': {"████", "█   ", "████", "   █", "████"},
	'w': {"█   █", "█   █", "█ █ █", "█ █ █", " █ █ "},
	'a': {" ███ ", "█   █", "█████", "█   █", "█   █"},
	'y': {"█   █", "█   █", " ███ ", "  █  ", " █   "},
	'd': {"████ ", "█   █", "█   █", "█   █", "████ "},
	'e': {"█████", "█    ", "████ ", "█    ", "█████"},
	'k': {"█   █", "█  █ ", "███  ", "█  █ ", "█   █"},
}

const logoWord = "swaydesk"

// logoRows devuelve las cinco filas del logotipo sin color. Todas tienen el
// mismo ancho para que el bloque quede alineado al centrarlo.
func logoRows() [5]string {
	var out [5]string
	width := 0
	for row := 0; row < 5; row++ {
		var b strings.Builder
		for _, r := range logoWord {
			b.WriteString(logo[r][row])
			b.WriteString(" ")
		}
		out[row] = strings.TrimRight(b.String(), " ")
		if n := ui.Width(out[row]); n > width {
			width = n
		}
	}
	for i := range out {
		out[i] = ui.PadRight(out[i], width)
	}
	return out
}

// paintLogo colorea el logotipo con un degradado horizontal.
func paintLogo(th *ui.Theme, t float64) [5]string {
	p := th.Palette
	rows := logoRows()
	g := p.GradientOr()
	out := [5]string{}
	for i, row := range rows {
		var b strings.Builder
		cols := ui.Width(row)
		for x, r := range []rune(row) {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			pos := 0.0
			if cols > 1 {
				pos = float64(x) / float64(cols-1)
			}
			// Un ligero pulso recorre el logotipo de izquierda a derecha.
			pos = math.Mod(pos+t*0.35, 1.0)
			c := ui.LerpColor(g[0], g[1], pos)
			b.WriteString(ui.Style{Fg: c, Bg: p.Bg, Bold: true}.Render(string(r)))
		}
		out[i] = b.String()
	}
	return out
}

// particle es una partícula decorativa de la animación.
type particle struct {
	x, y  float64
	speed float64
	drift float64
	ch    rune
	shade float64
}

func makeParticles(style string, n int, w, h int, seed int64) []particle {
	chars := []rune{'*', '.', '+', '·'}
	switch style {
	case "sakura":
		chars = []rune{'*', '.', '+', '·', '*'}
	case "neon":
		chars = []rune{':', ';', '|', '.', '/'}
	case "matrix":
		chars = []rune{'0', '1', '2', '9', '7', '4', '8', '5'}
	case "minimal":
		return nil
	}
	out := make([]particle, 0, n)
	rnd := seed
	next := func() float64 {
		rnd = (rnd*1103515245 + 12345) & 0x7fffffff
		return float64(rnd) / float64(0x7fffffff)
	}
	for i := 0; i < n; i++ {
		speed := 0.04 + next()*0.12
		if style == "neon" || style == "matrix" {
			speed = 0.12 + next()*0.3
		}
		out = append(out, particle{
			x:     next() * float64(w),
			y:     next() * float64(h),
			speed: speed,
			drift: (next() - 0.5) * 0.08,
			ch:    chars[int(next()*float64(len(chars)))%len(chars)],
			shade: next(),
		})
	}
	return out
}

// Run reproduce la animación en la pantalla alternativa y vuelve a dejar el
// terminal como estaba. any key la salta.
func Run(opts Options) error {
	th := opts.Theme
	if th == nil {
		th = ui.NewTheme("catppuccin", false)
	}
	if opts.Style == "" {
		opts.Style = "sakura"
	}
	if !ValidStyle(opts.Style) {
		return fmt.Errorf("estilo de arranque desconocido %q (usa %s)", opts.Style, strings.Join(Styles(), ", "))
	}
	if opts.Duration <= 0 {
		opts.Duration = 2600 * time.Millisecond
	}
	if opts.Style == "minimal" {
		opts.Duration = minDur(opts.Duration, 900*time.Millisecond)
	}

	sc, err := ui.OpenScreen()
	if err != nil {
		return err
	}
	defer sc.Close()

	start := time.Now()
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	keys := make(chan ui.Key, 1)
	go func() {
		for {
			k, err := sc.NextKey()
			if err != nil {
				return
			}
			select {
			case keys <- k:
			default:
			}
		}
	}()

	frame := func(elapsed time.Duration) {
		w, h := sc.W, sc.H
		if w < 20 || h < 10 {
			return
		}
		sc.Draw(render(th, opts, elapsed, w, h))
	}

	for {
		elapsed := time.Since(start)
		frame(elapsed)
		if elapsed >= opts.Duration {
			break
		}
		select {
		case <-ticker.C:
			select {
			case <-sc.Resize:
				sc.UpdateSize()
			default:
			}
		case <-keys:
			// Cualquier tecla acelera el arranque: se dibuja el último
			// fotograma y se sale.
			frame(opts.Duration)
			return nil
		}
	}
	return nil
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// render compone un fotograma completo.
func render(th *ui.Theme, opts Options, elapsed time.Duration, w, h int) string {
	p := th.Palette
	prog := float64(elapsed) / float64(opts.Duration)
	if prog > 1 {
		prog = 1
	}
	// Aparición y desvanecido suaves: el primer 12 % y el último 8 %.
	alpha := 1.0
	switch {
	case prog < 0.12:
		alpha = 0.25 + 0.75*(prog/0.12)
	case prog > 0.92:
		alpha = math.Max(0.35, 1-(prog-0.92)/0.08)
	}
	fade := func(c ui.Color) ui.Color {
		if alpha >= 0.999 {
			return c
		}
		return ui.LerpColor(p.Bg, c, alpha)
	}

	// Contenido central: logotipo, subtítulo, pasos y barra de progreso.
	block := make([]string, 0, 32)
	if opts.Intro {
		block = append(block, "")
		for _, row := range paintLogo(th, prog) {
			block = append(block, row)
		}
	}
	block = append(block, "",
		ui.Style{Fg: fade(p.FgDim), Bg: p.Bg}.Render("escritorio para Arch Linux + sway · escrito en Go"),
		"")

	stepsStart, stepsEnd := 0.15, 0.88
	for i, st := range opts.Steps {
		at := stepsStart
		if len(opts.Steps) > 0 {
			at = stepsStart + (stepsEnd-stepsStart)*float64(i+1)/float64(len(opts.Steps))
		}
		done := prog >= at
		mark := ui.Style{Fg: fade(p.Ok), Bg: p.Bg}.Render("✔")
		label := ui.Style{Fg: fade(p.Fg), Bg: p.Bg}.Render(ui.PadRight(st.Label, 17))
		if !done {
			// Spinner ASCII: se dibuja igual en cualquier fuente.
			spin := []string{"-", "\\", "|", "/"}
			f := spin[int(elapsed/(70*time.Millisecond))%len(spin)]
			mark = ui.Style{Fg: fade(p.Accent), Bg: p.Bg}.Render(f)
			label = ui.Style{Fg: fade(p.Muted), Bg: p.Bg}.Render(ui.PadRight(st.Label, 17))
		}
		detail := ui.Style{Fg: fade(p.FgDim), Bg: p.Bg}.Render(st.Detail)
		block = append(block, "  "+mark+"  "+label+" "+detail)
	}

	block = append(block, "", progressBar(th, 44, prog, fade), "")

	hint := "pulsa cualquier tecla para saltar"
	if prog < 0.9 {
		hint = fmt.Sprintf("%d %%  ·  %s", int(math.Round(prog*100)), hint)
	}
	block = append(block, ui.Style{Fg: fade(p.Muted), Bg: p.Bg}.Render(hint))
	if opts.Version != "" {
		block = append(block, ui.Style{Fg: fade(p.Border), Bg: p.Bg}.Render("swaydesk "+opts.Version))
	}

	// Todas las líneas del bloque se igualan al mismo ancho para que el centrado
	// sea uniforme (si no, cada fila se desplazaría según su longitud).
	blockW := 0
	for _, l := range block {
		if n := ui.Width(l); n > blockW {
			blockW = n
		}
	}
	for i, l := range block {
		if pad := blockW - ui.Width(l); pad > 0 {
			block[i] = l + ui.Style{Bg: p.Bg}.Render(strings.Repeat(" ", pad))
		}
	}

	// Partículas decorativas: solo se dibujan fuera del bloque de texto para no
	// romper la alineación de las líneas con color.
	var parts []particle
	if opts.Intro {
		parts = makeParticles(opts.Style, 110, w, h, int64(11+len(opts.Style)))
	}
	top := (h - len(block)) / 2
	if top < 0 {
		top = 0
	}
	bgSpace := func(n int) string {
		if n <= 0 {
			return ""
		}
		return ui.Style{Bg: p.Bg}.Render(strings.Repeat(" ", n))
	}

	// Prepara la rejilla de partículas por fila.
	deco := make([]map[int]ui.Color, h)
	for i := range deco {
		deco[i] = map[int]ui.Color{}
	}
	for _, pt := range parts {
		y := pt.y + elapsed.Seconds()*pt.speed*14
		x := pt.x + math.Sin(elapsed.Seconds()*0.7+pt.x*0.3)*pt.drift*10
		row := int(math.Mod(y, float64(h)))
		col := int(math.Mod(x, float64(w)))
		if row < 0 || row >= h || col < 0 || col >= w {
			continue
		}
		// No se pisa el bloque central.
		if row >= top-1 && row < top+len(block)+1 {
			continue
		}
		deco[row][col] = fade(ui.LerpColor(p.Accent2, p.Muted, pt.shade*0.75))
		_ = pt.ch
	}

	// Mapa de caracteres por fila/columna.
	chars := make([]map[int]rune, h)
	for i := range chars {
		chars[i] = map[int]rune{}
	}
	rnd := int64(11 + len(opts.Style))
	for _, pt := range parts {
		y := pt.y + elapsed.Seconds()*pt.speed*14
		x := pt.x + math.Sin(elapsed.Seconds()*0.7+pt.x*0.3)*pt.drift*10
		row := int(math.Mod(y, float64(h)))
		col := int(math.Mod(x, float64(w)))
		if row < 0 || row >= h || col < 0 || col >= w {
			continue
		}
		if row >= top-1 && row < top+len(block)+1 {
			continue
		}
		chars[row][col] = pt.ch
	}
	_ = rnd

	lines := make([]string, 0, h)
	for row := 0; row < h; row++ {
		// Línea del bloque, centrada y con el fondo del tema a los lados.
		if row >= top && row < top+len(block) {
			line := block[row-top]
			pad := (w - blockW) / 2
			if pad < 0 {
				pad = 0
			}
			tail := w - pad - ui.Width(line)
			if tail < 0 {
				tail = 0
			}
			lines = append(lines, bgSpace(pad)+line+bgSpace(tail))
			continue
		}
		// Fila decorativa: espacios con partículas.
		var b strings.Builder
		run := 0
		flush := func() {
			if run > 0 {
				b.WriteString(bgSpace(run))
				run = 0
			}
		}
		for col := 0; col < w; col++ {
			if ch, ok := chars[row][col]; ok {
				flush()
				b.WriteString(ui.Style{Fg: deco[row][col], Bg: p.Bg}.Render(string(ch)))
				continue
			}
			run++
		}
		flush()
		lines = append(lines, b.String())
	}
	for len(lines) < h {
		lines = append(lines, bgSpace(w))
	}
	return strings.Join(lines[:h], "\n")
}

// progressBar dibuja una barra con degradado y porcentaje.
func progressBar(th *ui.Theme, barW int, prog float64, fade func(ui.Color) ui.Color) string {
	p := th.Palette
	g := p.GradientOr()
	filled := int(math.Round(prog * float64(barW)))
	var b strings.Builder
	b.WriteString("  ")
	for i := 0; i < barW; i++ {
		pos := float64(i) / float64(max(1, barW-1))
		c := ui.LerpColor(g[0], g[1], pos)
		if i < filled {
			b.WriteString(ui.Style{Fg: fade(c), Bg: fade(c)}.Render("▬"))
		} else {
			b.WriteString(ui.Style{Fg: fade(p.Border), Bg: p.Bg}.Render("▬"))
		}
	}
	b.WriteString(ui.Style{Fg: fade(p.FgDim), Bg: p.Bg}.Render(fmt.Sprintf(" %3d %%", int(math.Round(prog*100)))))
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
