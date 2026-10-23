package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// notifyCenter es una pequeña TUI para revisar las notificaciones en pantalla y
// el historial, cerrarlas, responder y cambiar el modo «no molestar».
type notifyCenter struct {
	client *notify.Client
	model  *notify.Center
	theme  *ui.Theme
	status string // mensaje efímero en el pie
	bad    bool   // el mensaje es un error
	now    time.Time
}

// runCenter abre el centro de notificaciones.
func runCenter(cfgTheme string, nerdFont bool) error {
	client, err := notify.DialClient()
	if err != nil {
		return fmt.Errorf("swaynotify no está en marcha: %w", err)
	}
	defer client.Close()

	screen, err := ui.OpenScreen()
	if err != nil {
		return err
	}
	defer screen.Close()

	c := &notifyCenter{
		client: client,
		theme:  ui.NewTheme(cfgTheme, nerdFont),
		now:    time.Now(),
	}
	if err := c.reload(); err != nil {
		return err
	}

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	return screen.Loop(
		func(w, h int) string { return c.frame(w, h) },
		func(k ui.Key) bool { return c.handle(k) },
		tick.C,
	)
}

// reload vuelve a leer el estado del daemon.
func (c *notifyCenter) reload() error {
	live, err := c.client.ListVisible()
	if err != nil {
		return fmt.Errorf("no se pudo leer el estado: %w", err)
	}
	history, _ := c.client.ListHistory()
	dnd := c.model != nil && c.model.DND()
	query := ""
	sel := 0
	if c.model != nil {
		dnd = c.model.DND()
		query = c.model.Query()
		sel = c.model.Index()
	}
	if st, err := c.client.Status(); err == nil {
		dnd = st.DND
	}
	c.model = notify.NewCenter(live, history, dnd)
	c.model.SetQuery(query)
	c.model.Select(sel)
	c.now = time.Now()
	return nil
}

// handle procesa una tecla; devuelve true para salir.
func (c *notifyCenter) handle(k ui.Key) bool {
	switch {
	case k.Name == "esc":
		if c.model.Query() != "" {
			c.model.SetQuery("")
			return false
		}
		return true
	case k.Ctrl && (k.Rune == 'c' || k.Name == "ctrl-c"):
		return true
	case k.Name == "up" || k.Name == "ctrl-p" || (k.Ctrl && k.Rune == 'p'):
		c.model.Move(-1)
	case k.Name == "down" || k.Name == "ctrl-n" || (k.Ctrl && k.Rune == 'n'):
		c.model.Move(1)
	case k.Name == "backspace":
		c.model.Backspace()
	case k.Name == "enter":
		c.act()
	case k.IsRune('x'):
		c.dismiss()
	case k.IsRune('r'):
		c.reply()
	case k.IsRune('m'):
		c.toggleDND()
	case k.IsRune('c'):
		c.clearAll()
	case k.IsRune('X'):
		c.clearHistory()
	case k.Name == "home":
		c.model.Select(0)
	case k.Name == "end":
		c.model.Select(1 << 20)
	case k.Name == "f5":
		c.flash("recargado", false)
	case k.Rune != 0 && !k.Ctrl:
		c.model.Append(k.Rune)
	}
	return false
}

// act ejecuta la acción principal de la entrada seleccionada: responder si
// admite respuesta, cerrarla si sigue viva, y nada en el historial.
func (c *notifyCenter) act() {
	e, ok := c.model.Selected()
	if !ok {
		return
	}
	switch {
	case e.Live && e.InlineReply:
		c.reply()
	case e.Live:
		c.dismiss()
	default:
		c.flash("esa notificación ya está cerrada", true)
	}
}

func (c *notifyCenter) dismiss() {
	e, ok := c.model.Selected()
	if !ok {
		return
	}
	if !e.Live {
		c.flash("ya estaba cerrada", true)
		return
	}
	if _, err := c.client.Dismiss(e.ID); err != nil {
		c.flash(err.Error(), true)
		return
	}
	c.flash("cerrada: "+notify.Shorten(e.Summary, 40), false)
	_ = c.reload()
}

