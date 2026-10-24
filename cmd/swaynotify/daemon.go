package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/dbus"
	"github.com/ShadowNigth/swaydesk/internal/font"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wayland"
)

// daemon agrupa el estado del proceso.
type daemon struct {
	cfg    *config.Config
	store  *notify.Store
	svc    *notify.Service
	render *notify.Renderer
	conn   *dbus.Conn

	panel   *wayland.Panel
	pointer *wayland.Pointer
	scale   int
	cards   []notify.Card
	lastKey string
	// Se conservan para evitar repetir mensajes idénticos en el registro.
	lastDrawErr string
	lastResize  string
	lastFrame   string
	// stack recuerda qué pilas de notificaciones están desplegadas.
	stack *notify.Stack
}

// runDaemon arranca el servidor D-Bus y la superficie de notificaciones.
func runDaemon(args []string) {
	vals, _ := parseFlags(args, nil)
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	themeName := cfg.Theme
	if t := last(vals, "theme"); t != "" {
		themeName = t
	}
	scale := 1
	if v := last(vals, "scale"); v != "" {
		if n, err := atoi(v); err == nil && n >= 1 && n <= 3 {
			scale = n
		}
	} else if env := os.Getenv("SWAYDESK_NOTIFY_SCALE"); env != "" {
		if n, err := atoi(env); err == nil && n >= 1 && n <= 3 {
			scale = n
		}
	}

	store := notify.NewStore(notify.Options{
		Timeout:         time.Duration(cfg.NotifyTimeout) * time.Millisecond,
		CriticalTimeout: time.Duration(cfg.NotifyCriticalTime) * time.Millisecond,
		MaxVisible:      cfg.NotifyMaxVisible,
		GroupVisible:    cfg.NotifyGroup,
		HistorySize:     cfg.NotifyHistory,
		DNDBypass:       true,
	})
	if cfg.NotifyDND {
		store.SetDND(true)
	}
	_ = store.LoadHistory(historyPath(cfg))

	d := &daemon{
		cfg:   cfg,
		store: store,
		svc:   notify.NewService(store),
		scale: scale,
		stack: notify.NewStack(),
		render: notify.New(notify.RenderOptions{
			Width:    cfg.NotifyWidth,
			Padding:  cfg.NotifyPadding,
			Gap:      cfg.NotifyGap,
			Radius:   cfg.NotifyRadius,
			Opacity:  cfg.NotifyOpacity,
			FontPath: cfg.NotifyFont,
			Face:     font.Best(cfg.NotifyFont),
			Theme:    ui.PaletteByName(themeName),
			Scale:    scaleFor(cfg, scale),
			MaxLines: cfg.NotifyMaxLines,
			Shadow:   true,
		}),
	}

	// D-Bus: el nombre del servicio de notificaciones.
	conn, err := dbus.DialSession()
	if err != nil {
		fail(fmt.Errorf("D-Bus: %w", err))
	}
	if err := conn.Hello(); err != nil {
		fail(err)
	}
	d.conn = conn
	d.svc.Attach(conn)
	if err := d.svc.Register(conn); err != nil {
		fmt.Fprintf(os.Stderr, "swaynotify: %v\n", err)
		fmt.Fprintln(os.Stderr, "swaynotify: ¿hay otro daemon de notificaciones (mako, dunst) arrancado?")
		os.Exit(1)
	}

	// Wayland: la superficie donde se dibujan las tarjetas.
	wl, err := wayland.NewConn()
	if err != nil {
		fail(fmt.Errorf("Wayland: %w", err))
	}
	if _, err := wl.GetRegistry(); err != nil {
		fail(err)
	}
	anchors, mx, my := anchorsFor(cfg.NotifyPosition)
	panel, err := wayland.NewPanel(wl, wayland.PanelSpec{
		Namespace:     "swaydesk-notify",
		Layer:         wayland.LayerOverlay,
		Width:         cfg.NotifyWidth,
		Height:        1,
		Anchors:       anchors,
		ExclusiveZone: 0,
		MarginTop:     my,
		MarginBottom:  my,
		MarginLeft:    mx,
		MarginRight:   mx,
		Opaque:        false,
	})
	if err != nil {
		fail(err)
	}
	defer panel.Close()
	if err := panel.InitShm(2); err != nil {
		fail(err)
	}
	d.panel = panel

	// Ratón: izquierdo para cerrar, responder, abrir una pila o activar una
	// acción; derecho para cerrar la pila entera de un golpe.
	if p, err := wayland.NewPointer(wl, wayland.PointerHandler{
		Button: func(x, y float64, button uint32, pressed bool) {
			if !pressed || (button != 0x110 && button != 0x111) {
				return
			}
			d.click(int(x), int(y), button)
		},
	}); err == nil {
		d.pointer = p
		defer p.Close()
	}

	// Guardado periódico del historial.
	stopSave := make(chan struct{})
	defer close(stopSave)
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stopSave:
				return
			case <-t.C:
				_ = d.store.SaveHistory(historyPath(cfg))
			}
		}
	}()

	// Señales: al salir se guarda el historial.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		_ = d.store.SaveHistory(historyPath(cfg))
		os.Exit(0)
	}()

	if err := d.loop(); err != nil && !errors.Is(err, errStop) {
		fail(err)
	}
}

