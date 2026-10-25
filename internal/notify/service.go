package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/dbus"
)

// Constantes del protocolo de escritorio.
const (
	ObjPath       = "/org/freedesktop/Notifications"
	Iface         = "org.freedesktop.Notifications"
	OwnIface      = "org.swaydesk.Notifications"
	ServerName    = "swaynotify"
	ServerVendor  = "swaydesk"
	ServerVersion = "1.0"
	SpecVersion   = "1.2"

	// InlineReplyAction es la clave de la acción con la que las aplicaciones
	// piden una respuesta rápida (extensión de KDE adoptada por el ecosistema).
	InlineReplyAction = "inline-reply"
	// ReplyPlaceholderHint es el hint con el texto de ayuda del campo.
	ReplyPlaceholderHint = "x-kde-reply-placeholder-text"

	// ClosedReason es el motivo de cierre (según la especificación).
	ClosedExpired   = 1
	ClosedDismissed = 2
	ClosedCloseCall = 3
	ClosedUndefined = 4
)

// Service atiende org.freedesktop.Notifications sobre una conexión D-Bus.
type Service struct {
	Store *Store
	conn  *dbus.Conn

	mu    sync.Mutex
	dirty chan struct{}
	ready bool // la superficie Wayland ya mostró su primer fotograma
	// OnAction se llama cuando el usuario activa una acción (para abrir URLs).
	OnAction func(n Notification, key string)
	// OnClosed se llama al cerrar por petición del servidor.
	OnClosed func(id uint32)
	// OnReply se llama cuando llega una respuesta rápida.
	OnReply func(n Notification, text string)
}

// NewService crea el servicio sin conectar.
func NewService(store *Store) *Service {
	return &Service{Store: store, dirty: make(chan struct{}, 1)}
}

// Dirty devuelve un canal que recibe un aviso cuando cambia el estado.
func (s *Service) Dirty() <-chan struct{} { return s.dirty }

// Touch avisa al bucle de dibujo de que hay cambios.
func (s *Service) Touch() {
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}

// MarkReady indica que la superficie ya dibujó un fotograma y puede aceptar
// avisos sin que los clientes confundan el registro D-Bus con la disponibilidad
// visual del daemon.
func (s *Service) MarkReady() {
	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()
}

// Attach instala los manejadores de la conexión D-Bus.
func (s *Service) Attach(c *dbus.Conn) {
	s.conn = c
	c.SetMethodHandler(s.Handle)
}

// Register pide el nombre del servicio de notificaciones.
func (s *Service) Register(c *dbus.Conn) error {
	code, err := c.RequestName(Iface, dbus.FlagNoAuto)
	if err != nil {
		return err
	}
	switch code {
	case 1, 4:
		return nil
	case 3:
		return fmt.Errorf("ya hay otro servidor de notificaciones en el bus")
	}
	return fmt.Errorf("no se pudo tomar el nombre %s (código %d)", Iface, code)
}

// Handle atiende una llamada entrante.
func (s *Service) Handle(m *dbus.Message) {
	if s.conn == nil {
		return
	}
	switch m.Interface {
	case Iface:
		s.handleNotify(m)
	case OwnIface:
		s.handleOwn(m)
	default:
		_ = s.conn.ReplyError(m, "org.freedesktop.DBus.Error.UnknownInterface",
			"interfaz desconocida: "+m.Interface)
	}
}

func argString(m *dbus.Message, i int) string {
	if i < len(m.Body) {
		if v, ok := m.Body[i].(string); ok {
			return v
		}
	}
	return ""
}

func argUint(m *dbus.Message, i int) uint32 {
	if i < len(m.Body) {
		if n, ok := toUint(m.Body[i]); ok {
			return uint32(n)
		}
	}
	return 0
}

func argInt(m *dbus.Message, i int) int {
	if i < len(m.Body) {
		if n, ok := toUint(m.Body[i]); ok {
			return int(int32(uint32(n)))
		}
	}
	return 0
}

