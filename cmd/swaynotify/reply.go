package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// runReply abre el diálogo de respuesta rápida de una notificación.
//
// Se lanza desde el daemon al pulsar «Responder» en una tarjeta; la ventana es
// una terminal flotante, así que el teclado, los acentos y el portapapeles
// funcionan sin reimplementar xkbcommon en el panel.
//
//	swaynotify reply <id> [texto]
func runReply(args []string) {
	cfg, _ := config.Load()
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	if len(args) == 0 {
		fail(fmt.Errorf("uso: swaynotify reply <id> [texto]"))
	}
	id, err := atoi(args[0])
	if err != nil || id <= 0 {
		fail(fmt.Errorf("identificador inválido: %q", args[0]))
	}
	// Texto directo (para scripts y pruebas): no hace falta interfaz.
	if len(args) > 1 {
		sendReply(theme, uint32(id), strings.Join(args[1:], " "), false)
		return
	}

	client, err := notify.DialClient()
	if err != nil {
		fail(fmt.Errorf("no hay daemon de notificaciones: %w", err))
	}
	defer client.Close()

	n, ok := findNotification(client, uint32(id))
	if !ok {
		fail(fmt.Errorf("la notificación %d ya no existe", id))
	}
	if !n.InlineReply {
		fail(fmt.Errorf("la notificación %d no admite respuesta", id))
	}

	placeholder := n.ReplyPlaceholder
	if placeholder == "" {
		placeholder = "Escribe tu respuesta…"
	}
	if ui.IsInteractive() {
		text, ok := prompt(theme, uint32(id), n, placeholder)
		if !ok {
			return
		}
		sendReply(theme, uint32(id), text, true)
		return
	}
	// Sin TTY (por ejemplo, desde un script): se lee una línea de la entrada.
	var line string
	if _, err := fmt.Fscanln(os.Stdin, &line); err != nil || line == "" {
		fail(fmt.Errorf("no hay texto que enviar"))
	}
	sendReply(theme, uint32(id), line, false)
}

// findNotification busca una notificación viva por id.
func findNotification(c *notify.Client, id uint32) (notify.Notification, bool) {
	list, err := c.ListVisible()
	if err != nil {
		return notify.Notification{}, false
	}
	for _, n := range list {
		if n.ID == id {
			return n, true
		}
	}
	return notify.Notification{}, false
}

// sendReply entrega la respuesta y muestra el resultado.
func sendReply(theme *ui.Theme, id uint32, text string, clearScreen bool) {
	text = notify.SanitizeReply(text)
	if text == "" {
		fail(fmt.Errorf("la respuesta está vacía"))
	}
	client, err := notify.DialClient()
	if err != nil {
		fail(err)
	}
	defer client.Close()
	if err := client.Reply(id, text); err != nil {
		fail(err)
	}
	ok := theme.Icons.Check
	msg := fmt.Sprintf("%s respuesta enviada · %s", ok, notify.Shorten(text, 48))
	fmt.Println(ui.Style{Fg: theme.Palette.Ok}.Render(msg))
	_ = clearScreen
}

// prompt muestra el cuadro de escritura y devuelve el texto.
func prompt(theme *ui.Theme, id uint32, n notify.Notification, placeholder string) (string, bool) {
	screen, err := ui.OpenScreen()
	if err != nil {
		fail(err)
	}
	defer screen.Close()

	text := []rune{}
	scrolled := false
	done := false
	accepted := false

	err = screen.Loop(
		func(w, h int) string {
			return replyFrame(theme, w, h, n, string(text), placeholder, scrolled)
		},
		func(k ui.Key) bool {
			switch {
			case k.Name == "enter":
				done, accepted = true, true
				return true
			case k.Name == "esc" || (k.Ctrl && k.Rune == 'c'):
				done = true
				return true
			case k.Name == "backspace":
				if len(text) > 0 {
					text = text[:len(text)-1]
				}
			case k.Ctrl && k.Rune == 'u':
				text = nil
			case k.Rune != 0 && !k.Ctrl:
				if len(text) < notify.MaxReplyLen {
					text = append(text, k.Rune)
				}
			}
			return false
		}, nil)
	if err != nil {
		fail(err)
	}
	if !done || !accepted {
		return "", false
	}
	return string(text), true
}

// replyFrame compone la pantalla del diálogo.
func replyFrame(theme *ui.Theme, w, h int, n notify.Notification, text, placeholder string, scrolled bool) string {
	pal := theme.Palette
	if w < 12 || h < 5 {
		return ""
	}
	innerW := w - 2
	fit := func(s string) string { return ui.TruncateANSI(s, innerW) }
	sty := func(fg, bg ui.Color) ui.Style { return ui.Style{Fg: fg, Bg: bg} }

	title := fmt.Sprintf(" responder a %s · #%d ", appName(n), n.ID)
	lineas := []string{
		fit(sty(pal.Fg, pal.Bg).B().Render(ui.Truncate(n.Summary, innerW-2))),
		fit(sty(pal.Muted, pal.Bg).Render(ui.Truncate(appName(n)+" · "+ui.Truncate(n.Body, innerW), innerW))),
		"",
	}

	// Campo de escritura: el texto se desplaza para dejar ver el cursor.
	runas := []rune(text)
	anchoCampo := innerW - 3
	if anchoCampo < 4 {
		anchoCampo = 4
	}
	offset := 0
	if len(runas) > anchoCampo {
		offset = len(runas) - anchoCampo
	}
	visible := string(runas[offset:])
	estiloCampo := sty(pal.Fg, pal.BgAlt)
	if visible == "" {
		visible = ui.Truncate(placeholder, anchoCampo)
		estiloCampo = sty(pal.Muted, pal.BgAlt)
	}
	campo := " " + ui.PadRight(visible, anchoCampo)
	lineas = append(lineas, fit(estiloCampo.Render(campo)+sty(pal.Bg, pal.Accent).Render(" ")))

	contador := fmt.Sprintf("%d/%d", len(runas), notify.MaxReplyLen)
	lineas = append(lineas, fit(sty(pal.Muted, pal.Bg).Render(contador)), "")

	if scrolled {
		lineas = append(lineas, fit(sty(pal.Warn, pal.Bg).Render("la notificación ya no existe")))
	}

	// Pistas: se van quitando las menos importantes si no caben.
	pistas := [][2]string{
		{"Enter", "enviar"},
		{"Esc", "cancelar"},
		{"Ctrl+u", "borrar"},
	}
	var hints string
	for len(pistas) > 0 {
		hints = theme.HintBar(pistas...)
		if ui.Width(hints) <= innerW {
			break
		}
		pistas = pistas[:len(pistas)-1]
	}
	if hints != "" && ui.Width(hints) <= innerW {
		lineas = append(lineas, fit(hints))
	}

	// El marco solo puede mostrar h-2 líneas.
	if len(lineas) > h-2 {
		lineas = lineas[:h-2]
	}
	return theme.Frame(w, h, title, "swaynotify", lineas)
}

func appName(n notify.Notification) string {
	if strings.TrimSpace(n.App) == "" {
		return "sistema"
	}
	return n.App
}
