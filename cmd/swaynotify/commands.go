package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/notify"
)

// historyPath es el archivo donde el daemon guarda el historial.
func historyPath(cfg *config.Config) string {
	if p := os.Getenv("SWAYNOTIFY_HISTORY"); p != "" {
		return p
	}
	return filepath.Join(cfg.Cache, "notificaciones.json")
}

// runSend envía una notificación por D-Bus o, si no hay daemon, con
// notify-send (para que `swaydesk notify` siga funcionando sin swaynotify).
func runSend(args []string) {
	vals, pos := parseFlags(args, map[string]bool{"action": true})
	if len(pos) == 0 {
		fmt.Fprintln(os.Stderr, "swaynotify: falta el resumen de la notificación")
		os.Exit(2)
	}
	n := notify.Notification{
		App:      orDefault(last(vals, "app"), "swaydesk"),
		Icon:     last(vals, "icon"),
		Summary:  pos[0],
		Progress: -1,
	}
	if len(pos) > 1 {
		n.Body = strings.Join(pos[1:], " ")
	}
	switch strings.ToLower(last(vals, "urgency")) {
	case "baja", "low":
		n.Urgency = notify.Low
	case "urgente", "critical", "crítica", "critica":
		n.Urgency = notify.Critical
	}
	if _, ok := vals["reply"]; ok || last(vals, "reply-hint") != "" {
		// Respuesta rápida: la tarjeta muestra un botón que abre el diálogo.
		n.InlineReply = true
		n.ReplyPlaceholder = last(vals, "reply-hint")
	}
	if _, ok := vals["resident"]; ok {
		n.Resident = true
	}
	if t := last(vals, "timeout"); t != "" {
		if ms, err := atoi(t); err == nil {
			n.Timeout = ms
		}
	}
	n.Category = last(vals, "category")
	n.GroupID = last(vals, "group")
	if p := last(vals, "image"); p != "" {
		// Se comprueba al enviar: si la ruta no existe, se avisa en vez de
		// mandar una tarjeta sin la imagen que el usuario esperaba.
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintf(os.Stderr, "swaynotify: no se pudo leer la imagen: %v\n", err)
			os.Exit(1)
		}
		n.ImagePath = p
	}
	for _, spec := range vals["action"] {
		key, label := spec, spec
		if i := strings.IndexByte(spec, '='); i > 0 {
			key, label = spec[:i], spec[i+1:]
		}
		n.Actions = append(n.Actions, notify.Action{Key: key, Label: label})
	}

	if client, err := notify.DialClient(); err == nil {
		defer client.Close()
		if _, err := client.Send(n); err == nil {
			return
		}
	}
	// Reserva: notify-send del escritorio.
	fallbackNotifySend(n)
}

// fallbackNotifySend usa notify-send cuando no hay servidor escribiendo en el
// bus (por ejemplo, si swaynotify no está arrancado).
func fallbackNotifySend(n notify.Notification) {
	if !launcher.HasCommand("notify-send") {
		fmt.Fprintln(os.Stderr, "swaynotify: no hay daemon de notificaciones ni notify-send")
		os.Exit(1)
	}
	args := []string{}
	switch n.Urgency {
	case notify.Low:
		args = append(args, "-u", "low")
	case notify.Critical:
		args = append(args, "-u", "critical")
	}
	if n.Icon != "" {
		args = append(args, "-i", n.Icon)
	}
	if n.App != "" {
		args = append(args, "-a", n.App)
	}
	if n.Timeout > 0 {
		args = append(args, "-t", fmt.Sprint(n.Timeout))
	}
	args = append(args, n.Summary)
	if n.Body != "" {
		args = append(args, n.Body)
	}
	if err := launcher.StartDetached("notify-send", args...); err != nil {
		fmt.Fprintf(os.Stderr, "swaynotify: %v\n", err)
		os.Exit(1)
	}
}