func toUint(v any) (uint64, bool) {
	switch n := v.(type) {
	case uint8:
		return uint64(n), true
	case int8:
		return uint64(n), true
	case uint16:
		return uint64(n), true
	case int16:
		return uint64(n), true
	case uint32:
		return uint64(n), true
	case int32:
		return uint64(n), true
	case int64:
		return uint64(n), true
	case uint64:
		return n, true
	case int:
		return uint64(n), true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func (s *Service) handleNotify(m *dbus.Message) {
	switch m.Member {
	case "GetCapabilities":
		_ = s.conn.Reply(m, "as", Capabilities)
	case "GetServerInformation":
		_ = s.conn.Reply(m, "ssss", ServerName, ServerVendor, ServerVersion, SpecVersion)
	case "Notify":
		n := Notification{
			App:        argString(m, 0),
			ReplacesID: argUint(m, 1),
			Icon:       argString(m, 2),
			Summary:    argString(m, 3),
			Body:       argString(m, 4),
			Timeout:    argInt(m, 7),
			Progress:   -1,
		}
		n.Sender = m.Sender
		if len(m.Body) > 5 {
			if acts, ok := m.Body[5].([]any); ok {
				for i := 0; i+1 < len(acts); i += 2 {
					key, _ := acts[i].(string)
					label, _ := acts[i+1].(string)
					if key == InlineReplyAction {
						// La etiqueta suele ser «Responder»; el texto se pide
						// aparte, así que no se dibuja como botón normal.
						n.InlineReply = true
						n.ReplyPlaceholder = label
						continue
					}
					n.Actions = append(n.Actions, Action{Key: key, Label: label})
				}
			}
		}
		if len(m.Body) > 6 {
			if hints, ok := m.Body[6].(dbus.Dict); ok {
				applyHints(&n, hints)
			}
		}
		if n.Summary == "" {
			n.Summary = n.Body
		}
		if n.Summary == "" {
			_ = s.conn.ReplyError(m, "org.freedesktop.Notifications.Error.Invalid",
				"la notificación necesita un resumen")
			return
		}
		loadImage := n.imageRaw != nil || n.imagePathForSend() != ""
		saved := s.Store.Add(n)
		s.Touch()
		if loadImage {
			// El texto aparece de inmediato; leer o decodificar una carátula o
			// imagen grande no debe bloquear la respuesta D-Bus ni el primer dibujo.
			go s.loadImage(saved, n)
		}
		_ = s.conn.Reply(m, "u", saved.ID)
	case "CloseNotification":
		id := argUint(m, 0)
		s.closeByServer(id, ClosedCloseCall)
	default:
		_ = s.conn.ReplyError(m, "org.freedesktop.DBus.Error.UnknownMethod",
			"método desconocido: "+m.Member)
	}
}

// loadImage resuelve la imagen fuera del camino D-Bus y la adjunta solo si la
// notificación no fue reemplazada ni cerrada mientras se decodificaba.
func (s *Service) loadImage(saved, source Notification) {
	resolveImage(&source)
	if source.Image != nil && s.Store.SetImage(saved.ID, saved.revision, source.Image) {
		s.Touch()
	}
}

// applyHints interpreta los hints estándar.
func applyHints(n *Notification, h dbus.Dict) {
	if v := h.Value("urgency"); v != nil {
		n.Urgency = ParseUrgency(v)
	}
	if s, ok := h.Value("category").(string); ok {
		n.Category = s
	}
	if s, ok := h.Value("desktop-entry").(string); ok {
		n.Desktop = s
	}
	if b, ok := h.Value("transient").(bool); ok {
		n.Transient = b
	}
	if b, ok := h.Value("resident").(bool); ok {
		n.Resident = b
	}
	if ph, ok := h.Value(ReplyPlaceholderHint).(string); ok && ph != "" {
		n.ReplyPlaceholder = ph
	}
	// Etiqueta de pila: dunst y KDE usan nombres distintos para lo mismo.
	for _, key := range []string{"x-dunst-stack-tag", "x-kde-group-id", "group", "x-swaydesk-group"} {
		if g, ok := h.Value(key).(string); ok && strings.TrimSpace(g) != "" {
			n.GroupID = strings.TrimSpace(g)
			break
		}
	}
	// Imagen: el blob (image-data) o la ruta (image-path). Se aceptan también
	// las variantes con guion bajo, que usan algunos clientes.
	for _, key := range []string{"image-data", "image_data"} {
		if v := h.Value(key); v != nil {
			if _, ok := v.([]any); ok {
				n.imageRaw = v
				break
			}
		}
	}
	for _, key := range []string{"image-path", "image_path"} {
		if p, ok := h.Value(key).(string); ok && strings.TrimSpace(p) != "" {
			n.ImagePath = strings.TrimSpace(p)
			break
		}
	}
	if p := h.Value("progress"); p != nil {
		if v, ok := toUint(p); ok {
			n.Progress = int(int32(uint32(v)))
		}
	}
}

// resolveImage busca la imagen de la notificación: primero los bytes del hint
// `image-data`, después un archivo (`image-path`, `image_path`) y por último el
// icono si resulta ser una ruta de archivo.
//
// Un fallo no invalida la notificación: como mucho, la tarjeta se dibuja sin
// imagen.
func resolveImage(n *Notification) {
	if n.Image != nil {
		return
	}
	if raw, ok := n.imageData(); ok {
		n.Image = raw
		return
	}
	path := n.ImagePath
	if path == "" && strings.ContainsAny(n.Icon, "/") && !strings.HasPrefix(n.Icon, "http") {
		path = n.Icon
	}
	if path == "" {
		return
	}
	im, err := LoadImageFile(path)
	if err != nil {
		return
	}
	n.Image = im
}

// imageData decodifica el blob que trae el hint `image-data`.
func (n *Notification) imageData() (*Image, bool) {
	if n.imageRaw == nil {
		return nil, false
	}
	fields, ok := n.imageRaw.([]any)
	if !ok || len(fields) < 7 {
		return nil, false
	}
	width, _ := toInt(fields[0])
	height, _ := toInt(fields[1])
	rowstride, _ := toInt(fields[2])
	hasAlpha, _ := fields[3].(bool)
	bits, _ := toInt(fields[4])
	channels, _ := toInt(fields[5])
	data, _ := fields[6].([]byte)
	im, err := DecodeImageData(width, height, rowstride, hasAlpha, bits, channels, data)
	if err != nil {
		return nil, false
	}
	return im, true
}

// toInt acepta los enteros que puede devolver el decodificador de D-Bus.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int32:
		return int(n), true
	case uint32:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint8:
		return int(n), true
	}
	return 0, false
}

