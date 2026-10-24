// Package host recopila información del sistema leyendo /proc y /sys, sin
// dependencias externas ni procesos auxiliares cuando es posible.
package host

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Exec ejecuta un comando y devuelve su salida estándar sin espacios sobrantes.
func Exec(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// HasCommand indica si un ejecutable está disponible en PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// State es una foto del estado del sistema en un instante.
type State struct {
	Time        time.Time
	CPU         float64 // porcentaje 0-100
	MemUsed     uint64
	MemTotal    uint64
	SwapUsed    uint64
	SwapTotal   uint64
	LoadAvg     [3]float64
	Uptime      time.Duration
	Volume      int
	Muted       bool
	WifiName    string
	WifiSignal  int
	WifiEnabled bool
	NetUp       uint64
	NetDown     uint64
	Battery     *Battery
	DiskUsed    uint64
	DiskTotal   uint64
	Temp        float64
}

// Battery describe el estado de la batería.
type Battery struct {
	Present  bool
	Percent  int
	Status   string // Charging, Discharging, Full, Unknown
	TimeLeft time.Duration
}

// Charging indica si la batería se está cargando.
func (b *Battery) Charging() bool {
	return b != nil && b.Present && (b.Status == "Charging" || b.Status == "Full")
}

// Collector mantiene el estado previo para calcular deltas (CPU y red).
type Collector struct {
	mu        sync.Mutex
	prevCPU   cpuSample
	prevNet   netSample
	prevTime  time.Time
	cache     State
	haveCache bool
	// Estado de audio cacheado por si wpctl es lento.
	volMu   sync.Mutex
	volAt   time.Time
	volVal  int
	volMute bool
}

type cpuSample struct {
	total, idle uint64
}

type netSample struct {
	rx, tx uint64
}

// NewCollector crea un colector de métricas.
func NewCollector() *Collector {
	c := &Collector{}
	c.prevCPU = readCPUSample()
	c.prevNet = readNetSample()
	c.prevTime = time.Now()
	return c
}

// Snapshot devuelve el estado actual del sistema.
func (c *Collector) Snapshot() State {
	c.mu.Lock()
	defer c.mu.Unlock()

	st := State{Time: time.Now()}

	// CPU: delta desde la muestra anterior (mínimo 150 ms entre lecturas).
	cur := readCPUSample()
	if d := cur.total - c.prevCPU.total; d > 0 {
		busy := d - (cur.idle - c.prevCPU.idle)
		st.CPU = float64(busy) * 100 / float64(d)
	} else {
		st.CPU = c.cache.CPU
	}
	c.prevCPU = cur

	st.MemTotal, st.MemUsed = readMem()
	st.SwapTotal, st.SwapUsed = readSwap()
	st.LoadAvg = readLoadAvg()
	st.Uptime = readUptime()

	curNet := readNetSample()
	st.NetUp = curNet.tx - c.prevNet.tx
	st.NetDown = curNet.rx - c.prevNet.rx
	c.prevNet = curNet

	st.Volume, st.Muted = c.audio()
	st.WifiName, st.WifiSignal, st.WifiEnabled = readWifi()
	st.Battery = readBattery()
	st.DiskUsed, st.DiskTotal = DiskUsage("/")

	if t, ok := readTemp(); ok {
		st.Temp = t
	}
	c.cache = st
	c.haveCache = true
	return st
}

func readCPUSample() cpuSample {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		var total, idle uint64
		for i, f := range fields {
			v, _ := strconv.ParseUint(f, 10, 64)
			total += v
			if i == 3 || i == 4 { // idle + iowait
				idle += v
			}
		}
		return cpuSample{total: total, idle: idle}
	}
	return cpuSample{}
}

func readNetSample() netSample {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return netSample{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var s netSample
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, ":") || strings.HasPrefix(line, "Inter-") || strings.HasPrefix(line, " face") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" || strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "veth") {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		s.rx += rx
		s.tx += tx
	}
	return s
}

func readMem() (total, used uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	var available uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = v * 1024
		case "MemAvailable":
			available = v * 1024
		}
	}
	if total > 0 {
		used = total - available
	}
	return total, used
}