// reply abre el diálogo de respuesta en una terminal flotante y recarga.
func (c *notifyCenter) reply() {
	e, ok := c.model.Selected()
	if !ok {
		return
	}
	if !e.InlineReply {
		c.flash("esa notificación no admite respuesta", true)
		return
	}
	exe := "swaynotify"
	if self, err := os.Executable(); err == nil {
		// Si estamos instalados junto a swaynotify, se usa el binario hermano.
		if hermano := filepath.Join(filepath.Dir(self), "swaynotify"); fileExists(hermano) {
			exe = hermano
		}
	}
	if err := launcher.LaunchTerminal("", "swaynotify-reply", "Responder", 62, 10,
		[]string{exe, "reply", fmt.Sprint(e.ID)}); err != nil {
		c.flash("no se pudo abrir el diálogo: "+err.Error(), true)
		return
	}
	c.flash("respondiendo en una ventana nueva", false)
}

func (c *notifyCenter) toggleDND() {
	on, err := c.client.SetDND(!c.model.DND())
	if err != nil {
		c.flash(err.Error(), true)
		return
	}
	c.model.SetDND(on)
	if on {
		c.flash("no molestar activado", false)
	} else {
		c.flash("no molestar desactivado", false)
	}
}

func (c *notifyCenter) clearAll() {
	reply, err := c.client.Call("CloseAll", "")
	if err != nil {
		c.flash(err.Error(), true)
		return
	}
	n := 0
	if len(reply.Body) > 0 {
		if v, ok := reply.Body[0].(uint32); ok {
			n = int(v)
		}
	}
	c.flash(fmt.Sprintf("%d notificaciones cerradas", n), false)
	_ = c.reload()
}

func (c *notifyCenter) clearHistory() {
	if _, err := c.client.Call("ClearHistory", ""); err != nil {
		c.flash(err.Error(), true)
		return
	}
	c.flash("historial borrado", false)
	_ = c.reload()
}

func (c *notifyCenter) flash(msg string, bad bool) {
	c.status = msg
	c.bad = bad
}

// frame dibuja la pantalla completa.
func (c *notifyCenter) frame(w, h int) string {
	t := c.theme
	pal := t.Palette
	innerW := w - 2
	if innerW < 20 {
		innerW = 20
	}
	fit := func(s string) string { return ui.TruncateANSI(s, innerW) }
	bg := ui.Style{Bg: pal.Bg}

	// Cabecera: título + resumen del estado.
	title := " notificaciones "
	cabecera := ui.Style{Fg: pal.Accent, Bg: pal.Bg}.B().Render(" centro de notificaciones")
	resumen := ui.Style{Fg: pal.Muted, Bg: pal.Bg}.Render(" · " + c.model.Summary())
	if c.model.DND() {
		resumen += ui.Style{Fg: pal.Warn, Bg: pal.Bg}.Render(" · no molestar")
	}

	// Alto disponible: marco(2) + cabecera(2) + filtro(1) + pistas(1).
	listH := h - 6
	if listH < 1 {
		listH = 1
	}
	entries := c.model.Filtered()
	sel := c.model.Index()

	// Ventana de la lista centrada en la selección.
	offset := 0
	if sel >= listH {
		offset = sel - listH + 1
	}
	if extra := len(entries) - listH; offset > extra {
		offset = extra
	}
	if offset < 0 {
		offset = 0
	}

	body := []string{fit(cabecera + resumen), ""}

	if len(entries) == 0 {
		msg := "no hay notificaciones"
		switch {
		case c.model.Len() > 0:
			msg = fmt.Sprintf("nada coincide con «%s»", c.model.Query())
		case c.model.DND():
			msg = "nada en pantalla (modo no molestar)"
		}
		body = append(body, ui.Style{Fg: pal.Muted, Bg: pal.Bg}.Render("  "+msg))
	} else {
		for i := offset; i < len(entries) && i < offset+listH; i++ {
			body = append(body, c.row(entries[i], i == sel, innerW))
		}
	}

	// Línea de filtro, siempre en la misma fila para que no baile la lista.
	filtro := ui.Style{Fg: pal.Accent, Bg: pal.Bg}.Render(" / ")
	if c.model.Query() == "" {
		filtro += ui.Style{Fg: pal.Muted, Bg: pal.Bg}.Render("escribe para filtrar")
	} else {
		filtro += ui.Style{Fg: pal.Fg, Bg: pal.Bg}.Render(c.model.Query())
		contador := ui.Style{Fg: pal.Muted, Bg: pal.Bg}.Render(fmt.Sprintf("  %d/%d", len(entries), c.model.Len()))
		filtro += contador
	}
	// El filtro y las pistas van pegados al borde inferior.
	for len(body) < h-4 {
		body = append(body, "")
	}
	body = append(body, fit(filtro))

	// Pie: mensaje efímero a la izquierda y, si no hay, las pistas.
	footer := ""
	if c.status != "" {
		col := pal.Ok
		if c.bad {
			col = pal.Err
		}
		footer = ui.Style{Fg: col, Bg: pal.Bg}.Render(c.status)
	}
	hints := t.HintBar(
		[2]string{"↑↓", "mover"},
		[2]string{"Enter", "actuar"},
		[2]string{"r", "responder"},
		[2]string{"x", "cerrar"},
		[2]string{"m", "dnd"},
		[2]string{"Esc", "salir"},
	)
	anchoHints := ui.Width(hints)
	if c.status == "" || anchoHints+ui.Width(c.status)+3 > innerW {
		body = append(body, fit(hints))
	} else {
		body = append(body, fit(footer+"  "+hints))
	}
	if len(body) > h-2 {
		body = body[:h-2]
	}
	_ = bg
	return t.Frame(w, h, title, "", body)
}