// closeByServer cierra desde el servidor y avisa a los clientes.
func (s *Service) closeByServer(id uint32, reason uint32) {
	if !s.Store.Dismiss(id, reasonName(reason)) {
		return
	}
	s.emitClosed(id, reason)
	s.Touch()
	if s.OnClosed != nil && reason == ClosedDismissed {
		s.OnClosed(id)
	}
}

func reasonName(reason uint32) string {
	switch reason {
	case ClosedExpired:
		return "expired"
	case ClosedDismissed:
		return "dismissed"
	case ClosedCloseCall:
		return "closed"
	}
	return "undefined"
}

func (s *Service) emitClosed(id, reason uint32) {
	if s.conn == nil {
		return
	}
	_ = s.conn.Emit(ObjPath, Iface, "NotificationClosed", "uu", id, reason)
}

// Expire cierra las caducadas y avisa por D-Bus. Devuelve cuántas cerró.
func (s *Service) Expire() int {
	ids := s.Store.Expire()
	for _, id := range ids {
		s.emitClosed(id, ClosedExpired)
	}
	if len(ids) > 0 {
		s.Touch()
	}
	return len(ids)
}

// Dismiss cierra una notificación como si el usuario hiciera clic en ella.
func (s *Service) Dismiss(id uint32) bool {
	if !s.Store.Dismiss(id, "dismissed") {
		return false
	}
	s.emitClosed(id, ClosedDismissed)
	s.Touch()
	return true
}

