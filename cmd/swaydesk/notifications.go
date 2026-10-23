package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/dbus"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/notify"
)

// cmdNotifications gestiona el daemon de notificaciones propio.
//
//	swaydesk notifications [status|dnd on|off|toggle|history|clear|preview|test]
func cmdNotifications(args []string) {
	action := "status"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action = args[0]
		args = args[1:]
	}
	cfg, err := config.Load()
	must(err)

	switch action {
	case "status":
		notifyStatus(cfg, args)
	case "dnd", "nomolestar", "no-molestar":
		notifyDND(args)
	case "history", "historial":
		notifyHistory(cfg, args)
	case "clear", "limpiar":
		notifyClear(args)
	case "preview":
		notifyPreview(cfg, args)
	case "test", "prueba":
		notifyTest(cfg)
	case "center", "centro":
		if err := runCenter(cfg.Theme, cfg.NerdFont); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
			os.Exit(1)
		}
	case "reply", "responder":
		notifyReplyFromCLI(args)
	case "config":
		notifyConfig(cfg)
	case "help", "--help", "-h":
		notifyUsage()
	default:
		fmt.Fprintf(os.Stderr, "swaydesk: orden desconocida %q\n\n", action)
		notifyUsage()
		os.Exit(2)
	}
}

func notifyUsage() {
	fmt.Fprint(os.Stderr, `Uso: swaydesk notifications <orden>

  status                 Estado del daemon (JSON con --json)
  dnd [on|off|toggle]    Modo «no molestar»
  history [--json]       Historial de notificaciones
  clear [--all]          Cierra las visibles (--all borra el historial)
  center                 Centro interactivo (lista, filtro, responder)
  reply <id> [texto]     Responde a una notificación
  preview [--out RUTA]   PNG con tarjetas de ejemplo
  test                   Envía una notificación de prueba
  config                 Valores de la sección [notify]

El daemon se llama swaynotify y se arranca solo con la sesión.
`)
}

func notifyStatus(cfg *config.Config, args []string) {
	c, err := notify.DialClient()
	running := err == nil
	if running {
		defer c.Close()
		if r, err := c.Call("GetStatus", ""); err == nil {
			if hasFlagArg(args, "json") {
				if s, ok := r.Body[0].(string); ok {
					fmt.Println(s)
				}
				return
			}
			var st notify.Status
			if s, ok := r.Body[0].(string); ok {
				_ = json.Unmarshal([]byte(s), &st)
			}
			fmt.Printf("\n  notificaciones · swaydesk\n")
			fmt.Printf("  daemon       %s (D-Bus %s)\n", mark(running), notify.Iface)
			fmt.Printf("  en pantalla  %d · pendientes %d\n", len(st.Visible), st.Pending)
			dnd := "desactivado"
			if st.DND {
				dnd = "activado"
			}
			fmt.Printf("  no molestar  %s\n", dnd)
			fmt.Printf("  historial    %d entradas\n", st.History)
			fmt.Printf("  posición     %s · %d px de ancho · opacidad %.0f %%\n",
				cfg.NotifyPosition, cfg.NotifyWidth, cfg.NotifyOpacity*100)
			fmt.Printf("  caducidad    %d s (urgentes: %s)\n",
				cfg.NotifyTimeout/1000, criticalText(cfg.NotifyCriticalTime))
			fmt.Printf("\n  apagar: matar swaynotify · modo no molestar: swaydesk notifications dnd\n\n")
			return
		}
	}
	if hasFlagArg(args, "json") {
		fmt.Printf("{\"running\": false, \"bus\": %q}\n", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
		return
	}
	fmt.Printf("\n  notificaciones · swaydesk\n")
	fmt.Printf("  daemon       %s no está en marcha\n", mark(false))
	if launcher.HasCommand("mako") {
		fmt.Printf("  reserva      mako %s\n", mark(true))
	}
	fmt.Printf("\n  arrancar: swaynotify & · o reinicia la sesión (autostart)\n\n")
}

func notifyDND(args []string) {
	mode := "toggle"
	if len(args) > 0 {
		mode = strings.ToLower(args[0])
	}
	c, err := notify.DialClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: swaynotify no está en marcha (%v)\n", err)
		os.Exit(1)
	}
	defer c.Close()
	var (
		on bool
		r  *dbus.Message
		e  error
	)
	switch mode {
	case "on", "1", "true", "si", "sí":
		r, e = c.Call("SetDND", "b", true)
	case "off", "0", "false", "no":
		r, e = c.Call("SetDND", "b", false)
	case "toggle":
		r, e = c.Call("ToggleDND", "")
	default:
		fmt.Fprintf(os.Stderr, "swaydesk: opción desconocida %q (on, off, toggle)\n", mode)
		os.Exit(2)
	}
	if e != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", e)
		os.Exit(1)
	}
	if len(r.Body) > 0 {
		on, _ = r.Body[0].(bool)
	}
	if on {
		fmt.Println("no molestar · activado")
	} else {
		fmt.Println("no molestar · desactivado")
	}
}

