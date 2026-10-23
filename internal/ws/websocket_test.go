package ws

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestComputeAcceptKey(t *testing.T) {
	// RFC 6455 Section 4.2.2 test vector
	got := computeAcceptKey("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("computeAcceptKey = %q, want %q", got, want)
	}
}

func TestUpgradeAndEcho(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrade(w, r)
		if err != nil {
			t.Errorf("Upgrade error: %v", err)
			return
		}
		defer c.Close()

		msg, err := c.ReadMessage()
		if err != nil {
			t.Errorf("ReadMessage error: %v", err)
			return
		}
		if err := c.WriteText("echo:" + string(msg)); err != nil {
			t.Errorf("WriteText error: %v", err)
		}
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	req := "GET /ws HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: keep-alive, Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("Write handshake failed: %v", err)
	}

	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		t.Fatalf("Expected 101 status, got %q (err=%v)", status, err)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("Read header err: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	// Send masked client frame: "kitsune"
	payload := []byte("kitsune")
	mask := [4]byte{0x12, 0x34, 0x56, 0x78}
	frame := []byte{0x81, 0x80 | byte(len(payload)), mask[0], mask[1], mask[2], mask[3]}
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("Write frame failed: %v", err)
	}

	hdr := make([]byte, 2)
	if _, err := br.Read(hdr); err != nil {
		t.Fatalf("Read server frame header failed: %v", err)
	}
	n := int(hdr[1] & 0x7F)
	reply := make([]byte, n)
	if _, err := br.Read(reply); err != nil {
		t.Fatalf("Read server payload failed: %v", err)
	}
	if string(reply) != "echo:kitsune" {
		t.Fatalf("Got reply %q, want %q", string(reply), "echo:kitsune")
	}
}