// InvokeAction informa al cliente de que se activó una acción y cierra la
// notificación si no es residente.
func (s *Service) InvokeAction(id uint32, key string) {
	n, ok := s.Store.Get(id)
	if !ok {
		return
	}
	if s.conn != nil {
		_ = s.conn.Emit(ObjPath, Iface, "ActionInvoked", "us", id, key)
	}
	if s.OnAction != nil {
		s.OnAction(n, key)
	}
	if !n.Resident {
		s.closeByServer(id, ClosedDismissed)
	}
}

// Capabilities es la lista que se anuncia en GetCapabilities.
var Capabilities = []string{
	"body", "body-markup", "actions", "persistence", "icon-static", "inline-reply",
}

// Hold detiene la caducidad de una notificación y repinta.
func (s *Service) Hold(id uint32) bool {
	ok := s.Store.Hold(id)
	if ok {
		s.Touch()
	}
	return ok
}

// Reply entrega una respuesta rápida: emite NotificationReplied al cliente que
// envió la notificación (no a todo el bus, porque el texto puede ser privado)
// y cierra la notificación salvo que sea residente.
func (s *Service) Reply(id uint32, text string) error {
	n, ok := s.Store.Get(id)
	if !ok {
		return fmt.Errorf("la notificación %d ya no existe", id)
	}
	if !n.InlineReply {
		return fmt.Errorf("la notificación %d no admite respuesta", id)
	}
	text = SanitizeReply(text)
	if text == "" {
		return fmt.Errorf("la respuesta está vacía")
	}
	s.emit(n.Sender, "NotificationReplied", "us", id, text)
	if s.OnReply != nil {
		s.OnReply(n, text)
	}
	if !n.Resident {
		s.closeByServer(id, ClosedDismissed)
	} else {
		s.Touch()
	}
	return nil
}

// emit manda una señal al cliente concreto si se conoce su nombre único.
func (s *Service) emit(dest, member, sig string, args ...any) {
	if s.conn == nil {
		return
	}
	_ = s.conn.EmitTo(dest, ObjPath, Iface, member, sig, args...)
}

// VisibleJSON devuelve las notificaciones en pantalla serializadas.
func (s *Service) VisibleJSON() []byte {
	data, _ := json.Marshal(s.Store.Visible())
	return data
}

// ClearAll cierra todas las notificaciones.
func (s *Service) ClearAll() int {
	ids := s.Store.Clear()
	for _, id := range ids {
		s.emitClosed(id, ClosedDismissed)
	}
	if len(ids) > 0 {
		s.Touch()
	}
	return len(ids)
}

// ─── Interfaz propia (swaynotify) ───────────────────────────────────────────

func (s *Service) handleOwn(m *dbus.Message) {
	switch m.Member {
	case "SetDND":
		on := false
		if len(m.Body) > 0 {
			if b, ok := m.Body[0].(bool); ok {
				on = b
			}
		}
		s.Store.SetDND(on)
		s.Touch()
		_ = s.conn.Reply(m, "b", on)
	case "ToggleDND":
		on := s.Store.ToggleDND()
		s.Touch()
		_ = s.conn.Reply(m, "b", on)
	case "GetStatus":
		_ = s.conn.Reply(m, "s", string(s.StatusJSON()))
	case "ListHistory":
		recs := s.Store.History()
		data, _ := json.MarshalIndent(recs, "", "  ")
		_ = s.conn.Reply(m, "s", string(data))
	case "CloseAll":
		n := s.ClearAll()
		_ = s.conn.Reply(m, "u", uint32(n))
	case "ClearHistory":
		s.Store.ClearHistory()
		_ = s.conn.Reply(m, "b", true)
	case "Dismiss":
		ok := s.Dismiss(argUint(m, 0))
		_ = s.conn.Reply(m, "b", ok)
	case "ListVisible":
		_ = s.conn.Reply(m, "s", string(s.VisibleJSON()))
	case "Reply":
		id := argUint(m, 0)
		if len(m.Body) < 2 {
			_ = s.conn.ReplyError(m, "org.freedesktop.DBus.Error.InvalidArgs", "falta el texto")
			return
		}
		text, _ := m.Body[1].(string)
		if err := s.Reply(id, text); err != nil {
			_ = s.conn.ReplyError(m, "org.swaydesk.Notifications.Error.NoReply", err.Error())
			return
		}
		_ = s.conn.Reply(m, "b", true)
	default:
		_ = s.conn.ReplyError(m, "org.freedesktop.DBus.Error.UnknownMethod",
			"método desconocido: "+m.Member)
	}
}