// runDND cambia el modo «no molestar» del daemon.
func runDND(args []string) {
	want := "toggle"
	if len(args) > 0 {
		want = strings.ToLower(args[0])
	}
	c, err := notify.DialClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaynotify: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()
	var reply bool
	switch want {
	case "on", "1", "true", "sí", "si":
		r, err := c.Call("SetDND", "b", true)
		if err != nil {
			fail(err)
		}
		reply, _ = r.Body[0].(bool)
	case "off", "0", "false", "no":
		r, err := c.Call("SetDND", "b", false)
		if err != nil {
			fail(err)
		}
		reply, _ = r.Body[0].(bool)
	case "toggle", "":
		r, err := c.Call("ToggleDND", "")
		if err != nil {
			fail(err)
		}
		reply, _ = r.Body[0].(bool)
	default:
		fmt.Fprintf(os.Stderr, "swaynotify: opción desconocida %q (on, off, toggle)\n", want)
		os.Exit(2)
	}
	if reply {
		fmt.Println("no molestar: activado")
	} else {
		fmt.Println("no molestar: desactivado")
	}
}

// runStatus muestra el estado del daemon.
func runStatus(args []string) {
	c, err := notify.DialClient()
	if err != nil {
		fmt.Printf("swaynotify: parado (%v)\n", err)
		os.Exit(1)
	}
	defer c.Close()
	r, err := c.Call("GetStatus", "")
	if err != nil {
		fmt.Printf("swaynotify: no hay daemon atendiendo (%v)\n", err)
		os.Exit(1)
	}
	st, _ := r.Body[0].(string)
	fmt.Println(st)
	var status notify.Status
	if err := json.Unmarshal([]byte(st), &status); err != nil || !status.Ready {
		// El nombre D-Bus ya puede estar registrado mientras se prepara la
		// superficie. El autostart espera hasta que el primer fotograma se ve.
		os.Exit(1)
	}
}

// runHistory imprime el historial guardado por el daemon.
func runHistory(args []string) {
	cfg, _ := config.Load()
	path := historyPath(cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("sin historial de notificaciones")
			return
		}
		fail(err)
	}
	if hasFlag(args, "json") {
		fmt.Println(string(data))
		return
	}
	var recs []notify.Record
	if err := json.Unmarshal(data, &recs); err != nil {
		fail(fmt.Errorf("historial corrupto: %w", err))
	}
	if len(recs) == 0 {
		fmt.Println("sin historial de notificaciones")
		return
	}
	total := len(recs)
	if n := len(recs); n > 20 {
		recs = recs[n-20:]
	}
	for _, r := range recs {
		fmt.Printf("%s  %-12s  %s\n", r.ClosedAt.Local().Format("02/01 15:04"),
			truncate(r.App, 12), r.Summary)
		if r.Body != "" {
			fmt.Printf("              %s\n", truncate(notify.Sanitize(r.Body, 70), 70))
		}
	}
	fmt.Printf("\n%d entradas (%s)\n", total, path)
}

// runClear cierra las notificaciones visibles; con --all también el historial.
func runClear(args []string) {
	all := hasFlag(args, "all") || hasFlag(args, "todo")
	c, err := notify.DialClient()
	if err != nil {
		fail(err)
	}
	defer c.Close()
	r, err := c.Call("CloseAll", "")
	if err != nil {
		fail(err)
	}
	n := 0
	if len(r.Body) > 0 {
		if v, ok := r.Body[0].(uint32); ok {
			n = int(v)
		}
	}
	if all {
		if _, err := c.Call("ClearHistory", ""); err != nil {
			fail(err)
		}
		fmt.Printf("%d notificaciones cerradas y historial borrado\n", n)
		return
	}
	fmt.Printf("%d notificaciones cerradas\n", n)
}

// runPreview dibuja tarjetas de ejemplo y las guarda como PNG.
func runPreview(args []string) {
	vals, _ := parseFlags(args, nil)
	out := last(vals, "out")
	cfg, _ := config.Load()
	if out == "" {
		out = filepath.Join(cfg.Cache, "notificaciones.png")
	}
	backdrop := !hasFlag(args, "transparent")
	if err := renderPreviewWith(cfg, out, backdrop); err != nil {
		fail(err)
	}
	fmt.Println(out)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "swaynotify: %v\n", err)
	os.Exit(1)
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == "--"+name || a == "-"+name || a == name {
			return true
		}
	}
	return false
}

func atoi(s string) (int, error) {
	n := 0
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("valor no numérico")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("valor no numérico: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

func truncate(s string, n int) string {
	return notify.Shorten(s, n)
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

var _ = time.Now
