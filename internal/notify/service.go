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
	// OnAction se llama cuando el usuario activa una acción (para abrir URLs).
	OnAction func(n Notification, key string)
	// OnClosed se llama al cerrar por petición del servidor.
	OnClosed func(id uint32)
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
		_ = s.conn.Reply(m, "as", []string{"body", "body-markup", "actions", "persistence", "icon-static"})
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
		if len(m.Body) > 5 {
			if acts, ok := m.Body[5].([]any); ok {
				for i := 0; i+1 < len(acts); i += 2 {
					key, _ := acts[i].(string)
					label, _ := acts[i+1].(string)
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
		saved := s.Store.Add(n)
		s.Touch()
		_ = s.conn.Reply(m, "u", saved.ID)
	case "CloseNotification":
		id := argUint(m, 0)
		s.closeByServer(id, ClosedCloseCall)
	default:
		_ = s.conn.ReplyError(m, "org.freedesktop.DBus.Error.UnknownMethod",
			"método desconocido: "+m.Member)
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
	if p := h.Value("progress"); p != nil {
		if v, ok := toUint(p); ok {
			n.Progress = int(int32(uint32(v)))
		}
	}
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
	default:
		_ = s.conn.ReplyError(m, "org.freedesktop.DBus.Error.UnknownMethod",
			"método desconocido: "+m.Member)
	}
}

// Status es el estado del daemon tal como lo devuelve GetStatus.
type Status struct {
	Running bool     `json:"running"`
	DND     bool     `json:"dnd"`
	Pending int      `json:"pending"`
	Visible []uint32 `json:"visible"`
	History int      `json:"history"`
	Owner   string   `json:"owner"`
	Version string   `json:"version"`
	BusName string   `json:"bus_name"`
}

// StatusJSON devuelve el estado serializado.
func (s *Service) StatusJSON() []byte {
	vis := s.Store.Visible()
	ids := make([]uint32, 0, len(vis))
	for _, n := range vis {
		ids = append(ids, n.ID)
	}
	st := Status{
		Running: true, DND: s.Store.DND(), Pending: s.Store.Pending(),
		Visible: ids, History: len(s.Store.History()),
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

// Available indica si hay un servidor de notificaciones atendiendo.
func (c *Client) Available() bool {
	_, err := c.Call("GetStatus", "")
	return err == nil
}