// Status es el estado del daemon tal como lo devuelve GetStatus.
type Status struct {
	Running bool     `json:"running"`
	Ready   bool     `json:"ready"`
	DND     bool     `json:"dnd"`
	Pending int      `json:"pending"`
	Visible []uint32 `json:"visible"`
	History int      `json:"history"`
	// Replies indica cuántas notificaciones en pantalla admiten respuesta.
	Replies int    `json:"replies"`
	Owner   string `json:"owner"`
	Version string `json:"version"`
	BusName string `json:"bus_name"`
}

// StatusJSON devuelve el estado serializado.
func (s *Service) StatusJSON() []byte {
	s.mu.Lock()
	ready := s.ready
	s.mu.Unlock()
	vis := s.Store.Visible()
	ids := make([]uint32, 0, len(vis))
	for _, n := range vis {
		ids = append(ids, n.ID)
	}
	replies := 0
	for _, n := range vis {
		if n.InlineReply {
			replies++
		}
	}
	st := Status{
		Running: true, Ready: ready, DND: s.Store.DND(), Pending: s.Store.Pending(),
		Visible: ids, History: len(s.Store.History()), Replies: replies,
		Owner: ServerName, Version: ServerVersion, BusName: Iface,
	}
	data, _ := json.MarshalIndent(st, "", "  ")
	return data
}

// ─── Cliente ────────────────────────────────────────────────────────────────

// Client habla con un servidor de notificaciones por D-Bus.
type Client struct {
	conn *dbus.Conn
}

// DialClient abre el bus de sesión para mandar notificaciones.
func DialClient() (*Client, error) {
	c, err := dbus.DialSession()
	if err != nil {
		return nil, err
	}
	if err := c.Hello(); err != nil {
		c.Close()
		return nil, err
	}
	return &Client{conn: c}, nil
}

// Close cierra la conexión.
func (c *Client) Close() { _ = c.conn.Close() }

// OnSignal registra la función que recibe las señales del bus (por ejemplo
// NotificationReplied cuando este cliente es una aplicación de chat).
func (c *Client) OnSignal(fn func(*dbus.Message)) { c.conn.SetSignalHandler(fn) }

// Unique devuelve el nombre único de esta conexión en el bus.
func (c *Client) Unique() string { return c.conn.Unique }

// Send envía una notificación y devuelve su id.
func (c *Client) Send(n Notification) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	sig := "susssasa{sv}i"
	hints := dbus.Dict{}
	switch n.Urgency {
	case Low:
		hints = append(hints, dbus.DictEntry{Key: "urgency", Value: dbus.Variant{Sig: "y", Value: uint8(0)}})
	case Critical:
		hints = append(hints, dbus.DictEntry{Key: "urgency", Value: dbus.Variant{Sig: "y", Value: uint8(1)}})
	}
	if n.Category != "" {
		hints = append(hints, dbus.DictEntry{Key: "category", Value: dbus.Variant{Sig: "s", Value: n.Category}})
	}
	if n.Desktop != "" {
		hints = append(hints, dbus.DictEntry{Key: "desktop-entry", Value: dbus.Variant{Sig: "s", Value: n.Desktop}})
	}
	if n.Transient {
		hints = append(hints, dbus.DictEntry{Key: "transient", Value: dbus.Variant{Sig: "b", Value: true}})
	}
	if n.Resident {
		hints = append(hints, dbus.DictEntry{Key: "resident", Value: dbus.Variant{Sig: "b", Value: true}})
	}
	if n.Progress >= 0 {
		hints = append(hints, dbus.DictEntry{Key: "progress", Value: dbus.Variant{Sig: "i", Value: int32(n.Progress)}})
	}
	if n.GroupID != "" {
		hints = append(hints, dbus.DictEntry{Key: "x-swaydesk-group", Value: dbus.Variant{Sig: "s", Value: n.GroupID}})
	}
	// La imagen viaja como ruta: es el hint estándar y lo entienden también
	// mako y dunst.
	if path := n.imagePathForSend(); path != "" {
		hints = append(hints, dbus.DictEntry{Key: "image-path", Value: dbus.Variant{Sig: "s", Value: path}})
	}
	flat := make([]any, 0, len(n.Actions)*2)
	for _, a := range n.Actions {
		flat = append(flat, a.Key, a.Label)
	}
	// 0 pide el tiempo del servidor; -1 que no caduque nunca.
	timeout := int32(n.Timeout)
	reply, err := c.conn.Call(ctx, Iface, ObjPath, Iface, "Notify", sig,
		orDefault(n.App, "swaydesk"), uint32(0), n.Icon,
		n.Summary, n.Body, flat, hints, timeout)
	if err != nil {
		return 0, err
	}
	if len(reply.Body) > 0 {
		if v, ok := toUint(reply.Body[0]); ok {
			return uint32(v), nil
		}
	}
	return 0, nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// Call ejecuta un método de la interfaz propia (dnd, listas, etc.).