var errStop = errors.New("fin")

// notificationPollInterval limita la espera para despachar eventos Wayland
// cuando no llega una nueva notificación D-Bus.
const notificationPollInterval = 50 * time.Millisecond

// anchorsFor traduce la posición configurada en anclas y márgenes.
func anchorsFor(pos string) (anchors uint32, mx, my int) {
	switch strings.ToLower(pos) {
	case "top-left", "arriba-izquierda":
		return wayland.AnchorTop | wayland.AnchorLeft, 14, 14
	case "bottom-right", "abajo-derecha":
		return wayland.AnchorBottom | wayland.AnchorRight, 14, 14
	case "bottom-left", "abajo-izquierda":
		return wayland.AnchorBottom | wayland.AnchorLeft, 14, 14
	}
	return wayland.AnchorTop | wayland.AnchorRight, 14, 14
}

// scaleFor decide la escala de la fuente: la de la configuración o 1, y como
// mucho la del panel.
func scaleFor(cfg *config.Config, scale int) int {
	if cfg.NotifyFontScale > 0 {
		return cfg.NotifyFontScale
	}
	return scale
}

// loop dibuja y atiende eventos.
func (d *daemon) loop() error {
	// Dispatch no bloqueante sondea el socket de Wayland con el intervalo
	// corto anterior, evitando dejar una tarjeta esperando el tick viejo de 200 ms.
	tick := time.NewTicker(notificationPollInterval)
	defer tick.Stop()
	// Un primer dibujo para fijar el tamaño de la superficie.
	d.drawLogged()
	for {
		select {
		case <-d.svc.Dirty():
			d.drawLogged()
		case <-tick.C:
			d.svc.Expire()
			if d.panel.NeedsResize() {
				if err := d.panel.Resize(2); err != nil {
					return err
				}
				d.lastKey = "" // fuerza el repintado con el tamaño nuevo
			}
			d.drawLogged()
		}
		// Eventos del compositor (configure, entrada del ratón). Dispatch(0)
		// sondea sin bloquear, así las nuevas notificaciones siguen llegando.
		if err := d.panel.Dispatch(0); err != nil {
			if d.panel.Closed() {
				return errStop
			}
			return err
		}
	}
}

// drawLogged registra los errores de dibujo sin derribar el servicio D-Bus.
// Si el fotograma siguiente funciona, también deja constancia de la recuperación.
func (d *daemon) drawLogged() {
	if err := d.draw(); err != nil {
		if err.Error() != d.lastDrawErr {
			fmt.Fprintf(os.Stderr, "swaynotify: error al dibujar las notificaciones: %v\n", err)
		}
		d.lastDrawErr = err.Error()
		return
	}
	if d.lastDrawErr != "" {
		fmt.Fprintf(os.Stderr, "swaynotify: se recuperó el dibujo de notificaciones\n")
		d.lastDrawErr = ""
	}
}

// draw redibuja si el contenido cambió.
func (d *daemon) draw() error {
	now := time.Now()
	items := d.store.Visible()
	key := itemsKey(items, now) + "|" + strings.Join(d.stack.OpenKeys(), ",")
	if key == d.lastKey {
		return nil
	}
	var views []notify.GroupView
	if d.cfg.NotifyGroup {
		views = notify.Views(items, d.stack)
		d.stack.Prune(notify.GroupAll(items))
	} else {
		views = notify.Views(items, nil)
	}
	canvas, cards := d.render.RenderGroups(views, now)
	canvas = canvas.ScaleCanvas(d.scale)
	d.cards = cards
	// Solo las tarjetas reciben clics; el resto de la superficie deja pasar el
	// ratón a lo que haya debajo.
	rects := make([][4]int, 0, len(cards))
	for _, c := range cards {
		rects = append(rects, [4]int{
			c.Rect.X * d.scale, c.Rect.Y * d.scale,
			c.Rect.W * d.scale, c.Rect.H * d.scale,
		})
	}
	if err := d.panel.SetInputRegion(rects); err != nil {
		return fmt.Errorf("configurando la región de entrada: %w", err)
	}

	// La superficie se ajusta al contenido; hasta que el compositor confirma
	// el tamaño nuevo no se pinta (evitaría estirar las tarjetas).
	w, h := d.panel.Size()
	if w != canvas.W || h != canvas.H {
		request := fmt.Sprintf("%dx%d (actual %dx%d)", canvas.W, canvas.H, w, h)
		if request != d.lastResize {
			fmt.Fprintf(os.Stderr, "swaynotify: solicitando tamaño de panel %s\n", request)
			d.lastResize = request
		}
		if err := d.panel.RequestSize(canvas.W, canvas.H); err != nil {
			return fmt.Errorf("solicitando tamaño de panel %s: %w", request, err)
		}
		return nil
	}
	d.lastResize = ""
	if err := d.panel.Draw(canvas.Pix, canvas.W, canvas.H); err != nil {
		return fmt.Errorf("subiendo el fotograma %dx%d: %w", canvas.W, canvas.H, err)
	}
	d.lastKey = key
	frame := fmt.Sprintf("%d/%d/%dx%d", len(items), len(cards), canvas.W, canvas.H)
	if frame != d.lastFrame {
		fmt.Fprintf(os.Stderr, "swaynotify: fotograma enviado: %d notificaciones, %d tarjetas, superficie %dx%d\n",
			len(items), len(cards), canvas.W, canvas.H)
		d.lastFrame = frame
	}
	return nil
}

