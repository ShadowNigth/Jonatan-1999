package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

// Conn represents an upgraded RFC 6455 WebSocket connection using Go's standard library.
type Conn struct {
	netConn net.Conn
	rw      *bufio.ReadWriter
	writeMu sync.Mutex
	closed  bool
}

// Upgrade performs an HTTP -> WebSocket handshake accepting any preview/browser origin.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if !headerContainsToken(r.Header, "Upgrade", "websocket") {
		http.Error(w, "missing websocket upgrade header", http.StatusBadRequest)
		return nil, errors.New("missing Upgrade: websocket")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return nil, errors.New("missing Sec-WebSocket-Key")
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijack unsupported", http.StatusInternalServerError)
		return nil, errors.New("http.Hijacker not supported")
	}

	netConn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	accept := computeAcceptKey(key)
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"

	if _, err := rw.WriteString(resp); err != nil {
		_ = netConn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = netConn.Close()
		return nil, err
	}

	_ = netConn.SetDeadline(time.Time{})
	return &Conn{
		netConn: netConn,
		rw:      rw,
	}, nil
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func headerContainsToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

// ReadMessage reads a complete text or binary message from the client.
func (c *Conn) ReadMessage() ([]byte, error) {
	var messageBuf []byte
	for {
		b0, err := c.rw.ReadByte()
		if err != nil {
			return nil, err
		}
		b1, err := c.rw.ReadByte()
		if err != nil {
			return nil, err
		}

		fin := (b0 & 0x80) != 0
		opcode := b0 & 0x0F
		masked := (b1 & 0x80) != 0
		payloadLen := uint64(b1 & 0x7F)

		switch payloadLen {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
				return nil, err
			}
			payloadLen = uint64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
				return nil, err
			}
			payloadLen = binary.BigEndian.Uint64(ext[:])
		}

		if payloadLen > 4*1024*1024 {
			return nil, errors.New("frame payload exceeds 4MB limit")
		}

		var maskKey [4]byte
		if masked {
			if _, err := io.ReadFull(c.rw, maskKey[:]); err != nil {
				return nil, err
			}
		}

		payload := make([]byte, payloadLen)
		if payloadLen > 0 {
			if _, err := io.ReadFull(c.rw, payload); err != nil {
				return nil, err
			}
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}

		switch opcode {
		case opClose:
			_ = c.writeFrame(opClose, nil)
			_ = c.Close()
			return nil, io.EOF
		case opPing:
			_ = c.writeFrame(opPong, payload)
			continue
		case opPong:
			continue
		case opText, opBinary, opContinuation:
			messageBuf = append(messageBuf, payload...)
			if fin {
				return messageBuf, nil
			}
		default:
			// Ignore unknown control frames
		}
	}
}

// WriteText sends a UTF-8 text frame to the peer.
func (c *Conn) WriteText(msg string) error {
	return c.writeFrame(opText, []byte(msg))
}

// WriteJSON marshals v to JSON and sends it as a text frame.
func (c *Conn) WriteJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.writeFrame(opText, data)
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return net.ErrClosed
	}

	_ = c.netConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	var header [10]byte
	header[0] = 0x80 | (opcode & 0x0F)
	hlen := 2
	n := len(payload)

	switch {
	case n < 126:
		header[1] = byte(n)
	case n <= 65535:
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:4], uint16(n))
		hlen = 4
	default:
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:10], uint64(n))
		hlen = 10
	}

	if _, err := c.rw.Write(header[:hlen]); err != nil {
		return err
	}
	if n > 0 {
		if _, err := c.rw.Write(payload); err != nil {
			return err
		}
	}
	return c.rw.Flush()
}

// Close closes the underlying TCP socket.
func (c *Conn) Close() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.netConn.Close()
}

// Event represents a server-to-desktop IPC event.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Hub maintains active desktop WebSocket subscribers for real-time state & telemetry sync.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Conn]struct{}),
	}
}

func (h *Hub) Register(c *Conn) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	_ = c.Close()
}

func (h *Hub) Broadcast(evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.RLock()
	conns := make([]*Conn, 0, len(h.clients))
	for c := range h.clients {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	msg := string(data)
	for _, c := range conns {
		if err := c.WriteText(msg); err != nil {
			h.Unregister(c)
		}
	}
}
