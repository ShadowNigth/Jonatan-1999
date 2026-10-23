package main

import (
	"image"
	"image/png"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/bar"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/host"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/notify"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// notificationState consulta al daemon cuántas notificaciones hay en pantalla.
// Si el daemon no está en marcha devuelve ceros: la barra no debe fallar por eso.
func notificationState() (visible, replies int, dnd bool) {
	client, err := notify.DialClient()
	if err != nil {
		return 0, 0, false
	}
	defer client.Close()
	st, err := client.Status()
	if err != nil {
		return 0, 0, false
	}
	return len(st.Visible), st.Replies, st.DND
}

// encodePNG escribe una imagen en PNG.
func encodePNG(w io.Writer, img image.Image) error {
	return png.Encode(w, img)
}

// snapshot reúne en un instante todo lo que la barra puede mostrar.
func snapshot(cfg *config.Config, col *host.Collector) bar.Snapshot {
	now := time.Now()
	s := bar.Snapshot{
		Clock: now.Format("15:04"),
		Date:  strings.ToLower(now.Format("Mon 02 Jan")),
		Theme: ui.PaletteByName(cfg.Theme).Name,
	}

	st := col.Snapshot()
	s.CPU = st.CPU
	if st.MemTotal > 0 {
		s.RAM = float64(st.MemUsed) / float64(st.MemTotal) * 100
	}
	if st.DiskTotal > 0 {
		s.Disk = float64(st.DiskUsed) / float64(st.DiskTotal) * 100
	}
	s.Temp = st.Temp
	s.Volumen = st.Volume
	s.Muted = st.Muted
	s.Wifi = st.WifiName
	s.Signal = st.WifiSignal
	s.Connected = st.WifiEnabled || st.WifiName != ""
	if st.Battery != nil {
		s.HasBattery = st.Battery.Present
		s.Battery = st.Battery.Percent
		s.Charging = st.Battery.Charging()
	}
	s.Uptime = shortDuration(st.Uptime)

	if mods(cfg.BarModules)["media"] {
		s.Media = mediaTitle()
	}
	if mods(cfg.BarModules)["recording"] {
		s.Recording = running("wf-recorder") || running("wl-screenrec")
	}
	if mods(cfg.BarModules)["notify"] {
		s.Notifications, s.NotifReplies, s.NotifDND = notificationState()
	}

	// Estado de sway: workspaces y ventana enfocada.
	if c, err := swayipc.Dial(); err == nil {
		defer c.Close()
		if wss, err := c.Workspaces(); err == nil {
			counts := windowCounts(c)
			sort.SliceStable(wss, func(i, j int) bool {
				if wss[i].Num != wss[j].Num {
					return wss[i].Num < wss[j].Num
				}
				return wss[i].Name < wss[j].Name
			})
			for _, w := range wss {
				name := w.Name
				s.Workspaces = append(s.Workspaces, bar.Workspace{
					Name:    name,
					Focused: w.Focused,
					Visible: w.Visible,
					Urgent:  w.Urgent,
					Windows: counts[name],
				})
			}
		}
		if tree, err := c.Tree(); err == nil {
			if n := tree.FindFocused(); n != nil {
				s.Title = n.Title()
				s.AppID = n.App()
			}
		}
	}
	return s
}

// windowCounts cuenta las ventanas visibles de cada workspace.
func windowCounts(c *swayipc.Conn) map[string]int {
	out := map[string]int{}
	tree, err := c.Tree()
	if err != nil {
		return out
	}
	for _, ws := range tree.Nodes {
		if ws.Type != "output" {
			continue
		}
		for _, content := range ws.Nodes {
			if content.Type != "con" && content.Type != "floating_con" {
				continue
			}
			n := 0
			content.Walk(func(x *swayipc.Node) bool {
				if x.IsWindow() {
					n++
				}
				return true
			})
			out[content.Name] += n
		}
	}
	return out
}

// mods convierte la lista de módulos en un conjunto.
func mods(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, x := range list {
		m[x] = true
	}
	return m
}

// mediaTitle consulta playerctl si existe.
func mediaTitle() string {
	if !launcher.HasCommand("playerctl") {
		return ""
	}
	out, err := exec.Command("playerctl", "metadata", "--format", "{{status}}|{{artist}}|{{title}}").Output()
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) < 3 || parts[0] != "Playing" {
		return ""
	}
	title := strings.TrimSpace(parts[2])
	artist := strings.TrimSpace(parts[1])
	if title == "" {
		return ""
	}
	if artist != "" && artist != title {
		return artist + " · " + title
	}
	return title
}

// running indica si un proceso está en ejecución.
func running(name string) bool {
	if !launcher.HasCommand("pgrep") {
		return false
	}
	return exec.Command("pgrep", "-x", name).Run() == nil
}

// shortDuration formatea una duración de forma compacta: 3d 4h, 5h 12m, 42m.
func shortDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h"
	case hours > 0:
		return strconv.Itoa(hours) + "h " + strconv.Itoa(mins) + "m"
	default:
		return strconv.Itoa(mins) + "m"
	}
}
