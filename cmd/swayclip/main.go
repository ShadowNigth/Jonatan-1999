// swayclip muestra el historial del portapapeles de Wayland (cliphist o
// clipman) con búsqueda difusa, vista previa y copia al pulsar Enter.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/actions"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// clipItem es una entrada del historial.
type clipItem struct {
	id      string
	raw     string // línea original (necesaria para borrar con cliphist)
	content string
	label   string
	hint    string
}

// backend abstrae el gestor de portapapeles disponible.
type backend interface {
	name() string
	list() ([]clipItem, error)
	copy(clipItem) error
	remove(clipItem) error
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "swayclip · historial del portapapeles\n\nUso: swayclip [--backend auto|cliphist|clipman|cliente]")
		fmt.Fprintln(os.Stderr, "  Enter copia la entrada seleccionada, Ctrl+D la borra, Esc cierra.")
	}
	backendName := flag.String("backend", "auto", "backend a usar: auto, cliphist, clipman o cliente")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swayclip: %v\n", err)
	}
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	b := pickBackend(*backendName)
	if b == nil {
		fmt.Fprintln(os.Stderr, "swayclip: no se encontró ningún gestor de portapapeles.")
		fmt.Fprintln(os.Stderr, "Instala uno con: sudo pacman -S cliphist wl-clipboard")
		os.Exit(1)
	}

	items, err := b.list()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swayclip: %v\n", err)
		os.Exit(1)
	}
	if len(items) == 0 {
		fmt.Println("swayclip: el historial está vacío.")
		fmt.Println("Asegúrate de tener en marcha:")
		fmt.Println("  wl-paste --type text --watch cliphist store")
		return
	}

	run(theme, cfg, b, items)
}

func pickBackend(name string) backend {
	switch name {
	case "cliphist":
		return cliphist{}
	case "clipman":
		return clipman{}
	case "cliente", "client", "wl-paste":
		return client{}
	}
	if launcher.HasCommand("cliphist") {
		return cliphist{}
	}
	if launcher.HasCommand("clipman") {
		return clipman{}
	}
	if launcher.HasCommand("wl-paste") {
		return client{}
	}
	return nil
}

// ───────────────────────── backends ─────────────────────────

type cliphist struct{}

func (cliphist) name() string { return "cliphist" }

func (cliphist) list() ([]clipItem, error) {
	out, err := launcher.Output("cliphist", "list")
	if err != nil {
		return nil, fmt.Errorf("cliphist list: %w", err)
	}
	var items []clipItem
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		id := ""
		content := line
		if i := strings.IndexByte(line, '\t'); i >= 0 {
			id = line[:i]
			content = line[i+1:]
		}
		// cliphist sustituye los saltos de línea por la secuencia "\n".
		readable := strings.ReplaceAll(content, "\\n", "\n")
		items = append(items, clipItem{
			id: id, raw: line, content: readable,
			label: firstLine(readable),
			hint:  fmt.Sprintf("%s · %s", linesLabel(readable), humanBytes(len(readable))),
		})
	}
	return items, nil
}

func (cliphist) copy(it clipItem) error {
	cmd := exec.Command("sh", "-c", "cliphist decode "+it.id+" | wl-copy")
	return cmd.Run()
}

func (cliphist) remove(it clipItem) error {
	cmd := exec.Command("cliphist", "delete")
	cmd.Stdin = strings.NewReader(it.raw + "\n")
	return cmd.Run()
}

type clipman struct{}

func (clipman) name() string { return "clipman" }

func (clipman) list() ([]clipItem, error) {
	out, err := launcher.Output("clipman", "list")
	if err != nil {
		return nil, fmt.Errorf("clipman list: %w", err)
	}
	var items []clipItem
	for i, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		content := strings.ReplaceAll(line, "\\n", "\n")
		items = append(items, clipItem{
			id: fmt.Sprintf("%d", i+1), raw: line, content: content,
			label: firstLine(content),
			hint:  fmt.Sprintf("%s · %s", linesLabel(content), humanBytes(len(content))),
		})
	}
	return items, nil
}