func notifyHistory(cfg *config.Config, args []string) {
	path := filepath.Join(cfg.Cache, "notificaciones.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("sin historial de notificaciones")
			return
		}
		must(err)
	}
	if hasFlagArg(args, "json") {
		fmt.Println(string(data))
		return
	}
	var recs []notify.Record
	if err := json.Unmarshal(data, &recs); err != nil {
		must(fmt.Errorf("historial corrupto: %w", err))
	}
	if len(recs) == 0 {
		fmt.Println("sin historial de notificaciones")
		return
	}
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].ClosedAt.Before(recs[j].ClosedAt) })
	if len(recs) > 20 {
		recs = recs[len(recs)-20:]
	}
	for _, r := range recs {
		app := r.App
		if app == "" {
			app = "sistema"
		}
		fmt.Printf("  %s  %-12s  %s\n", r.ClosedAt.Local().Format("02/01 15:04"),
			truncateRunes(app, 12), r.Summary)
		if body := notify.Sanitize(r.Body, 74); body != "" {
			fmt.Printf("                %s\n", body)
		}
	}
	fmt.Printf("\n  %s\n", path)
}

func notifyClear(args []string) {
	c, err := notify.DialClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: swaynotify no está en marcha (%v)\n", err)
		os.Exit(1)
	}
	defer c.Close()
	r, err := c.Call("CloseAll", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		os.Exit(1)
	}
	n := 0
	if len(r.Body) > 0 {
		if v, ok := r.Body[0].(uint32); ok {
			n = int(v)
		}
	}
	if hasFlagArg(args, "all") {
		if _, err := c.Call("ClearHistory", ""); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		}
		fmt.Printf("%d notificaciones cerradas · historial borrado\n", n)
		return
	}
	fmt.Printf("%d notificaciones cerradas\n", n)
}

func notifyPreview(cfg *config.Config, args []string) {
	out := optValue(args, "out")
	if out == "" {
		out = filepath.Join(cfg.Cache, "notificaciones.png")
	}
	// Una notificación de ejemplo por cada tipo, para ver la vista previa.
	now := time.Now()
	items := []notify.Notification{
		{ID: 1, App: "swaydesk", Summary: "Tema aplicado",
			Body: "Sakura · 31 paletas disponibles", Urgency: notify.Normal,
			Created: now, Expires: now.Add(6 * time.Second), Timeout: 6000, Progress: -1},
		{ID: 2, App: "pacman", Summary: "Actualización pendiente",
			Body: "12 paquetes nuevos, entre ellos el kernel", Urgency: notify.Critical,
			Created: now, Progress: 42,
			Actions: []notify.Action{{Key: "default", Label: "Actualizar"}, {Key: "later", Label: "Luego"}}},
	}
	backdrop := !hasFlagArg(args, "transparent")
	if err := writeNotifyPNGWith(cfg, items, out, backdrop); err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}

// notifyReplyFromCLI delega en swaynotify para no duplicar el diálogo.
func notifyReplyFromCLI(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: swaydesk notifications reply <id> [texto]")
		os.Exit(2)
	}
	exe := "swaynotify"
	if self, err := os.Executable(); err == nil {
		if hermano := filepath.Join(filepath.Dir(self), "swaynotify"); fileExists(hermano) {
			exe = hermano
		}
	}
	if !launcher.HasCommand(exe) && !fileExists(exe) {
		fmt.Fprintln(os.Stderr, "swaydesk: falta swaynotify")
		os.Exit(1)
	}
	// Se ejecuta en la misma terminal: el diálogo necesita el TTY.
	if err := launcher.ExecCommand(exe, append([]string{"reply"}, args...)...); err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		os.Exit(1)
	}
}