func (c *Client) Call(member, sig string, args ...any) (*dbus.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return c.conn.Call(ctx, Iface, ObjPath, OwnIface, member, sig, args...)
}

// Reply envía una respuesta rápida a una notificación.
func (c *Client) Reply(id uint32, text string) error {
	_, err := c.Call("Reply", "us", id, text)
	return err
}

// ListVisible pide las notificaciones en pantalla.
func (c *Client) ListVisible() ([]Notification, error) {
	reply, err := c.Call("ListVisible", "")
	if err != nil {
		return nil, err
	}
	if len(reply.Body) == 0 {
		return nil, nil
	}
	raw, _ := reply.Body[0].(string)
	var out []Notification
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("respuesta ilegible del daemon: %w", err)
	}
	return out, nil
}

// ListHistory pide el historial completo.
func (c *Client) ListHistory() ([]Record, error) {
	reply, err := c.Call("ListHistory", "")
	if err != nil {
		return nil, err
	}
	if len(reply.Body) == 0 {
		return nil, nil
	}
	raw, _ := reply.Body[0].(string)
	var out []Record
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("historial ilegible del daemon: %w", err)
	}
	return out, nil
}

// Status pide el estado del daemon ya decodificado.
func (c *Client) Status() (Status, error) {
	reply, err := c.Call("GetStatus", "")
	if err != nil {
		return Status{}, err
	}
	var st Status
	if len(reply.Body) > 0 {
		raw, _ := reply.Body[0].(string)
		if err := json.Unmarshal([]byte(raw), &st); err != nil {
			return st, err
		}
	}
	return st, nil
}

// Dismiss pide al daemon que cierre una notificación.
func (c *Client) Dismiss(id uint32) (bool, error) {
	reply, err := c.Call("Dismiss", "u", id)
	if err != nil {
		return false, err
	}
	ok := false
	if len(reply.Body) > 0 {
		ok, _ = reply.Body[0].(bool)
	}
	return ok, nil
}

// SetDND fija el modo «no molestar».
func (c *Client) SetDND(on bool) (bool, error) {
	reply, err := c.Call("SetDND", "b", on)
	if err != nil {
		return false, err
	}
	v := on
	if len(reply.Body) > 0 {
		if b, ok := reply.Body[0].(bool); ok {
			v = b
		}
	}
	return v, nil
}

// imagePathForSend devuelve la ruta que se manda en el hint image-path.
func (n *Notification) imagePathForSend() string {
	if n.ImagePath != "" {
		return n.ImagePath
	}
	// Un icono con forma de ruta también se anuncia como imagen.
	if strings.ContainsAny(n.Icon, "/") && !strings.HasPrefix(n.Icon, "http") {
		return n.Icon
	}
	return ""
}

// Available indica si el servidor ya dibujó su primer fotograma y puede mostrar avisos.
func (c *Client) Available() bool {
	status, err := c.Status()
	return err == nil && status.Running && status.Ready
}