// row dibuja una entrada del listado: marca, hora, título y insignias.
func (c *notifyCenter) row(e notify.Entry, selected bool, w int) string {
	t := c.theme
	pal := t.Palette
	fg, bgc := pal.Fg, pal.Bg
	muted := pal.Muted
	if selected {
		fg, bgc, muted = pal.Bg, pal.BgSel, pal.Bg
	}
	sty := func(col ui.Color) ui.Style { return ui.Style{Fg: col, Bg: bgc} }

	// Marca de urgencia: ! crítica, · baja, • viva, espacio si ya cerró.
	dot, dotCol := " ", pal.Accent
	switch {
	case e.Urgency == notify.Critical:
		dot, dotCol = "!", pal.Err
	case e.Urgency == notify.Low:
		dot, dotCol = "·", pal.Muted
	case e.Live:
		dot = "•"
	}
	if selected {
		dotCol = pal.Bg
	}

	hora := e.Created.Local().Format("15:04")
	if !e.Live {
		hora = e.ClosedAt.Local().Format("02/01 15:04")
	}

	// Insignias: solo las que quepan; primero se recorta el título.
	badges := e.Badges()
	badgeTxt := ""
	if len(badges) > 0 {
		badgeTxt = " " + strings.Join(badges, " ")
	}
	badgeMax := 26
	if len([]rune(badgeTxt)) > badgeMax {
		badgeTxt = string([]rune(badgeTxt)[:badgeMax]) + "…"
	}
	resto := w - 2 - len(hora) - 1 - ui.Width(badgeTxt) - 1
	if resto < 12 {
		// Sin sitio para insignias: se sacrifican antes que el título.
		badgeTxt = ""
		resto = w - 2 - len(hora) - 1
	}
	if resto < 8 {
		resto = 8
	}

	var b strings.Builder
	b.WriteString(sty(dotCol).Render(dot + " "))
	b.WriteString(sty(muted).Render(hora + " "))
	b.WriteString(sty(fg).Render(ui.PadRight(ui.Truncate(e.Title(), resto), resto)))
	if badgeTxt != "" {
		col := pal.Accent2
		if selected {
			col = pal.Bg
		}
		b.WriteString(sty(col).Render(badgeTxt))
	}
	line := b.String()
	if d := w - ui.Width(line); d > 0 {
		line += sty(fg).Render(strings.Repeat(" ", d))
	}
	return ui.TruncateANSI(line, w)
}