func (clipman) copy(it clipItem) error {
	cmd := exec.Command("sh", "-c", "clipman get "+it.id+" | wl-copy")
	return cmd.Run()
}

func (clipman) remove(it clipItem) error {
	// clipman no permite borrar entradas concretas desde la CLI.
	return fmt.Errorf("clipman no permite borrar entradas individuales")
}

// client es el modo mínimo: sólo muestra el contenido actual del portapapeles.
type client struct{}

func (client) name() string { return "cliente wl-clipboard" }

func (client) list() ([]clipItem, error) {
	if _, err := exec.LookPath("wl-paste"); err != nil {
		return nil, fmt.Errorf("wl-paste no está instalado")
	}
	out, err := launcher.Output("wl-paste", "--no-newline")
	if err != nil {
		return nil, fmt.Errorf("el portapapeles está vacío")
	}
	if out == "" {
		return nil, nil
	}
	return []clipItem{{
		id: "0", content: out, label: firstLine(out),
		hint: fmt.Sprintf("%s · %s", linesLabel(out), humanBytes(len(out))),
	}}, nil
}

func (client) copy(it clipItem) error {
	return launcher.WriteToCommand(it.content, "wl-copy")
}

func (client) remove(clipItem) error {
	return fmt.Errorf("no hay historial que borrar con el backend 'cliente'")
}

// ───────────────────────── interfaz ─────────────────────────

func run(theme *ui.Theme, cfg *config.Config, b backend, items []clipItem) {
	sc, err := ui.OpenScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swayclip: %v\n", err)
		os.Exit(1)
	}
	defer sc.Close()

	sel := ui.NewSelector(theme, "swaydesk · portapapeles ("+b.name()+")", nil)
	sel.Placeholder = "busca en el historial…"

	var (
		filter    []clipItem
		cursor    int
		offset    int
		status    string
		previewOf = func() []string {
			if len(filter) == 0 {
				return nil
			}
			it := filter[cursor]
			return ui.Wrap(ui.Sanitize(it.content), sc.W-8)
		}
	)
	rebuild := func() {
		list := make([]ui.Item, 0, len(items))
		for _, it := range items {
			list = append(list, ui.Item{Key: it.id, Label: it.label, Hint: it.hint})
		}
		sel.SetItems(list)
	}
	rebuild()
	applyFilter := func() {
		filter = nil
		for _, it := range items {
			if sel.Filter == "" {
				filter = append(filter, it)
				continue
			}
			if _, ok := ui.Fuzzy(sel.Filter, it.label+" "+it.content); ok {
				filter = append(filter, it)
			}
		}
		if cursor >= len(filter) {
			cursor = 0
		}
	}
	applyFilter()

	_ = sc.Loop(
		func(w, h int) string { return viewClip(theme, sel, filter, cursor, offset, status, previewOf(), w, h) },
		func(k ui.Key) bool {
			switch {
			case k.Is("esc"):
				if sel.Filter != "" {
					sel.ClearFilter()
					applyFilter()
					return false
				}
				return true
			case k.Is("enter"):
				if len(filter) == 0 {
					return true
				}
				if err := b.copy(filter[cursor]); err != nil {
					status = "error: " + err.Error()
					return false
				}
				actions.Notification("Portapapeles", "Entrada copiada", "low")
				return true
			case k.Ctrl && k.IsRune('d'):
				if len(filter) == 0 {
					return false
				}
				if err := b.remove(filter[cursor]); err != nil {
					status = err.Error()
					return false
				}
				removed := filter[cursor]
				var kept []clipItem
				for _, it := range items {
					if it.raw != removed.raw {
						kept = append(kept, it)
					}
				}
				items = kept
				status = "entrada borrada"
				rebuild()
				applyFilter()
			case k.Is("up") || (k.Ctrl && k.IsRune('p')):
				if cursor > 0 {
					cursor--
				}
			case k.Is("down") || (k.Ctrl && k.IsRune('n')):
				if cursor < len(filter)-1 {
					cursor++
				}
			case k.Is("pgup"):
				cursor = max(0, cursor-8)
			case k.Is("pgdn"):
				cursor = min(len(filter)-1, cursor+8)
			case k.Is("home"):
				cursor = 0
			case k.Is("end"):
				cursor = max(0, len(filter)-1)
			case k.Is("backspace"):
				sel.Backspace()
				applyFilter()
			case k.Ctrl && k.IsRune('u'):
				sel.ClearFilter()
				applyFilter()
			case k.Rune >= 32 && !k.Ctrl && !k.Alt:
				sel.Type(k.Rune)
				applyFilter()
			}
			return false
		}, nil)
}

