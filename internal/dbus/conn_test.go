package dbus

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeBus levanta un socket UNIX que hace el handshake y delega el resto en
// serve. Devuelve la dirección para Dial.
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
		// NUL inicial + AUTH EXTERNAL.
		if _, err := br.ReadByte(); err != nil {
			return
		}
		line, err := br.ReadString('\n')
		if err != nil || !strings.HasPrefix(line, "AUTH EXTERNAL") {
			t.Errorf("autenticación inesperada: %q (%v)", line, err)
			return
		}
		fmt.Fprint(nc, "OK 1234deadbeef5678\r\n")
		if line, err = br.ReadString('\n'); err != nil || strings.TrimSpace(line) != "BEGIN" {
			t.Errorf("se esperaba BEGIN, llegó %q", line)
			return
		}
		serve(br, nc)
	}()
	return "unix:path=" + path
}

func writeMessage(t *testing.T, nc net.Conn, m *Message, serial uint32) {
	t.Helper()
	m.Serial = serial
	data, err := m.Encode(serial)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := nc.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestReadLoopHandlesUnpaddedBackToBackMessages(t *testing.T) {
	client, server := net.Pipe()
	conn := WrapConn(client)
	t.Cleanup(func() {
		_ = conn.Close()
		_ = server.Close()
	})

	received := make(chan *Message, 2)
	conn.SetSignalHandler(func(m *Message) { received <- m })
	first, err := NewSignal("/test", "org.example.Test", "First", "s", "a").Encode(1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSignal("/test", "org.example.Test", "Second", "s", "b").Encode(2)
	if err != nil {
		t.Fatal(err)
	}
	stream := append(first, second...)
	writeDone := make(chan error, 1)
	go func() {
		_, err := server.Write(stream)
		writeDone <- err
	}()

	for i, want := range []string{"First", "Second"} {
		select {
		case m := <-received:
			if m.Member != want {
				t.Fatalf("mensaje %d: llegó %q, se esperaba %q", i+1, m.Member, want)
			}
			if len(m.Body) != 1 || m.Body[0] != []string{"a", "b"}[i] {
				t.Fatalf("cuerpo del mensaje %q: %#v", want, m.Body)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no llegó el mensaje %q", want)
		}
	}
	if err := <-writeDone; err != nil {
		t.Fatalf("escritura del flujo: %v", err)
	}
}

func TestClientHello(t *testing.T) {
	addr := fakeBus(t, func(br *bufio.Reader, nc net.Conn) {
		msg, err := DecodeMessage(readMessage(t, br))
		if err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		if msg.Member != "Hello" || msg.Destination != BusName {
			t.Errorf("se esperaba Hello a %s, llegó %s → %s", BusName, msg.Destination, msg.Member)
		}
		writeMessage(t, nc, &Message{
			Type: MethodReturn, Destination: ":1.5", ReplySerial: msg.Serial,
			Signature: "s", Body: []any{":1.42"},
		}, 2)
	})

	c, err := Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if c.GUID != "1234deadbeef5678" {
		t.Errorf("GUID: %q", c.GUID)
	}
	if err := c.Hello(); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if c.Unique != ":1.42" {
		t.Fatalf("nombre único: %q", c.Unique)
	}
}

func TestClientCallError(t *testing.T) {
	addr := fakeBus(t, func(br *bufio.Reader, nc net.Conn) {
		msg, _ := DecodeMessage(readMessage(t, br))
		writeMessage(t, nc, &Message{
			Type: ErrorReply, Destination: ":1.5", ReplySerial: msg.Serial,
			ErrorName: "org.freedesktop.DBus.Error.UnknownMethod",
			Signature: "s", Body: []any{"no existe"},
		}, 2)
	})
	c, err := Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = c.Call(ctx, "org.freedesktop.Notifications", "/org/freedesktop/Notifications",
		"org.freedesktop.Notifications", "NoExiste", "")
	if err == nil || !strings.Contains(err.Error(), "UnknownMethod") {
		t.Fatalf("se esperaba error UnknownMethod, llegó %v", err)
	}
}

func TestServerRepliesToCall(t *testing.T) {
	done := make(chan *Message, 1)
	addr := fakeBus(t, func(br *bufio.Reader, nc net.Conn) {
		// El bus de pruebas actúa ahora como cliente.
		writeMessage(t, nc, NewCall(":1.9", "/org/freedesktop/Notifications",
			"org.freedesktop.Notifications", "GetCapabilities", ""), 10)
		reply, err := DecodeMessage(readMessage(t, br))
		if err != nil {
			t.Errorf("decode de la respuesta: %v", err)
			return
		}
		done <- reply
	})

	c, err := Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	c.SetMethodHandler(func(m *Message) {
		if m.Member != "GetCapabilities" {
			_ = c.ReplyError(m, "org.freedesktop.DBus.Error.UnknownMethod", "no")
			return
		}
		_ = c.Reply(m, "as", []string{"body", "actions"})
	})

	select {
	case reply := <-done:
		if reply.Type != MethodReturn || reply.ReplySerial != 10 {
			t.Fatalf("respuesta inesperada: %+v", reply)
		}
		if reply.Signature != "as" {
			t.Fatalf("firma: %q", reply.Signature)
		}
		arr, _ := reply.Body[0].([]any)
		if len(arr) != 2 || arr[0] != "body" {
			t.Fatalf("cuerpo: %#v", reply.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("el servidor no respondió")
	}
}

func TestRequestName(t *testing.T) {
	addr := fakeBus(t, func(br *bufio.Reader, nc net.Conn) {
		msg, err := DecodeMessage(readMessage(t, br))
		if err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		if msg.Member != "RequestName" || msg.Signature != "su" {
			t.Errorf("llamada inesperada: %s %q", msg.Member, msg.Signature)
		}
		name, _ := msg.Body[0].(string)
		if name != "org.freedesktop.Notifications" {
			t.Errorf("nombre: %q", name)
		}
		writeMessage(t, nc, &Message{
			Type: MethodReturn, ReplySerial: msg.Serial, Signature: "u", Body: []any{uint32(1)},
		}, 2)
	})
	c, err := Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	code, err := c.RequestName("org.freedesktop.Notifications", FlagNoAuto)
	if err != nil {
		t.Fatalf("request name: %v", err)
	}
	if code != 1 {
		t.Fatalf("código: %d", code)
	}
}

func TestParseAddress(t *testing.T) {
	cases := []struct{ in, network, target string }{
		{"unix:path=/run/user/1000/bus", "unix", "/run/user/1000/bus"},
		{"unix:abstract=/tmp/dbus-abc,guid=ff", "unix", "@/tmp/dbus-abc"},
		{"unix:path=/x;unix:path=/y", "unix", "/x"},
		{"tcp:host=127.0.0.1,port=1234", "tcp", "127.0.0.1:1234"},
	}
	for _, c := range cases {
		n, target, err := parseAddress(c.in)
		if err != nil {
			t.Errorf("parseAddress(%q): %v", c.in, err)
			continue
		}
		if n != c.network || target != c.target {
			t.Errorf("parseAddress(%q) = %q, %q", c.in, n, target)
		}
	}
	if _, _, err := parseAddress("launchd:env=X"); err == nil {
		t.Error("se esperaba error con una dirección desconocida")
	}
}
