package notify

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/dbus"
)

// fakeBus levanta un socket que hace el handshake SASL y cede el control a
// serve, que a su vez atiende una conexión dbus real del lado servidor.
func fakeBus(t *testing.T, serve func(br *bufio.Reader, nc net.Conn)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bus")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		defer nc.Close()
		br := bufio.NewReader(nc)
		if _, err := br.ReadByte(); err != nil {
			return
		}
		if _, err := br.ReadString('\n'); err != nil {
			return
		}
		fmt.Fprint(nc, "OK deadbeef\r\n")
		if line, err := br.ReadString('\n'); err != nil || strings.TrimSpace(line) != "BEGIN" {
			return
		}
		serve(br, nc)
	}()
	return "unix:path=" + path
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (b *bufferedConn) Read(p []byte) (int, error) { return b.r.Read(p) }

// TestServiceEndToEnd monta un bus mínimo sobre un socket UNIX: el servicio
// atiende del otro lado y un cliente real le manda notificaciones.
func TestServiceEndToEnd(t *testing.T) {
	store := NewStore(Options{Timeout: time.Hour, MaxVisible: 4})
	store.SetNow(func() time.Time { return at(0) })
	svc := NewService(store)

	path := filepath.Join(t.TempDir(), "bus")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		defer nc.Close()
		br := bufio.NewReader(nc)
		if _, err := br.ReadByte(); err != nil {
			return
		}
		if _, err := br.ReadString('\n'); err != nil {
			return
		}
		fmt.Fprint(nc, "OK deadbeef\r\n")
		if _, err := br.ReadString('\n'); err != nil {
			return
		}
		srv := dbus.WrapConn(&bufferedConn{Conn: nc, r: br})
		svc.Attach(srv)
		srv.SetMethodHandler(func(m *dbus.Message) {
			// El bus responde al saludo y enruta el resto al servicio.
			if m.Member == "Hello" {
				_ = srv.Reply(m, "s", ":1.1")
				return
			}
			svc.Handle(m)
		})
		// Se mantiene la conexión viva mientras dure la prueba.
		time.Sleep(5 * time.Second)
	}()

	c, err := dialClient("unix:path=" + path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.conn.Close()

	id, err := c.Send(Notification{App: "foot", Summary: "Terminado", Body: "Compilado"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if id == 0 {
		t.Fatal("el servicio no devolvió id")
	}
	// El servidor debe tenerla viva.
	deadline := time.Now().Add(2 * time.Second)
	for len(store.Visible()) != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	vis := store.Visible()
	if len(vis) != 1 || vis[0].Summary != "Terminado" {
		t.Fatalf("visibles: %+v", vis)
	}
	if vis[0].Body != "Compilado" || vis[0].App != "foot" {
		t.Fatalf("datos: %+v", vis[0])
	}

	// La interfaz propia responde.
	reply, err := c.Call("ToggleDND", "")
	if err != nil {
		t.Fatalf("toggle dnd: %v", err)
	}
	if b, _ := reply.Body[0].(bool); !b {
		t.Errorf("se esperaba DND activado: %#v", reply.Body)
	}

	reply, err = c.Call("GetStatus", "")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	st, _ := reply.Body[0].(string)
	if !strings.Contains(st, `"dnd": true`) {
		t.Errorf("estado: %s", st)
	}
}

func TestApplyHints(t *testing.T) {
	h := dbus.Dict{
		{Key: "urgency", Value: dbus.Variant{Sig: "y", Value: uint8(2)}},
		{Key: "category", Value: dbus.Variant{Sig: "s", Value: "transfer"}},
		{Key: "desktop-entry", Value: dbus.Variant{Sig: "s", Value: "firefox"}},
		{Key: "transient", Value: dbus.Variant{Sig: "b", Value: true}},
		{Key: "resident", Value: dbus.Variant{Sig: "b", Value: true}},
		{Key: "progress", Value: dbus.Variant{Sig: "i", Value: int32(37)}},
	}
	var n Notification
	applyHints(&n, h)
	if n.Urgency != Critical {
		t.Errorf("urgencia: %v", n.Urgency)
	}
	if n.Category != "transfer" || n.Desktop != "firefox" {
		t.Errorf("categoría o escritorio: %+v", n)
	}
	if !n.Transient || !n.Resident {
		t.Errorf("transient/resident: %+v", n)
	}
	if n.Progress != 37 {
		t.Errorf("progreso: %d", n.Progress)
	}
}

func TestUrgencyParsing(t *testing.T) {
	cases := []struct {
		in   any
		want Urgency
	}{
		{uint8(0), Low}, {uint8(1), Normal}, {uint8(2), Critical},
		{int32(0), Low}, {int32(9), Critical}, {int64(1), Normal},
		{"raro", Normal}, {nil, Normal},
	}
	for _, c := range cases {
		if got := ParseUrgency(c.in); got != c.want {
			t.Errorf("ParseUrgency(%#v) = %v; esperado %v", c.in, got, c.want)
		}
	}
}

func TestStatusAndExpiry(t *testing.T) {
	store := NewStore(Options{Timeout: time.Second, MaxVisible: 3})
	now := at(0)
	store.SetNow(func() time.Time { return now })
	svc := NewService(store)

	a := store.Add(Notification{Summary: "uno", Timeout: 500})
	store.Add(Notification{Summary: "dos"})
	if n := len(store.Visible()); n != 2 {
		t.Fatalf("visibles: %d", n)
	}
	st := string(svc.StatusJSON())
	if !strings.Contains(st, `"pending": 2`) {
		t.Errorf("estado sin pendientes: %s", st)
	}

	now = at(0.6)
	if n := svc.Expire(); n != 1 {
		t.Fatalf("caducadas: %d", n)
	}
	if _, ok := store.Get(a.ID); ok {
		t.Error("la caducada no debería seguir viva")
	}
	h := store.History()
	if len(h) != 1 || h[0].Reason != "expired" {
		t.Fatalf("historial: %+v", h)
	}
}

func TestInvokeActionCloses(t *testing.T) {
	store := NewStore(Options{Timeout: time.Hour})
	store.SetNow(func() time.Time { return at(0) })
	svc := NewService(store)
	var gotKey string
	svc.OnAction = func(n Notification, key string) { gotKey = key }

	n := store.Add(Notification{Summary: "con acción", Actions: []Action{{Key: "default", Label: "Abrir"}}})
	svc.InvokeAction(n.ID, "default")
	if gotKey != "default" {
		t.Errorf("acción recibida: %q", gotKey)
	}
	if _, ok := store.Get(n.ID); ok {
		t.Error("la notificación no residente debe cerrarse al actuar")
	}

	res := store.Add(Notification{Summary: "residente", Resident: true,
		Actions: []Action{{Key: "x", Label: "X"}}})
	svc.InvokeAction(res.ID, "x")
	if _, ok := store.Get(res.ID); !ok {
		t.Error("la residente debe seguir viva")
	}
}

func TestClearAll(t *testing.T) {
	store := NewStore(Options{Timeout: time.Hour})
	store.SetNow(func() time.Time { return at(0) })
	svc := NewService(store)
	store.Add(Notification{Summary: "a"})
	store.Add(Notification{Summary: "b"})
	if n := svc.ClearAll(); n != 2 {
		t.Fatalf("cerradas: %d", n)
	}
	if len(store.Visible()) != 0 {
		t.Error("deberían estar todas cerradas")
	}
	if len(store.History()) != 2 {
		t.Errorf("historial: %d", len(store.History()))
	}
}

func TestDismissUnknown(t *testing.T) {
	store := NewStore(Options{})
	svc := NewService(store)
	if svc.Dismiss(999) {
		t.Error("cerrar una inexistente debe devolver false")
	}
}

// ─── Pruebas del cliente contra un bus de mentira ───────────────────────────

func TestClientSendEncodesNotify(t *testing.T) {
	got := make(chan *dbus.Message, 1)
	addr := fakeBus(t, func(br *bufio.Reader, nc net.Conn) {
		hello, err := decodeOne(t, br)
		if err != nil {
			t.Errorf("decode del saludo: %v", err)
			return
		}
		replyHello := &dbus.Message{Type: dbus.MethodReturn, ReplySerial: hello.Serial,
			Signature: "s", Body: []any{":1.7"}}
		replyHello.Serial = 1
		if data, err := replyHello.Encode(1); err == nil {
			_, _ = nc.Write(data)
		}
		msg, err := decodeOne(t, br)
		if err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		got <- msg
		reply := &dbus.Message{
			Type: dbus.MethodReturn, ReplySerial: msg.Serial, Signature: "u",
			Body: []any{uint32(77)},
		}
		reply.Serial = 2
		data, _ := reply.Encode(2)
		_, _ = nc.Write(data)
	})

	c, err := dialClient(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.conn.Close()

	id, err := c.Send(Notification{
		App: "swaydesk", Summary: "Hola", Body: "Mundo", Urgency: Critical,
		Actions: []Action{{Key: "default", Label: "Abrir"}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if id != 77 {
		t.Fatalf("id: %d", id)
	}
	msg := <-got
	if msg.Member != "Notify" || msg.Interface != Iface {
		t.Fatalf("llamada: %s.%s", msg.Interface, msg.Member)
	}
	if msg.Signature != "susssasa{sv}i" {
		t.Fatalf("firma: %q", msg.Signature)
	}
	if msg.Body[0] != "swaydesk" || msg.Body[3] != "Hola" || msg.Body[4] != "Mundo" {
		t.Fatalf("cuerpo: %#v", msg.Body)
	}
	acts, _ := msg.Body[5].([]any)
	if len(acts) != 2 || acts[0] != "default" || acts[1] != "Abrir" {
		t.Fatalf("acciones: %#v", msg.Body[5])
	}
	hints, ok := msg.Body[6].(dbus.Dict)
	if !ok {
		t.Fatalf("hints: %T", msg.Body[6])
	}
	if u := hints.Value("urgency"); u == nil {
		t.Fatalf("falta urgency: %#v", hints)
	} else if v, _ := toUint(u); v != 1 {
		t.Errorf("urgency debería ser 1 (normal): %v", u)
	}
}

func dialClient(addr string) (*Client, error) {
	c, err := dbus.Dial(addr)
	if err != nil {
		return nil, err
	}
	if err := c.Hello(); err != nil {
		c.Close()
		return nil, err
	}
	return &Client{conn: c}, nil
}

func decodeOne(t *testing.T, br *bufio.Reader) (*dbus.Message, error) {
	t.Helper()
	header := make([]byte, 16)
	if _, err := io.ReadFull(br, header); err != nil {
		return nil, err
	}
	bodyLen := int(binary.LittleEndian.Uint32(header[4:]))
	fieldsLen := int(binary.LittleEndian.Uint32(header[12:]))
	align := func(n int) int { return (n + 7) &^ 7 }
	total := align(align(16+fieldsLen) + bodyLen)
	msg := make([]byte, total)
	copy(msg, header)
	if _, err := io.ReadFull(br, msg[16:]); err != nil {
		return nil, err
	}
	return dbus.DecodeMessage(msg)
}

// Contexto de las pruebas de red: se comprueba que un tiempo agotado no
// bloquea el cliente.
func TestClientTimeout(t *testing.T) {
	addr := fakeBus(t, func(br *bufio.Reader, nc net.Conn) {
		// Contesta al saludo y después no responde nunca.
		msg, err := decodeOne(t, br)
		if err != nil {
			return
		}
		reply := &dbus.Message{Type: dbus.MethodReturn, ReplySerial: msg.Serial,
			Signature: "s", Body: []any{":1.9"}}
		data, _ := reply.Encode(2)
		_, _ = nc.Write(data)
		time.Sleep(1500 * time.Millisecond)
	})
	c, err := dialClient(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = c.conn.Call(ctx, Iface, ObjPath, Iface, "GetCapabilities", "")
	if err == nil {
		t.Fatal("se esperaba un tiempo agotado")
	}
}

// TestInlineReplyEndToEnd comprueba el ciclo completo de una respuesta rápida:
// la aplicación envía la acción inline-reply, el daemon responde y la
// aplicación recibe NotificationReplied solo para ella.
func TestInlineReplyEndToEnd(t *testing.T) {
	store := NewStore(Options{Timeout: time.Hour, MaxVisible: 4})
	store.SetNow(func() time.Time { return at(0) })
	svc := NewService(store)

	path := filepath.Join(t.TempDir(), "bus")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		defer nc.Close()
		br := bufio.NewReader(nc)
		if _, err := br.ReadByte(); err != nil {
			return
		}
		if _, err := br.ReadString('\n'); err != nil {
			return
		}
		fmt.Fprint(nc, "OK cafebabe\r\n")
		if _, err := br.ReadString('\n'); err != nil {
			return
		}
		srv := dbus.WrapConn(&bufferedConn{Conn: nc, r: br})
		svc.Attach(srv)
		srv.SetMethodHandler(func(m *dbus.Message) {
			if m.Member == "Hello" {
				_ = srv.Reply(m, "s", ":1.9")
				return
			}
			// El bus real rellena el remitente; el de prueba hace lo mismo.
			m.Sender = ":1.9"
			svc.Handle(m)
		})
		time.Sleep(5 * time.Second)
	}()

	c, err := dialClient("unix:path=" + path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.conn.Close()

	replies := make(chan *dbus.Message, 4)
	c.OnSignal(func(m *dbus.Message) {
		if m.Member == "NotificationReplied" {
			replies <- m
		}
	})

	// Una aplicación de chat pide respuesta rápida.
	id, err := c.Send(Notification{
		App: "Telegram", Summary: "Nuevo mensaje", Body: "¿Vienes?",
		Actions:     []Action{{Key: InlineReplyAction, Label: "Responder"}},
		InlineReply: true,
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(store.Visible()) != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	n, ok := store.Get(id)
	if !ok {
		t.Fatal("la notificación no llegó al almacén")
	}
	if !n.InlineReply {
		t.Fatal("debería admitir respuesta rápida")
	}
	if n.ReplyPlaceholder != "Responder" {
		t.Errorf("texto de ayuda: %q", n.ReplyPlaceholder)
	}
	if len(n.Actions) != 0 {
		t.Errorf("la acción inline-reply no debe dibujarse como botón: %+v", n.Actions)
	}
	if n.Sender != ":1.9" {
		t.Errorf("remitente: %q", n.Sender)
	}

	// El daemon (aquí el propio servicio) entrega la respuesta.
	if err := svc.Reply(id, "  ¡Allá voy!  "); err != nil {
		t.Fatalf("reply: %v", err)
	}
	select {
	case sig := <-replies:
		if sig.Signature != "us" || len(sig.Body) < 2 {
			t.Fatalf("señal: %s %#v", sig.Signature, sig.Body)
		}
		if got, _ := asUint(sig.Body[0]); got != id {
			t.Errorf("id en la señal: %d", got)
		}
		if text, _ := sig.Body[1].(string); text != "¡Allá voy!" {
			t.Errorf("texto: %q", text)
		}
		if sig.Destination != ":1.9" {
			t.Errorf("la respuesta debe ir solo al remitente, va a %q", sig.Destination)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó NotificationReplied")
	}

	// Y la notificación se cierra (no era residente).
	if _, ok := store.Get(id); ok {
		t.Error("tras responder debe cerrarse")
	}
	if err := svc.Reply(id, "otra vez"); err == nil {
		t.Error("responder a una cerrada debe fallar")
	}
}

func TestReplyValidation(t *testing.T) {
	store := NewStore(Options{Timeout: time.Hour})
	store.SetNow(func() time.Time { return at(0) })
	svc := NewService(store)
	sinRespuesta := store.Add(Notification{Summary: "sin respuesta", Progress: -1})
	if err := svc.Reply(sinRespuesta.ID, "hola"); err == nil {
		t.Error("una notificación sin inline-reply no admite respuesta")
	}

	var got string
	svc.OnReply = func(n Notification, text string) { got = text }
	conRespuesta := store.Add(Notification{Summary: "chat", InlineReply: true,
		Resident: true, Progress: -1, Sender: ""})
	if err := svc.Reply(conRespuesta.ID, ""); err == nil {
		t.Error("una respuesta vacía debe rechazarse")
	}
	if err := svc.Reply(conRespuesta.ID, "vale"); err != nil {
		t.Fatalf("reply: %v", err)
	}
	if got != "vale" {
		t.Errorf("callback: %q", got)
	}
	// Residente: sigue viva tras responder.
	if _, ok := store.Get(conRespuesta.ID); !ok {
		t.Error("una residente debe seguir viva")
	}
	if err := svc.Reply(999, "x"); err == nil {
		t.Error("un id inexistente debe fallar")
	}
}

// asUint normaliza los enteros que llegan por D-Bus.
func asUint(v any) (uint32, bool) {
	if n, ok := toUint(v); ok {
		return uint32(n), true
	}
	return 0, false
}

func TestCapabilitiesIncludeInlineReply(t *testing.T) {
	found := false
	for _, c := range Capabilities {
		if c == "inline-reply" {
			found = true
		}
	}
	if !found {
		t.Fatal("GetCapabilities debe anunciar inline-reply")
	}
	if len(Capabilities) == 0 {
		t.Fatal("sin capacidades")
	}
}