// openReply lanza el diálogo de respuesta rápida en una ventana flotante.
func (d *daemon) openReply(n notify.Notification) {
	exe := "swaynotify"
	if self, err := os.Executable(); err == nil {
		exe = self
	}
	args := []string{exe, "reply", fmt.Sprint(n.ID)}
	// Si la notificación desaparece mientras se escribe, el diálogo avisa y no
	// envía nada.
	if err := launcher.LaunchTerminal(d.cfg.Terminal, "swaynotify-reply", "Responder",
		62, 10, args); err != nil {
		fmt.Fprintf(os.Stderr, "swaynotify: no se pudo abrir el diálogo: %v\n", err)
		return
	}
	// Mientras se escribe, la tarjeta no debe caducar y se marca como «en
	// respuesta» para que se vea que está activa.
	d.svc.Hold(n.ID)
	d.drawLogged()
}

// itemsKey resume el estado para evitar repintados innecesarios.
func itemsKey(items []notify.Notification, now time.Time) string {
	var b strings.Builder
	for _, n := range items {
		fmt.Fprintf(&b, "%d:%d:%d:%d:%q:%q:%q:%q:%q:%q:%q:%v:%t:%q:%t:%d;",
			n.ID, n.Timeout, n.Progress, n.Urgency, n.App, n.Summary, n.Body,
			n.Icon, n.GroupID, n.ImagePath, n.Category, n.Actions, n.InlineReply,
			n.ReplyPlaceholder, n.Resident, n.Created.UnixNano())
	}
	// La barra de tiempo avanza; se repinta una vez por segundo. Los cambios
	// reales (incluido un replace con el mismo resumen) se detectan enseguida.
	fmt.Fprintf(&b, "@%d|%d", now.Unix(), len(items))
	return b.String()
}

// click busca la tarjeta (y la acción) en la posición pulsada.
func (d *daemon) click(x, y int, button uint32) {
	// Las coordenadas llegan en píxeles de la superficie.
	lx, ly := x/d.scale, y/d.scale
	for _, c := range d.cards {
		if lx < c.Rect.X || lx >= c.Rect.X+c.Rect.W ||
			ly < c.Rect.Y || ly >= c.Rect.Y+c.Rect.H {
			continue
		}
		// Botón derecho: se cierra todo lo que representa la tarjeta (la pila
		// entera si es un grupo).
		if button == 0x111 {
			d.dismissAll(c)
			return
		}
		for _, b := range c.Buttons {
			if lx < b.Rect.X || lx >= b.Rect.X+b.Rect.W ||
				ly < b.Rect.Y || ly >= b.Rect.Y+b.Rect.H {
				continue
			}
			switch b.Key {
			case notify.GroupExpandAction:
				d.setExpanded(c.GroupKey, true)
				return
			case notify.GroupCollapseAction:
				d.setExpanded(c.GroupKey, false)
				return
			case notify.InlineReplyAction:
				// La respuesta rápida necesita teclado: se abre el diálogo en
				// una ventana flotante (la terminal ya sabe de teclado y de
				// acentos, cosa que el panel no puede hacer sin xkbcommon).
				d.openReply(c.Notification)
				return
			}
			d.svc.InvokeAction(c.Notification.ID, b.Key)
			return
		}
		// Un grupo plegado (o su franja) se abre o se pliega; una tarjeta
		// suelta se cierra.
		if c.Toggle && c.GroupKey != "" {
			d.stack.Toggle(c.GroupKey)
			d.drawLogged()
			return
		}
		d.svc.Dismiss(c.Notification.ID)
		return
	}
}

// setExpanded pliega o despliega una pila y repinta.
func (d *daemon) setExpanded(key string, open bool) {
	if key == "" {
		return
	}
	if open {
		d.stack.Expand(key)
	} else {
		d.stack.Collapse(key)
	}
	d.drawLogged()
}

// dismissAll cierra todas las notificaciones de una tarjeta.
func (d *daemon) dismissAll(c notify.Card) {
	ids := c.IDs
	if len(ids) == 0 {
		ids = []uint32{c.Notification.ID}
	}
	for _, id := range ids {
		d.svc.Dismiss(id)
	}
	if c.GroupKey != "" {
		d.stack.Collapse(c.GroupKey)
	}
	d.drawLogged()
}