// viewClip dibuja la lista y la vista previa de la entrada seleccionada.
func viewClip(t *ui.Theme, sel *ui.Selector, items []clipItem, cursor, offset int, status string, preview []string, w, h int) string {
	innerW := w - 2
	p := t.Palette

	head := " " + ui.Style{Fg: p.Accent, Bg: p.BgAlt}.B().Render(" "+t.Icons.Scroll+" ") +
		" " + ui.Style{Fg: p.Fg, Bg: p.BgAlt}.Render(sel.Filter) +
		ui.Style{Fg: p.Accent, Bg: p.BgAlt}.Render("▏")
	if sel.Filter == "" {
		head = " " + ui.Style{Fg: p.Accent, Bg: p.BgAlt}.B().Render(" "+t.Icons.Search+" ") +
			" " + ui.Style{Fg: p.Muted, Bg: p.BgAlt}.Render(sel.Placeholder) +
			ui.Style{Fg: p.Accent, Bg: p.BgAlt}.Render("▏")
	}
	head = ui.PadRight(head, innerW)
	body := []string{head, t.Muted().Render(strings.Repeat("─", innerW))}

	previewH := min(6, h/3)
	listH := h - 2 - len(body) - previewH - 3
	if listH < 1 {
		listH = 1
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+listH {
		offset = cursor - listH + 1
	}

	if len(items) == 0 {
		body = append(body, " "+t.Muted().Render("sin resultados"))
	}
	for i := offset; i < len(items) && i < offset+listH; i++ {
		it := items[i]
		selected := i == cursor
		bg := p.Bg
		if selected {
			bg = p.BgSel
		}
		mark := "  "
		if selected {
			mark = ui.Style{Fg: p.Accent, Bg: bg}.Render("▌ ")
		}
		hintW := ui.Width(it.hint) + 1
		label := ui.Style{Fg: p.Fg, Bg: bg}.Render(ui.Truncate(it.label, innerW-6-hintW))
		pad := innerW - 2 - ui.Width(mark) - ui.Width(label) - hintW
		if pad < 0 {
			pad = 0
		}
		body = append(body, mark+label+ui.Style{Bg: bg}.Render(strings.Repeat(" ", pad))+
			t.Muted().Render(it.hint)+" ")
	}
	for len(body) < 2+listH {
		body = append(body, "")
	}

	body = append(body, t.Muted().Render(strings.Repeat("─", innerW)))
	label := " " + t.Muted().Render("vista previa")
	body = append(body, label)
	for i := 0; i < previewH-1; i++ {
		line := ""
		if i < len(preview) {
			line = preview[i]
		}
		body = append(body, " "+t.Faint().Render(line))
	}

	footer := t.HintBar(
		[2]string{"↵", "copiar"},
		[2]string{"↑↓", "mover"},
		[2]string{"ctrl+d", "borrar"},
		[2]string{"esc", "salir"},
	)
	if status != "" {
		footer = t.HintBar([2]string{"•", status})
	}
	return t.Frame(w, h, "swaydesk · portapapeles", footer, body)
}

// ───────────────────────── utilidades ─────────────────────────

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(ui.Truncate(ui.Sanitize(s), 90))
}

// linesLabel devuelve "1 línea" o "N líneas".
func linesLabel(s string) string {
	n := countLines(s)
	if n == 1 {
		return "1 línea"
	}
	return fmt.Sprintf("%d líneas", n)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func humanBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
