package bar

import (
	"fmt"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Workspace es una entrada de la barra de workspaces.
type Workspace struct {
	Name    string
	Focused bool
	Visible bool
	Urgent  bool
	Windows int
}

// Snapshot es una foto de todo lo que muestra la barra en un instante.
type Snapshot struct {
	Workspaces []Workspace
	Title      string
	AppID      string

	Clock string
	Date  string

	CPU     float64
	RAM     float64
	Disk    float64
	Temp    float64
	Uptime  string
	Volumen int
	Muted   bool

	Wifi      string
	Signal    int
	Connected bool

	HasBattery bool
	Battery    int
	Charging   bool

	Recording bool
	Media     string
	// Notifications es cuántas hay en pantalla; NotifDND, el modo no molestar.
	Notifications int
	NotifDND      bool
	// NotifReplies, cuántas de ellas admiten respuesta rápida.
	NotifReplies int
	Theme        string
	Updates      int
}

// Options configura el aspecto de la barra.
type Options struct {
	Height    int      // alto total en píxeles (incluye márgenes)
	Margin    int      // separación con los bordes de la pantalla
	Padding   int      // relleno interno de cada isla
	Gap       int      // separación entre islas
	Radius    int      // radio de las esquinas (0 = pill)
	Opacity   float64  // opacidad del fondo de las islas
	Islands   bool     // islas separadas o una sola barra
	Modules   []string // módulos de la derecha, en orden
	ShowLogo  bool
	FontPath  string
	FontScale int // 0 = automático
	Spacing   int // separación entre letras (píxeles de la fuente)
	Theme     ui.Palette
	Face      *font.Face
	Shadow    bool // sombra suave bajo las islas
}

// DefaultModules es la lista de módulos de la derecha por defecto.
var DefaultModules = []string{"media", "recording", "notify", "cpu", "ram", "volume", "network", "battery", "clock", "power"}

func hasPowerModule(modules []string) bool {
	for _, module := range modules {
		if module == "power" {
			return true
		}
	}
	return false
}

// item es un fragmento de texto con su color dentro de una isla.
type item struct {
	text  string
	fg    RGBA
	kind  int // 0 texto, 1 pastilla, 2 separador
	extra string
	// click, si no está vacío, marca el elemento como zona pulsable con ese
	// nombre (por ejemplo "notify").
	click string
}

// Region es una zona pulsable de la barra, en píxeles del lienzo.
type Region struct {
	Name string
	X, Y int
	W, H int
}

// Contains indica si un punto cae dentro de la región.
func (r Region) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

const (
	itemText = iota
	itemPill
	itemGap
)

// Renderer dibuja la barra.
type Renderer struct {
	Opt   Options
	Theme ui.Palette
	face  *font.Face
	scale int
	// regions acumula las zonas pulsables del último Render, en píxeles.
	regions []Region
}

// Regions devuelve las zonas pulsables del último Render.
func (r *Renderer) Regions() []Region { return r.regions }

// RegionAt devuelve la zona pulsable que contiene el punto, si hay alguna.
func (r *Renderer) RegionAt(x, y int) (Region, bool) {
	for _, reg := range r.regions {
		if reg.Contains(x, y) {
			return reg, true
		}
	}
	return Region{}, false
}

// New crea un renderizador con la fuente del sistema si está disponible.
func New(opt Options) *Renderer {
	if opt.Face == nil {
		opt.Face = font.Best(opt.FontPath)
	}
	if opt.Height <= 0 {
		opt.Height = 34
	}
	if opt.Margin < 0 {
		opt.Margin = 0
	}
	if opt.Padding <= 0 {
		opt.Padding = 12
	}
	if opt.Gap <= 0 {
		opt.Gap = 8
	}
	if opt.Opacity <= 0 || opt.Opacity > 1 {
		opt.Opacity = 0.88
	}
	if opt.Spacing < 0 {
		opt.Spacing = 0
	}
	if len(opt.Modules) == 0 {
		opt.Modules = append([]string(nil), DefaultModules...)
	} else if !hasPowerModule(opt.Modules) {
		// El botón de apagado es un control permanente, igual que en Waybar.
		// Lo añadimos también a configuraciones antiguas que no lo conocían.
		opt.Modules = append(append([]string(nil), opt.Modules...), "power")
	}
	r := &Renderer{Opt: opt, Theme: opt.Theme, face: opt.Face}
	if r.Theme.Bg == "" {
		r.Theme = ui.PaletteByName("catppuccin")
	}
	r.scale = opt.FontScale
	if r.scale <= 0 {
		// Se reserva un pequeño respiro vertical dentro de la isla.
		inner := opt.Height - 2*opt.Margin - 8
		r.scale = inner / r.face.H
		if r.scale < 1 {
			r.scale = 1
		}
	}
	return r
}

// Face devuelve la fuente que se está usando (para diagnóstico).
func (r *Renderer) Face() *font.Face { return r.face }

// Scale devuelve la escala de la fuente.
func (r *Renderer) Scale() int { return r.scale }

// textHeight es el alto del texto escalado.
func (r *Renderer) textHeight() int { return r.face.Rows(r.scale) }

// measure mide un texto.
func (r *Renderer) measure(s string) int {
	return TextWidth(r.face, s, r.scale, r.Opt.Spacing)
}

// colors devuelve los colores base a partir de la paleta del tema.
func (r *Renderer) colors() (bg, border, fg, muted, accent, accent2, ok, warn, err RGBA) {
	p := r.Theme
	bg = ParseColor(string(p.Bg), r.Opt.Opacity)
	border = ParseColor(string(p.Border), minF(1, r.Opt.Opacity+0.15))
	fg = ParseColor(string(p.Fg), 1)
	muted = ParseColor(string(p.FgDim), 1)
	accent = ParseColor(string(p.Accent), 1)
	accent2 = ParseColor(string(p.Accent2), 1)
	ok = ParseColor(string(p.Ok), 1)
	warn = ParseColor(string(p.Warn), 1)
	err = ParseColor(string(p.Err), 1)
	return
}

// Render dibuja la barra completa con el ancho indicado.
func (r *Renderer) Render(s Snapshot, width int) *Canvas {
	if width < 80 {
		width = 80
	}
	_, _, fg, muted, accent, accent2, ok, warn, errc := r.colors()
	r.regions = r.regions[:0]
	c := NewCanvas(width, r.Opt.Height)

	left := r.leftItems(s, fg, muted, accent, accent2)
	right := r.rightItems(s, fg, muted, accent, ok, warn, errc)

	if !r.Opt.Islands {
		// Una sola barra de lado a lado.
		inner := append([]item{}, left...)
		inner = append(inner, item{kind: itemGap})
		rightAll := r.joinWithSeparator(right)
		x := r.Opt.Margin
		w := width - 2*r.Opt.Margin
		r.drawIsland(c, x, w, inner, false, fg, muted, accent)
		// Extremo derecho: se calcula desde el borde.
		total := r.itemsWidth(rightAll)
		r.drawItems(c, width-r.Opt.Margin-r.Opt.Padding-total, rightAll, fg, muted, accent)
		return c
	}

	// Islas: izquierda, centro y derecha. Si con todos los módulos el centro
	// no cabe, se sacrifica primero el reproductor y luego el título.
	leftW := r.itemsWidth(left) + 2*r.Opt.Padding
	x := r.Opt.Margin

	rightVariants := [][]item{right}
	if len(right) > 0 && r.Opt.Modules[0] == "media" {
		rightVariants = append(rightVariants, right[1:])
	}

	var rightAll []item
	var rightW, rx int
	drawRight := func(items []item) (int, int) {
		all := r.joinWithSeparator(items)
		w := r.itemsWidth(all) + 2*r.Opt.Padding
		return w, width - r.Opt.Margin - w
	}
	chosen := false
	for i, items := range rightVariants {
		w, x0 := drawRight(items)
		gap := x0 - r.Opt.Gap - (x + leftW + r.Opt.Gap)
		// Se exige espacio para el título (al menos ~12 caracteres) salvo en
		// la última variante, que siempre se acepta.
		minW := 12 * r.face.W * r.scale
		if gap > 2*r.Opt.Padding+minW || i == len(rightVariants)-1 {
			rightAll, rightW, rx = items, w, x0
			chosen = true
			break
		}
	}
	if !chosen {
		rightAll = right
		rightW, rx = drawRight(right)
	}
	r.drawIsland(c, x, leftW, left, false, fg, muted, accent)
	r.drawIsland(c, rx, rightW, r.joinWithSeparator(rightAll), true, fg, muted, accent)

	// Isla central: se ajusta el título al hueco real que queda entre las otras
	// dos islas y se centra en ese hueco (no en la pantalla, para no solaparse).
	bandStart := x + leftW + r.Opt.Gap
	bandEnd := rx - r.Opt.Gap
	if band := bandEnd - bandStart; band > 120 {
		if cand, ok := r.centerFitted(s, fg, muted, band-2*r.Opt.Padding); ok {
			centerW := r.itemsWidth(cand) + 2*r.Opt.Padding
			if centerW <= band {
				cx := bandStart + (band-centerW)/2
				r.drawIsland(c, cx, centerW, cand, true, fg, muted, accent)
			}
		}
	}
	return c
}

// joinWithSeparator inserta puntos entre módulos consecutivos.
func (r *Renderer) joinWithSeparator(items []item) []item {
	var out []item
	for i, it := range items {
		if i > 0 {
			out = append(out, item{kind: itemGap})
		}
		out = append(out, it)
	}
	return out
}

// itemsWidth suma el ancho de una lista de elementos.
func (r *Renderer) itemsWidth(items []item) int {
	total := 0
	for i, it := range items {
		if i > 0 {
			total += r.itemGap()
		}
		total += r.itemWidth(it)
	}
	return total
}

func (r *Renderer) itemWidth(it item) int {
	switch it.kind {
	case itemGap:
		return r.separatorWidth()
	case itemPill:
		return r.measure(it.text) + 2*r.pillPad()
	default:
		return r.measure(it.text)
	}
}

func (r *Renderer) pillPad() int {
	p := r.Opt.Padding / 2
	if p < 4 {
		p = 4
	}
	return p
}

// itemGap es la separación entre elementos de una isla.
func (r *Renderer) itemGap() int {
	g := r.textHeight() / 2
	if g < 5 {
		g = 5
	}
	return g
}

// separatorWidth es el hueco de un separador.
func (r *Renderer) separatorWidth() int { return r.textHeight() / 2 }

// drawIsland dibuja el fondo redondeado y su contenido.
func (r *Renderer) drawIsland(c *Canvas, x, w int, items []item, withBorder bool, fg, muted, accent RGBA) {
	bg, border, _, _, _, _, _, _, _ := r.colors()
	h := r.Opt.Height - 2*r.Opt.Margin
	if h <= 0 {
		h = r.Opt.Height
	}
	y := r.Opt.Margin
	radius := r.Opt.Radius
	if radius == 0 {
		radius = h / 2
	}
	var borderCol *RGBA
	if withBorder {
		borderCol = &border
	}
	if r.Opt.Shadow {
		shadow := RGBA{0, 0, 0, 40}
		c.FillRound(x, y+2, w, h, radius, shadow, nil)
	}
	c.FillRound(x, y, w, h, radius, bg, borderCol)
	tx := x + r.Opt.Padding
	r.drawItems(c, tx, items, fg, muted, accent)
}

// drawItems dibuja los elementos en una fila a partir de x.
func (r *Renderer) drawItems(c *Canvas, x int, items []item, fg, muted, accent RGBA) {
	y := r.Opt.Margin + ((r.Opt.Height-2*r.Opt.Margin)-r.textHeight())/2
	for i, it := range items {
		if i > 0 {
			x += r.itemGap()
		}
		if it.click != "" {
			h := r.Opt.Height - 2*r.Opt.Margin
			if h <= 0 {
				h = r.Opt.Height
			}
			r.regions = append(r.regions, Region{
				Name: it.click,
				X:    x - 3,
				Y:    r.Opt.Margin,
				W:    r.itemWidth(it) + 6,
				H:    h,
			})
		}
		switch it.kind {
		case itemGap:
			// Punto separador.
			dotW := r.separatorWidth()
			line := r.textHeight() / 2
			c.FillRect(x+dotW/2-1, y+(r.textHeight()-line)/2+line/2, 2, 2, muted)
			x += dotW
			continue
		case itemPill:
			pw := r.measure(it.text) + 2*r.pillPad()
			ph := r.textHeight() + r.textHeight()/3
			py := y - r.textHeight()/6
			radius := ph / 2
			c.FillRound(x, py, pw, ph, radius, it.fg, nil)
			// Texto del color del fondo para que contraste con la pastilla.
			tfg := RGBA{0, 0, 0, 255}
			bg, _, _, _, _, _, _, _, _ := r.colors()
			if luminance(it.fg) > 0.6 {
				tfg = bg
				tfg.A = 255
				tfg = RGBA{30, 30, 40, 255}
			} else {
				tfg = RGBA{250, 250, 252, 255}
			}
			c.Text(r.face, x+r.pillPad(), y, it.text, r.scale, r.Opt.Spacing, tfg)
			x += pw
			continue
		}
		c.Text(r.face, x, y, it.text, r.scale, r.Opt.Spacing, it.fg)
		x += r.measure(it.text)
	}
	_ = fg
	_ = muted
	_ = accent
}

// leftItems construye la isla izquierda: marca + workspaces.
func (r *Renderer) leftItems(s Snapshot, fg, muted, accent, accent2 RGBA) []item {
	var items []item
	if r.Opt.ShowLogo {
		items = append(items, item{text: "swaydesk", fg: accent})
		items = append(items, item{kind: itemGap})
	}
	if len(s.Workspaces) == 0 {
		items = append(items, item{text: "sin workspaces", fg: muted})
		return items
	}
	for _, w := range s.Workspaces {
		label := w.Name
		if label == "" {
			continue
		}
		col := muted
		kind := itemText
		switch {
		case w.Urgent:
			col, kind = accent2, itemPill
		case w.Focused:
			col, kind = accent, itemPill
		case w.Visible:
			col = fg
		}
		items = append(items, item{text: label, fg: col, kind: kind})
	}
	return items
}

// centerFitted construye la isla central con el título recortado al ancho
// disponible (en píxeles).
func (r *Renderer) centerFitted(s Snapshot, fg, muted RGBA, maxW int) ([]item, bool) {
	title := strings.TrimSpace(s.Title)
	if title == "" {
		if s.Date == "" {
			return nil, false
		}
		return []item{{text: s.Date, fg: muted}}, true
	}
	cell := r.face.W * r.scale
	if cell <= 0 {
		return nil, false
	}
	chars := maxW / cell
	if chars < 6 {
		return nil, false
	}
	// Si sobra sitio se añade el app_id como sufijo atenuado.
	app := s.AppID
	if app != "" && chars > len([]rune(title))+len([]rune(app))+4 && len([]rune(title)) <= 46 {
		return []item{
			{text: title, fg: fg},
			{kind: itemGap},
			{text: app, fg: muted},
		}, true
	}
	return []item{{text: shorten(title, chars), fg: fg}}, true
}

// shorten recorta un texto por runas añadiendo puntos suspensivos.
func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 2 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// centerItems construye la isla central: ventana activa o, si no hay, la fecha.
func (r *Renderer) centerItems(s Snapshot, fg, muted, accent RGBA) []item {
	var items []item
	title := strings.TrimSpace(s.Title)
	if title == "" {
		if s.Date == "" {
			return nil
		}
		return []item{{text: s.Date, fg: muted}}
	}
	if len(title) > 46 {
		title = title[:45] + "…"
	}
	items = append(items, item{text: title, fg: fg})
	if s.AppID != "" && s.AppID != title {
		items = append(items, item{kind: itemGap})
		items = append(items, item{text: s.AppID, fg: muted})
	}
	return items
}

// rightItems construye la isla derecha con los módulos configurados.
func (r *Renderer) rightItems(s Snapshot, fg, muted, accent, ok, warn, errc RGBA) []item {
	var items []item
	// add separa los módulos con un punto medio para que la lectura sea clara.
	add := func(text, label string, col RGBA) {
		if text == "" {
			return
		}
		full := text
		if label != "" {
			full = label + " " + text
		}
		items = append(items, item{text: full, fg: col})
	}
	plain := func(text string, col RGBA) { add(text, "", col) }
	chargeColor := func(pct float64) RGBA {
		switch {
		case pct >= 85:
			return errc
		case pct >= 65:
			return warn
		default:
			return ok
		}
	}
	for _, m := range r.Opt.Modules {
		switch m {
		case "media":
			if s.Media != "" {
				plain("♪ "+truncate(s.Media, 20), accent)
			}
		case "recording":
			if s.Recording {
				add("rec", "●", errc)
			}
		case "cpu":
			add(fmt.Sprintf("%2.0f%%", s.CPU), "cpu", chargeColor(s.CPU))
		case "ram":
			add(fmt.Sprintf("%2.0f%%", s.RAM), "ram", chargeColor(s.RAM))
		case "disk":
			add(fmt.Sprintf("%2.0f%%", s.Disk), "ssd", chargeColor(s.Disk))
		case "temp":
			if s.Temp > 0 {
				plain(fmt.Sprintf("%2.0f°c", s.Temp), chargeColor(s.Temp))
			}
		case "uptime":
			add(s.Uptime, "up", muted)
		case "volume":
			if s.Muted {
				add("silencio", "vol", muted)
			} else {
				add(fmt.Sprintf("%d%%", s.Volumen), "vol", fg)
			}
		case "network":
			if s.Connected {
				name := s.Wifi
				if name == "" {
					name = "red"
				}
				add(fmt.Sprintf("%s %d%%", truncate(name, 12), s.Signal), "", fg)
			} else {
				plain("sin red", muted)
			}
		case "battery":
			if s.HasBattery {
				icon := ""
				if s.Charging {
					icon = "+"
				}
				col := ok
				if s.Battery <= 15 {
					col = errc
				} else if s.Battery <= 35 {
					col = warn
				}
				add(fmt.Sprintf("%s%d%%", icon, s.Battery), "", col)
			}
		case "updates":
			if s.Updates > 0 {
				add(fmt.Sprintf("%d", s.Updates), "act.", accent)
			}
		case "power":
			// Control permanente: abre el menú de apagar/reiniciar/suspender.
			items = append(items, item{text: "OFF", fg: errc, kind: itemPill, click: "power"})
		case "notify":
			// Indicador del centro de notificaciones: al pulsarlo se abre.
			// Con avisos en pantalla es una pastilla con el número; el «+»
			// avisa de que alguna tarjeta admite respuesta rápida.
			it := item{text: "avisos", fg: muted, click: "notify"}
			switch {
			case s.NotifDND:
				it.text, it.fg = "dnd", warn
				if s.Notifications > 0 {
					it.text = fmt.Sprintf("dnd %d", s.Notifications)
				}
			case s.Notifications > 0:
				it.text, it.fg, it.kind = fmt.Sprintf("%d", s.Notifications), accent, itemPill
				if s.NotifReplies > 0 {
					it.text += "+"
				}
			}
			items = append(items, it)
		case "title":
			plain(truncate(s.Title, 28), fg)
		case "clock":
			plain(s.Clock, fg)
		case "clock24":
			plain(s.Clock, fg)
		case "date":
			plain(s.Date, muted)
		}
	}
	if len(items) == 0 {
		plain(s.Clock, fg)
	}
	return items
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// luminance estima el brillo de un color (0 = negro, 1 = blanco).
func luminance(c RGBA) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
