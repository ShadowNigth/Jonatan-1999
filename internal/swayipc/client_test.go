package swayipc

import (
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

func TestSwayIPCRoundTrip(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "sway-ipc.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("Listen unix failed: %v", err)
	}
	defer ln.Close()

	var receivedCmds []string
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				hdr := make([]byte, 14)
				if _, err := io.ReadFull(c, hdr); err != nil {
					return
				}
				n := binary.LittleEndian.Uint32(hdr[6:10])
				mType := binary.LittleEndian.Uint32(hdr[10:14])
				body := make([]byte, n)
				if n > 0 {
					_, _ = io.ReadFull(c, body)
				}

				var reply string
				if mType == MsgGetVersion {
					reply = `{"human_readable":"swayfx version 0.4 (based on sway 1.9)","variant":"swayfx"}`
				} else {
					receivedCmds = append(receivedCmds, string(body))
					reply = `[{"success":true}]`
				}
				outHdr := make([]byte, 14)
				copy(outHdr[0:6], magic)
				binary.LittleEndian.PutUint32(outHdr[6:10], uint32(len(reply)))
				binary.LittleEndian.PutUint32(outHdr[10:14], mType)
				_, _ = c.Write(outHdr)
				_, _ = c.Write([]byte(reply))
			}(conn)
		}
	}()

	cli := New(sock)
	if !cli.Available() {
		t.Fatalf("expected Available() == true")
	}

	ver, isFX, err := cli.Version()
	if err != nil || !isFX {
		t.Fatalf("expected SwayFX version detection, got ver=%q isFX=%v err=%v", ver, isFX, err)
	}

	store := config.NewStore("")
	s := store.Settings()
	s.Compositor = "swayfx"
	if err := cli.ApplyLiveState(store.ActiveTheme(), s); err != nil {
		t.Fatalf("ApplyLiveState failed: %v", err)
	}
	if len(receivedCmds) == 0 || !strings.Contains(receivedCmds[0], "corner_radius") {
		t.Fatalf("expected corner_radius in SwayFX IPC command, got %v", receivedCmds)
	}
}
