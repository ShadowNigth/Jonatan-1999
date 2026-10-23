package gallery

import (
	"fmt"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// ThemeResult es el resultado de la galería de temas.
type ThemeResult struct {
	Key     string
	Applied bool
}

// ThemeGallery abre la galería interactiva de temas y devuelve la paleta
// elegida (Applied=false si el usuario canceló).
func ThemeGallery(current string, nerdFont bool) (ThemeResult, error) {
	pal := ui.PaletteByName(current)
	theme := ui.NewTheme(current, nerdFont)
	// El marco de la galería usa la paleta de cada tema en la previsualización,
	// pero la interfaz base mantiene la paleta activa.
	sc, err := ui.OpenScreen()
	if err != nil {
		return ThemeResult{}, err
	}
	defer sc.Close()

	cats := append([]string{"todos"}, ui.PaletteCategories()...)
	catIdx := 0
	pane := &listPane{}
	var (
		filter  string
		live    = true
		engine  *wallpaper.Engine
		result  ThemeResult
		applied bool
	)

	rebuild := func() {
		cat := cats[catIdx]
		var metas []ui.PaletteMeta
		if cat == "todos" {
			metas = ui.PaletteList("")
		} else {
			metas = ui.PaletteList(cat)
		}
		pane.entries = pane.entries[:0]
		for _, m := range metas {
			if filter != "" {
				if _, ok := ui.Fuzzy(filter, m.Name+" "+m.Key+" "+m.Category); !ok {
					continue
				}
			}
			p := ui.PaletteByName(m.Key)
			icon := theme.Icons.Theme
			if m.Light {
				icon = "☀"
			}
			pane.entries = append(pane.entries, Entry{
				Key: m.Key, Title: p.Name, Tag: m.Category,
				Sub: string(p.Accent2), Icon: icon, Pal: p,
			})
		}
		if pane.cursor >= len(pane.entries) {
			pane.cursor = max(0, len(pane.entries)-1)
		}
		// Reconstruye la previsualización animada con la escena del tema.
		if engine != nil {
			engine.Close()
			engine = nil
		}
		if e, ok := pane.current(); ok {
			scene := e.Pal.Scene
			if scene == "" {
				scene = "aurora"
			}
			eng, err := wallpaper.NewEngine(wallpaper.Spec{
				Kind: wallpaper.KindScene, Scene: scene, FPS: 22, Pal: e.Pal,
			})
			if err == nil {
				engine = eng
			}
		}
	}
	rebuild()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	view := func(w, h int) string {
		entry, ok := pane.current()
		preview := entry.Pal
		if !ok {
			preview = pal
		}
		pt := ui.NewTheme("", nerdFont)
		pt.Palette = preview
		pt.Border = theme.Border

		innerW := w - 4
		head := header(theme, innerW, "galería de temas", cats, catIdx)
		search := searchBar(theme, innerW, filter, "filtra temas… (Enter aplica · / escribe)", theme.Icons.Search)

		listW := clampInt(w/3, 24, 36)
		bodyH := h - 8
		if bodyH < 8 {
			bodyH = 8
		}
		leftBlock := pane.render(theme, listW, bodyH, "")

		// Panel derecho: previsualización animada + paleta + ventana de ejemplo.
		rightW := innerW - listW - 5
		if rightW < 20 {
			rightW = 20
		}
		var right []string
		// 1. Fondo animado del tema con el nombre superpuesto.
		wallH := clampInt(bodyH/3, 4, 9)
		if engine != nil && live {
			frame := wallpaper.PreviewBlock(engine, rightW, wallH)
			right = append(right, frame...)
		} else {
			for i := 0; i < wallH; i++ {
				right = append(right, ui.Style{Bg: preview.BgAlt}.Render(strings.Repeat(" ", rightW)))
			}
		}
		// Nombre del tema y categoría sobre el panel de datos.
		nameLine := " " + ui.Style{Fg: preview.Fg, Bg: preview.BgSel}.B().Render(" "+preview.Name+" ") +
			" " + badge(theme, preview.Category, preview.Bg, preview.Accent)
		right = append(right, nameLine, "")
		// 2. Muestras de color y degradado.
		colors := []ui.Color{preview.Bg, preview.BgAlt, preview.BgSel, preview.Muted,
			preview.Fg, preview.Accent, preview.Accent2, preview.Ok, preview.Warn, preview.Err, preview.Border}
		right = append(right, " "+pt.Swatches(colors, max(1, (rightW-4)/len(colors))))
		g := preview.GradientOr()
		right = append(right, " "+pt.GradientBar(rightW-2, g[0], g[1]))
		right = append(right, "")
		// 3. Ventana de ejemplo con el estilo del tema.
		mockH := clampInt(bodyH-wallH-8, 4, 7)
		right = append(right, pt.MockWindow(rightW-1, mockH, "swaydesk · vista previa", "demo — foot")...)
		right = append(right, "")
		// 4. Metadatos.
		kind := "oscuro"
		if preview.Light {
			kind = "claro"
		}
		scene := preview.Scene
		if scene == "" {
			scene = "aurora"
		}
		if info, ok := wallpaper.SceneByKey(scene); ok {
			scene = info.Name
		}
		right = append(right,
			metaLine(theme, "tipo", kind, rightW),
			metaLine(theme, "acento", string(preview.Accent)+"  "+string(preview.Accent2), rightW),
			metaLine(theme, "fondo", scene, rightW),
		)

		cols := joinColumns(leftBlock, listW, right, 2, theme.Palette.Border, theme.Palette.Bg)
		// Recorta/alinea el cuerpo a la altura disponible.
		body := make([]string, 0, bodyH)
		for i := 0; i < bodyH; i++ {
			if i < len(cols) {
				body = append(body, cols[i])
			} else {
				body = append(body, "")
			}
		}

		footer := footerBar(theme,
			[2]string{"↑↓", "elegir"}, [2]string{"Tab", "categoría"},
			[2]string{"↵", "aplicar"}, [2]string{"espacio", "animación"},
			[2]string{"esc", "salir"})

		content := []string{head[0], head[1], search, ""}
		content = append(content, body...)
		return theme.Frame(w, h, "swaydesk · temas", footer, content)
	}

	err = sc.Loop(view, func(k ui.Key) bool {
		switch {
		case k.Is("esc"):
			return true
		case k.Is("enter"):
			if e, ok := pane.current(); ok {
				result.Key = e.Key
				applied = true
				return true
			}
		case k.Is("tab"):
			catIdx = (catIdx + 1) % len(cats)
			pane.cursor, pane.offset = 0, 0
			rebuild()
		case k.Is("backtab"):
			catIdx = (catIdx + len(cats) - 1) % len(cats)
			pane.cursor, pane.offset = 0, 0
			rebuild()
		case k.Is("up"):
			pane.move(-1)
			rebuild()
		case k.Is("down"):
			pane.move(1)
			rebuild()
		case k.Is("pgup"):
			pane.move(-8)
			rebuild()
		case k.Is("pgdn"):
			pane.move(8)
			rebuild()
		case k.Is("home"):
			pane.cursor = 0
			rebuild()
		case k.Is("end"):
			pane.cursor = len(pane.entries) - 1
			rebuild()
		case k.IsRune(' '):
			live = !live
		case k.Is("backspace"):
			if len(filter) > 0 {
				filter = filter[:len(filter)-1]
				rebuild()
			}
		case k.Rune >= 32 && !k.Ctrl && !k.Alt:
			filter += string(k.Rune)
			rebuild()
		}
		return false
	}, ticker.C)
	if engine != nil {
		engine.Close()
	}
	return ThemeResult{Key: result.Key, Applied: applied}, err
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// DescribeTheme devuelve un resumen de una paleta en una línea.
func DescribeTheme(p ui.Palette) string {
	kind := "oscuro"
	if p.Light {
		kind = "claro"
	}
	return fmt.Sprintf("%-18s %-14s %s → %s", p.Name, p.Category+" · "+kind, p.Accent, p.Accent2)
}
