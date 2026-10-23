package swayipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeSway simula el servidor IPC de sway para las pruebas.
type fakeSway struct {
	ln       net.Listener
	mu       sync.Mutex
	received []struct {
		T       MessageType
		Payload string
	}
	events  chan []byte
	replies map[MessageType]func(payload string) string
}

func newFakeSway(t *testing.T) (*fakeSway, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sway-ipc-test.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeSway{ln: ln, events: make(chan []byte, 16), replies: map[MessageType]func(string) string{}}
	f.replies[MsgGetWorkspaces] = func(string) string {
		return `[{"num":1,"name":"1:web","visible":true,"focused":true,"urgent":false,"output":"HDMI-A-1"}]`
	}
	f.replies[MsgGetOutputs] = func(string) string {
		return `[{"name":"HDMI-A-1","active":true,"current_workspace":"1:web","rect":{"width":1920,"height":1080}}]`
	}
	f.replies[MsgRunCommand] = func(p string) string {
		if p == "fail me" {
			return `[{"success":false,"error":"Unknown/invalid command 'fail me'","parse_error":true}]`
		}
		return `[{"success":true}]`
	}
	f.replies[MsgGetVersion] = func(string) string {
		return `{"major":1,"minor":10,"patch":1,"human_readable":"1.10.1","loaded_config_file_name":"/home/u/.config/sway/config"}`
	}
	f.replies[MsgGetTree] = func(string) string {
		return `{"id":1,"type":"root","nodes":[{"id":2,"type":"output","name":"HDMI-A-1","nodes":[
			{"id":3,"type":"workspace","name":"1:web","nodes":[
				{"id":4,"type":"con","name":"Firefox","app_id":"firefox","focused":true,"window":5,
				 "nodes":[{"id":6,"type":"con","name":"Hija"}]}
			]}]}]}`
	}
	f.replies[MsgGetInputs] = func(string) string {
		return `[{"identifier":"1234:5678:Keyboard","name":"Keyboard","type":"keyboard","xkb_active_layout_name":"Spanish (Latin American)"}]`
	}
	f.replies[MsgSendTick] = func(string) string { return "true" }
	f.replies[MsgSubscribe] = func(string) string { return "true" }
	f.replies[MsgSync] = func(string) string { return `{"success":true}` }
	go f.serve()
	t.Cleanup(func() { ln.Close() })
	return f, path
}

func (f *fakeSway) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeSway) handle(conn net.Conn) {
	defer conn.Close()
	for {
		payload, mt, err := readFrame(conn)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.received = append(f.received, struct {
			T       MessageType
			Payload string
		}{mt, string(payload)})
		handler, ok := f.replies[mt]
		f.mu.Unlock()
		if mt == MsgSubscribe {
			// Confirmar y acto seguido emitir los eventos registrados.
			_ = writeFrame(conn, mt, []byte(handler(string(payload))))
			for {
				select {
				case ev := <-f.events:
					if writeFrame(conn, EventWorkspace, ev) != nil {
						return
					}
				case <-time.After(50 * time.Millisecond):
					return
				}
			}
		}
		body := "[]"
		if ok {
			body = handler(string(payload))
		}
		if err := writeFrame(conn, mt, []byte(body)); err != nil {
			return
		}
	}
}

func TestSwaySockFromEnv(t *testing.T) {
	t.Setenv("SWAYSOCK", "/run/user/1000/sway-ipc.sock")
	if got := SwaySock(); got != "/run/user/1000/sway-ipc.sock" {
		t.Fatalf("SwaySock() = %q", got)
	}
	os.Unsetenv("SWAYSOCK")
	t.Setenv("I3SOCK", "/tmp/i3.sock")
	if got := SwaySock(); got != "/tmp/i3.sock" {
		t.Fatalf("SwaySock() con I3SOCK = %q", got)
	}
}

func TestDialSinSocket(t *testing.T) {
	os.Unsetenv("SWAYSOCK")
	os.Unsetenv("I3SOCK")
	t.Setenv("PATH", t.TempDir()) // sin binario "sway" disponible
	if _, err := Dial(); !errors.Is(err, ErrNoSocket) {
		t.Fatalf("esperaba ErrNoSocket, obtuve %v", err)
	}
}

