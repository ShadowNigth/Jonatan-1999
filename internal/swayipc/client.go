package swayipc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
	"github.com/ShadowNigth/Jonatan-1999/internal/ricer"
)

const magic = "i3-ipc"

const (
	MsgRunCommand    uint32 = 0
	MsgGetWorkspaces uint32 = 1
	MsgGetOutputs    uint32 = 3
	MsgGetTree       uint32 = 4
	MsgGetVersion    uint32 = 7
)

// Client speaks the native binary sway-ipc / i3-ipc protocol over $SWAYSOCK.
type Client struct {
	socketPath string
}

// NewFromEnv creates a Sway/SwayFX IPC client using $SWAYSOCK if available.
func NewFromEnv() *Client {
	return &Client{
		socketPath: strings.TrimSpace(os.Getenv("SWAYSOCK")),
	}
}

// New creates a Sway/SwayFX IPC client bound to a specific UNIX socket path.
func New(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// Available reports whether a live Sway or SwayFX UNIX socket is reachable.
func (c *Client) Available() bool {
	if c == nil || c.socketPath == "" {
		return false
	}
	info, err := os.Stat(c.socketPath)
	return err == nil && (info.Mode()&os.ModeSocket != 0)
}

func (c *Client) SocketPath() string {
	if c == nil {
		return ""
	}
	return c.socketPath
}

// Send transmits a raw sway-ipc frame and returns the JSON response payload.
func (c *Client) Send(msgType uint32, payload string) ([]byte, error) {
	if c == nil || c.socketPath == "" {
		return nil, errors.New("SWAYSOCK no está definido (fuera de una sesión nativa Sway/SwayFX)")
	}
	conn, err := net.DialTimeout("unix", c.socketPath, 1500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	body := []byte(payload)
	hdr := make([]byte, 14)
	copy(hdr[0:6], magic)
	binary.LittleEndian.PutUint32(hdr[6:10], uint32(len(body)))
	binary.LittleEndian.PutUint32(hdr[10:14], msgType)

	if _, err := conn.Write(hdr); err != nil {
		return nil, err
	}
	if len(body) > 0 {
		if _, err := conn.Write(body); err != nil {
			return nil, err
		}
	}

	respHdr := make([]byte, 14)
	if _, err := io.ReadFull(conn, respHdr); err != nil {
		return nil, err
	}
	if string(respHdr[0:6]) != magic {
		return nil, errors.New("cabecera sway-ipc inválida")
	}
	respLen := binary.LittleEndian.Uint32(respHdr[6:10])
	if respLen > 8*1024*1024 {
		return nil, errors.New("respuesta sway-ipc excede límite de 8MB")
	}
	respBody := make([]byte, respLen)
	if respLen > 0 {
		if _, err := io.ReadFull(conn, respBody); err != nil {
			return nil, err
		}
	}
	return respBody, nil
}

// RunCommand executes one or more semicolon-separated Sway/SwayFX commands.
func (c *Client) RunCommand(cmd string) ([]byte, error) {
	return c.Send(MsgRunCommand, cmd)
}

// Version queries GET_VERSION and detects whether SwayFX extensions are present.
func (c *Client) Version() (human string, isSwayFX bool, err error) {
	raw, err := c.Send(MsgGetVersion, "")
	if err != nil {
		return "", false, err
	}
	var parsed struct {
		HumanReadable string `json:"human_readable"`
		Variant       string `json:"variant"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return string(raw), false, nil
	}
	lower := strings.ToLower(parsed.HumanReadable + " " + parsed.Variant)
	return parsed.HumanReadable, strings.Contains(lower, "fx") || strings.Contains(lower, "swayfx"), nil
}

// ApplyLiveState pushes theme colors, gaps, borders, and SwayFX eye-candy to a running Sway/SwayFX compositor.
func (c *Client) ApplyLiveState(theme config.Theme, s config.DesktopSettings) error {
	if !c.Available() {
		return nil
	}
	cmds := []string{
		fmt.Sprintf("gaps inner all set %d", s.GapsIn),
		fmt.Sprintf("gaps outer all set %d", s.GapsOut),
		fmt.Sprintf("default_border pixel %d", s.BorderSize),
		fmt.Sprintf("default_floating_border pixel %d", s.BorderSize),
		fmt.Sprintf("client.focused %s %s %s %s %s", theme.Primary, theme.BgSurface, theme.TextMain, theme.Secondary, theme.Primary),
		fmt.Sprintf("client.unfocused %s %s %s %s %s", theme.BgOverlay, theme.BgBase, theme.TextMuted, theme.BgOverlay, theme.BgOverlay),
	}

	_, detectedFX, _ := c.Version()
	if s.Compositor == "swayfx" || detectedFX {
		blurState := "enable"
		if !s.BlurEnabled {
			blurState = "disable"
		}
		shadowState := "enable"
		if !s.BorderGlow {
			shadowState = "disable"
		}
		pHex := strings.TrimPrefix(theme.Primary, "#")
		cmds = append(cmds,
			fmt.Sprintf("corner_radius %d", s.BorderRadius),
			fmt.Sprintf("blur %s", blurState),
			fmt.Sprintf("shadows %s", shadowState),
			fmt.Sprintf("shadow_color #%s77", pHex),
		)
	}

	_, err := c.RunCommand(strings.Join(cmds, "; "))
	return err
}

// SyncNativeDesktop updates ~/.config dotfiles and triggers 60fps swww wallpaper animation + live Waybar/Kitty/SwayNC CSS reload.
func (c *Client) SyncNativeDesktop(theme config.Theme, s config.DesktopSettings) {
	if !c.Available() {
		return
	}
	_ = c.ApplyLiveState(theme, s)

	go func() {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return
		}
		cfgDir := filepath.Join(home, ".config")
		_, _ = ricer.ExportToDir(cfgDir, theme, s)

		wallPath := filepath.Join(cfgDir, "kitsune-de", "wallpapers", filepath.Base(theme.Wallpaper))
		if _, err := os.Stat(wallPath); err == nil {
			if swwwBin, err := exec.LookPath("swww"); err == nil {
				_ = exec.Command(swwwBin, "img", wallPath,
					"--transition-type", "grow",
					"--transition-pos", "0.85,0.90",
					"--transition-duration", "1.1",
					"--transition-fps", "60",
				).Run()
			} else {
				_, _ = c.RunCommand(fmt.Sprintf("output * bg %s fill", wallPath))
			}
		}

		// Hot-reload Waybar CSS, Kitty ANSI colors, and SwayNC theme without flickering
		_ = exec.Command("pkill", "-SIGUSR2", "waybar").Run()
		_ = exec.Command("killall", "-SIGUSR1", "kitty").Run()
		if swayncBin, err := exec.LookPath("swaync-client"); err == nil {
			_ = exec.Command(swayncBin, "-rs").Run()
		}
	}()
}