func readSwap() (total, used uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	var free uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "SwapTotal":
			total = v * 1024
		case "SwapFree":
			free = v * 1024
		}
	}
	if total > 0 {
		used = total - free
	}
	return total, used
}

func readLoadAvg() [3]float64 {
	var la [3]float64
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return la
	}
	fields := strings.Fields(string(data))
	for i := 0; i < 3 && i < len(fields); i++ {
		la[i], _ = strconv.ParseFloat(fields[i], 64)
	}
	return la
}

func readUptime() time.Duration {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	secs, _ := strconv.ParseFloat(fields[0], 64)
	return time.Duration(secs) * time.Second
}

// readWifi obtiene el SSID y la calidad de señal de la interfaz inalámbrica.
func readWifi() (ssid string, signal int, enabled bool) {
	enabled = false
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return "", 0, false
	}
	var iface string
	for _, e := range entries {
		name := e.Name()
		if _, err := os.Stat(filepath.Join("/sys/class/net", name, "wireless")); err == nil {
			iface = name
			break
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", name, "phy80211")); err == nil && iface == "" {
			iface = name
		}
	}
	if iface == "" {
		return "", 0, false
	}
	if state, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "operstate")); err == nil {
		enabled = strings.TrimSpace(string(state)) == "up"
	}
	// SSID: primero iw, luego nmcli, luego iwgetid.
	if out, err := Exec(400*time.Millisecond, "iw", "dev", iface, "link"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "SSID:") {
				ssid = strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
			}
			if strings.HasPrefix(line, "signal:") {
				sig := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "signal:"), ""))
				sig = strings.TrimSuffix(sig, " dBm")
				if v, err := strconv.Atoi(sig); err == nil {
					// De dBm (-30 excelente, -90 malo) a porcentaje.
					signal = clampPercent((v + 100) * 100 / 60)
				}
			}
		}
	}
	if ssid == "" {
		if out, err := Exec(400*time.Millisecond, "nmcli", "-t", "-f", "ACTIVE,SSID", "dev", "wifi"); err == nil {
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "yes:") {
					ssid = strings.TrimPrefix(line, "yes:")
					break
				}
			}
		}
	}
	if ssid == "" {
		if out, err := Exec(400*time.Millisecond, "iwgetid", "-r"); err == nil {
			ssid = out
		}
	}
	return ssid, signal, enabled && ssid != ""
}

func clampPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// readBattery busca la primera batería disponible en /sys/class/power_supply.
func readBattery() *Battery {
	entries, err := os.ReadDir("/sys/class/power_supply")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		dir := filepath.Join("/sys/class/power_supply", e.Name())
		typ, err := os.ReadFile(filepath.Join(dir, "type"))
		if err != nil || strings.TrimSpace(string(typ)) != "Battery" {
			continue
		}
		b := &Battery{Present: true}
		if v, err := readInt(filepath.Join(dir, "capacity")); err == nil {
			b.Percent = v
		} else {
			// Algunos equipos no exponen capacity: se calcula con energy_full.
			full, err1 := readInt(filepath.Join(dir, "charge_full"))
			now, err2 := readInt(filepath.Join(dir, "charge_now"))
			if err1 != nil || err2 != nil {
				full, err1 = readInt(filepath.Join(dir, "energy_full"))
				now, err2 = readInt(filepath.Join(dir, "energy_now"))
			}
			if err1 == nil && err2 == nil && full > 0 {
				b.Percent = now * 100 / full
			}
		}
		if v, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
			b.Status = strings.TrimSpace(string(v))
		}
		if b.Status == "Discharging" {
			if power, err := readInt(filepath.Join(dir, "power_now")); err == nil && power > 0 {
				if energy, err := readInt(filepath.Join(dir, "energy_now")); err == nil {
					b.TimeLeft = time.Duration(float64(energy)/float64(power)*3600) * time.Second
				}
			}
		}
		return b
	}
	return nil
}

func readInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// readTemp devuelve la temperatura de la zona térmica más relevante.
func readTemp() (float64, bool) {
	entries, err := os.ReadDir("/sys/class/thermal")
	if err != nil {
		return 0, false
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "thermal_zone") {
			continue
		}
		v, err := readInt(filepath.Join("/sys/class/thermal", e.Name(), "temp"))
		if err != nil {
			continue
		}
		t := float64(v) / 1000
		if t > 0 && t < 150 {
			return t, true
		}
	}
	return 0, false
}

// DiskUsage devuelve el espacio usado y total de la ruta indicada.
func DiskUsage(path string) (used, total uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	total = st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	if total < free {
		return 0, total
	}
	return total - free, total
}

// audio consulta el volumen con wpctl o pactl, con caché de 300 ms para no
// lanzar un proceso por cada refresco de la barra.
func (c *Collector) audio() (int, bool) {
	c.volMu.Lock()
	defer c.volMu.Unlock()
	if time.Since(c.volAt) < 300*time.Millisecond && !c.volAt.IsZero() {
		return c.volVal, c.volMute
	}
	vol, muted, ok := queryAudio()
	if ok {
		c.volVal, c.volMute = vol, muted
		c.volAt = time.Now()
	}
	return c.volVal, c.volMute
}

// Audio devuelve el volumen del altavoz por defecto (0-100+), si está
// silenciado y si se pudo consultar. Lo usan el OSD y la barra.
func Audio() (int, bool, bool) { return queryAudio() }

// AudioDevice intenta dar el nombre descriptivo del altavoz por defecto para
// los diagnósticos de `swaydesk osd`. Devuelve "" si no se puede saber.
func AudioDevice() string {
	if HasCommand("wpctl") {
		out, err := Exec(400*time.Millisecond, "wpctl", "inspect", "@DEFAULT_AUDIO_SINK@")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				l := strings.TrimSpace(line)
				for _, key := range []string{"node.description =", "node.nick ="} {
					if strings.HasPrefix(l, key) {
						name := strings.Trim(strings.TrimSpace(l[len(key):]), "\"")
						if name != "" && !strings.EqualFold(name, "default") {
							return name
						}
					}
				}
			}
		}
	}
	if HasCommand("pactl") {
		if out, err := Exec(400*time.Millisecond, "pactl", "get-default-sink"); err == nil {
			if name := strings.TrimSpace(out); name != "" {
				return name
			}
		}
	}
	return ""
}

func queryAudio() (int, bool, bool) {
	if HasCommand("wpctl") {
		if out, err := Exec(500*time.Millisecond, "wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@"); err == nil {
			fields := strings.Fields(out)
			// Formato: "Volume: 0.65 [MUTED]"
			for i, f := range fields {
				if f == "Volume:" && i+1 < len(fields) {
					v, _ := strconv.ParseFloat(fields[i+1], 64)
					return clampPercent(int(v*100 + 0.5)), strings.Contains(out, "MUTED"), true
				}
			}
		}
	}
	if HasCommand("pactl") {
		if out, err := Exec(500*time.Millisecond, "pactl", "get-sink-volume", "@DEFAULT_SINK@"); err == nil {
			if i := strings.Index(out, "%"); i > 0 {
				start := strings.LastIndexAny(out[:i], " \t")
				if v, err := strconv.Atoi(strings.TrimSpace(out[start:i])); err == nil {
					muteOut, _ := Exec(500*time.Millisecond, "pactl", "get-sink-mute", "@DEFAULT_SINK@")
					return clampPercent(v), strings.Contains(muteOut, "yes"), true
				}
			}
		}
	}
	if HasCommand("amixer") {
		if out, err := Exec(500*time.Millisecond, "amixer", "get", "Master"); err == nil {
			if i := strings.Index(out, "["); i > 0 {
				if j := strings.Index(out[i:], "%"); j > 0 {
					if v, err := strconv.Atoi(out[i+1 : i+j]); err == nil {
						return clampPercent(v), strings.Contains(out, "[off]"), true
					}
				}
			}
		}
	}
	return 0, false, false
}

// Hostname devuelve el nombre del equipo.
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "arch"
	}
	return h
}

// User devuelve el usuario actual.
func User() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "user"
}

// Distro intenta leer el nombre de la distribución desde /etc/os-release.
func Distro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "Linux"
}