func TestProtocoloBasico(t *testing.T) {
	t.Setenv("SWAYSOCK", "")
	f, path := newFakeSway(t)
	t.Setenv("SWAYSOCK", path)

	c, err := Dial()
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ws, err := c.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces: %v", err)
	}
	if len(ws) != 1 || ws[0].Name != "1:web" || !ws[0].Focused {
		t.Fatalf("workspaces inesperados: %+v", ws)
	}

	outs, err := c.Outputs()
	if err != nil || len(outs) != 1 || outs[0].Rect.Width != 1920 {
		t.Fatalf("salidas inesperadas: %+v (%v)", outs, err)
	}

	v, err := c.Version()
	if err != nil || v.Minor != 10 {
		t.Fatalf("versión inesperada: %+v (%v)", v, err)
	}

	in, err := c.Inputs()
	if err != nil || len(in) != 1 || in[0].XkbLayout == "" {
		t.Fatalf("inputs inesperados: %+v (%v)", in, err)
	}

	ok, err := c.SendTick("hola")
	if err != nil || !ok {
		t.Fatalf("SendTick: %v %v", ok, err)
	}

	// Los mensajes sin payload deben llegar como "[]".
	_ = c.Sync()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.received {
		if r.T == MsgGetTree && r.Payload != "[]" {
			t.Fatalf("GET_TREE envió payload %q, esperaba []", r.Payload)
		}
	}
}

func TestCommand(t *testing.T) {
	_, path := newFakeSway(t)
	c, err := DialPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	replies, err := c.Command("workspace 2")
	if err != nil || len(replies) != 1 || !replies[0].Success {
		t.Fatalf("Command: %+v %v", replies, err)
	}
	if err := c.CommandOK("fail me"); err == nil {
		t.Fatal("CommandOK debía fallar con un comando inválido")
	}
	if err := c.CommandOK("workspace 2", "layout tabbed"); err != nil {
		t.Fatalf("CommandOK múltiple: %v", err)
	}
}

func TestTree(t *testing.T) {
	_, path := newFakeSway(t)
	c, err := DialPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	tree, err := c.Tree()
	if err != nil {
		t.Fatal(err)
	}
	leaves := tree.Leaves()
	if len(leaves) != 2 {
		t.Fatalf("esperaba 2 ventanas, obtuve %d", len(leaves))
	}
	focused := tree.FindFocused()
	if focused == nil || focused.ID != 4 || focused.App() != "firefox" {
		t.Fatalf("nodo enfocado inesperado: %+v", focused)
	}
	if focused.Title() != "Firefox" {
		t.Fatalf("título = %q", focused.Title())
	}
}

func TestSubscribeYEventos(t *testing.T) {
	f, path := newFakeSway(t)
	c, err := DialPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetTimeout(2 * time.Second)

	if err := c.Subscribe("workspace", "window"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	f.events <- []byte(`{"change":"focus","current":{"name":"2:term","num":2},"old":{"name":"1:web"}}`)

	ev, err := c.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ev.Type != EventWorkspace || ev.Change() != "focus" {
		t.Fatalf("evento inesperado: %s %s", ev.Type, ev.Change())
	}
	var we WorkspaceEvent
	if err := ev.Decode(&we); err != nil {
		t.Fatal(err)
	}
	if we.Current.Name != "2:term" {
		t.Fatalf("workspace del evento: %q", we.Current.Name)
	}

	f.mu.Lock()
	last := f.received[len(f.received)-1]
	f.mu.Unlock()
	if last.T != MsgSubscribe || last.Payload != `["workspace","window"]` {
		t.Fatalf("suscripción enviada: %s %s", last.T, last.Payload)
	}
}

func TestEncodePayload(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "[]"},
		{"workspace 2", `"workspace 2"`},
		{[]string{"a", "b"}, `["a","b"]`},
		{[]byte(`{"x":1}`), `{"x":1}`},
	}
	for _, c := range cases {
		got, err := encodePayload(c.in)
		if err != nil {
			t.Fatalf("%v: %v", c.in, err)
		}
		if string(got) != c.want {
			t.Fatalf("encodePayload(%v) = %s, esperaba %s", c.in, got, c.want)
		}
	}
}

func TestFrameRoundTrip(t *testing.T) {
	var buf []byte
	pr, pw := net.Pipe()
	go func() {
		_ = writeFrame(pw, MsgGetTree, []byte(`{"a":1}`))
		pw.Close()
	}()
	payload, mt, err := readFrame(pr)
	if err != nil {
		t.Fatal(err)
	}
	buf = payload
	if mt != MsgGetTree || string(buf) != `{"a":1}` {
		t.Fatalf("frame inesperado: %s %s", mt, buf)
	}
}

func TestErrBadMagic(t *testing.T) {
	pr, pw := net.Pipe()
	go func() {
		_, _ = pw.Write([]byte("XXXXXX\x00\x00\x00\x00\x00\x00\x00\x00"))
		pw.Close()
	}()
	if _, _, err := readFrame(pr); !errors.Is(err, ErrBadMagic) {
		t.Fatalf("esperaba ErrBadMagic, obtuve %v", err)
	}
}