func notifyTest(cfg *config.Config) {
	// Tres avisos de muestra: uno informativo, un chat con respuesta rápida y
	// una descarga con progreso, para ver de un vistazo todo lo que dibuja el
	// daemon.
	now := time.Now()
	// Tres mensajes con la misma etiqueta de pila: se apilan en una tarjeta.
	for i, texto := range []string{"¿Quedamos el sábado?", "Traigo el postre", "¿Comemos a las dos?"} {
		_ = notifySend(notify.Notification{
			App: "Chat de prueba", GroupID: "chat-marta", Summary: "Marta", Sender: "Marta",
			Body: texto, Urgency: notify.Normal, Progress: -1, Created: now.Add(time.Duration(i) * time.Second),
			InlineReply: i == 2, ReplyPlaceholder: "Responder a Marta…",
			Icon:    "mail-unread",
			Timeout: -1,
		})
	}
	// Una tarjeta con carátula: la imagen se genera en la caché y se pasa con
	// image-path, el mismo hint que usan los reproductores.
	if cover := demoCover(cfg.Cache, cfg.Theme); cover != "" {
		_ = notifySend(notify.Notification{
			App: "swaydesk", Summary: "Lofi Girl · Midnight Study",
			Body:    "Reproduciendo la lista «estudio»",
			Urgency: notify.Low, Icon: "audio-x-generic", Progress: -1,
			Created: now, ImagePath: cover, Timeout: -1,
		})
	}
	_ = notifySend(notify.Notification{
		App: "swaydesk", Summary: "Descarga completada",
		Body: "archlinux-2026.10.01-x86_64.iso", Urgency: notify.Low,
		Icon: "folder-download", Progress: 100, Created: now,
	})
	n := notify.Notification{
		App: "swaydesk", Summary: "Notificaciones funcionando",
		Body:    "Este aviso lo dibuja <b>swaynotify</b> con el tema activo.",
		Urgency: notify.Normal, Icon: "dialog-information", Progress: -1,
	}
	if notifySend(n) {
		fmt.Println("notificación enviada a swaynotify")
		return
	}
	if launcher.HasCommand("notify-send") {
		_ = launcher.StartDetached("notify-send", "-a", "swaydesk", n.Summary, n.Body)
		fmt.Println("notificación enviada con notify-send")
		return
	}
	fmt.Fprintln(os.Stderr, "swaydesk: no hay daemon de notificaciones ni notify-send")
	os.Exit(1)
}

func notifyConfig(cfg *config.Config) {
	fmt.Printf("enabled              = %v\n", cfg.NotifyEnabled)
	fmt.Printf("position             = %s\n", cfg.NotifyPosition)
	fmt.Printf("width                = %d\n", cfg.NotifyWidth)
	fmt.Printf("margin               = %d\n", cfg.NotifyMargin)
	fmt.Printf("padding              = %d\n", cfg.NotifyPadding)
	fmt.Printf("gap                  = %d\n", cfg.NotifyGap)
	fmt.Printf("radius               = %d\n", cfg.NotifyRadius)
	fmt.Printf("opacity              = %.2f\n", cfg.NotifyOpacity)
	fmt.Printf("timeout_ms           = %d\n", cfg.NotifyTimeout)
	fmt.Printf("critical_timeout_ms  = %d\n", cfg.NotifyCriticalTime)
	fmt.Printf("max_visible          = %d\n", cfg.NotifyMaxVisible)
	fmt.Printf("history              = %d\n", cfg.NotifyHistory)
	fmt.Printf("max_lines            = %d\n", cfg.NotifyMaxLines)
	fmt.Printf("dnd                  = %v\n", cfg.NotifyDND)
}

// mark devuelve una marca de estado.
func mark(ok bool) string {
	if ok {
		return "✔"
	}
	return "✘"
}

// criticalText describe la caducidad de las notificaciones urgentes.
func criticalText(ms int) string {
	if ms <= 0 {
		return "no caducan"
	}
	return fmt.Sprintf("%d s", ms/1000)
}

func truncateRunes(s string, n int) string { return notify.Shorten(s, n) }

func hasFlagArg(args []string, name string) bool {
	for _, a := range args {
		if a == "--"+name || a == "-"+name {
			return true
		}
	}
	return false
}
