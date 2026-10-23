package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type CoreStat struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

type ProcEntry struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	State   string  `json:"state"`
	MemMB   float64 `json:"memMB"`
	Threads int     `json:"threads"`
}

type Snapshot struct {
	Timestamp     int64       `json:"timestamp"`
	Hostname      string      `json:"hostname"`
	Username      string      `json:"username"`
	Distro        string      `json:"distro"`
	IsArchHost    bool        `json:"isArchHost"`
	Kernel        string      `json:"kernel"`
	Arch          string      `json:"arch"`
	GoVersion     string      `json:"goVersion"`
	Goroutines    int         `json:"goroutines"`
	CPUModel      string      `json:"cpuModel"`
	CPUCores      int         `json:"cpuCores"`
	CPUPercent    float64     `json:"cpuPercent"`
	PerCore       []CoreStat  `json:"perCore"`
	MemTotalMB    uint64      `json:"memTotalMB"`
	MemUsedMB     uint64      `json:"memUsedMB"`
	MemPercent    float64     `json:"memPercent"`
	SwapTotalMB   uint64      `json:"swapTotalMB"`
	SwapUsedMB    uint64      `json:"swapUsedMB"`
	DiskTotalGB   float64     `json:"diskTotalGB"`
	DiskUsedGB    float64     `json:"diskUsedGB"`
	DiskPercent   float64     `json:"diskPercent"`
	UptimeSeconds uint64      `json:"uptimeSeconds"`
	UptimeHuman   string      `json:"uptimeHuman"`
	LoadAvg       string      `json:"loadAvg"`
	NetRxKBps     float64     `json:"netRxKBps"`
	NetTxKBps     float64     `json:"netTxKBps"`
	Processes     []ProcEntry `json:"processes"`
}

type cpuRaw struct {
	idle  uint64
	total uint64
}

type Collector struct {
	mu       sync.Mutex
	prevCPU  map[string]cpuRaw
	prevRx   uint64
	prevTx   uint64
	prevTime time.Time
	cpuModel string
	kernel   string
	distro   string
	isArch   bool
}

func NewCollector() *Collector {
	c := &Collector{
		prevCPU:  readCPURaw(),
		cpuModel: readCPUModel(),
		kernel:   readKernel(),
	}
	c.distro, c.isArch = readDistro()
	c.prevRx, c.prevTx = readNetBytes()
	c.prevTime = time.Now()
	return c
}

func (c *Collector) Collect() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	dt := now.Sub(c.prevTime).Seconds()
	if dt <= 0 {
		dt = 1
	}

	curCPU := readCPURaw()
	var totalPct float64
	var cores []CoreStat

	if prev, ok := c.prevCPU["cpu"]; ok {
		if cur, ok2 := curCPU["cpu"]; ok2 {
			totalPct = calcCPUPercent(prev, cur)
		}
	}

	var coreNames []string
	for k := range curCPU {
		if k != "cpu" && strings.HasPrefix(k, "cpu") {
			coreNames = append(coreNames, k)
		}
	}
	sort.Slice(coreNames, func(i, j int) bool {
		ni, _ := strconv.Atoi(strings.TrimPrefix(coreNames[i], "cpu"))
		nj, _ := strconv.Atoi(strings.TrimPrefix(coreNames[j], "cpu"))
		return ni < nj
	})
	for _, name := range coreNames {
		pct := 0.0
		if prev, ok := c.prevCPU[name]; ok {
			pct = calcCPUPercent(prev, curCPU[name])
		}
		cores = append(cores, CoreStat{Name: strings.ToUpper(name), Percent: round1(pct)})
	}
	c.prevCPU = curCPU

	rx, tx := readNetBytes()
	var rxKB, txKB float64
	if rx >= c.prevRx {
		rxKB = float64(rx-c.prevRx) / 1024.0 / dt
	}
	if tx >= c.prevTx {
		txKB = float64(tx-c.prevTx) / 1024.0 / dt
	}
	c.prevRx, c.prevTx = rx, tx
	c.prevTime = now

	memTotal, memUsed, memPct, swapTotal, swapUsed := readMemInfo()
	diskTotal, diskUsed, diskPct := readDiskUsage("/")
	uptimeSec := readUptime()
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "arch-kitsune"
	}
	username := "kitsune"
	if u, err := user.Current(); err == nil && u.Username != "" {
		username = u.Username
	}

	return Snapshot{
		Timestamp:     now.UnixMilli(),
		Hostname:      hostname,
		Username:      username,
		Distro:        c.distro,
		IsArchHost:    c.isArch,
		Kernel:        c.kernel,
		Arch:          runtime.GOARCH,
		GoVersion:     runtime.Version(),
		Goroutines:    runtime.NumGoroutine(),
		CPUModel:      c.cpuModel,
		CPUCores:      runtime.NumCPU(),
		CPUPercent:    round1(totalPct),
		PerCore:       cores,
		MemTotalMB:    memTotal,
		MemUsedMB:     memUsed,
		MemPercent:    round1(memPct),
		SwapTotalMB:   swapTotal,
		SwapUsedMB:    swapUsed,
		DiskTotalGB:   round1(diskTotal),
		DiskUsedGB:    round1(diskUsed),
		DiskPercent:   round1(diskPct),
		UptimeSeconds: uptimeSec,
		UptimeHuman:   formatUptime(uptimeSec),
		LoadAvg:       readLoadAvg(),
		NetRxKBps:     round1(rxKB),
		NetTxKBps:     round1(txKB),
		Processes:     readTopProcesses(12),
	}
}

func calcCPUPercent(prev, cur cpuRaw) float64 {
	if cur.total <= prev.total {
		return 0
	}
	dTotal := float64(cur.total - prev.total)
	dIdle := float64(cur.idle - prev.idle)
	if dIdle > dTotal {
		return 0
	}
	pct := (1.0 - dIdle/dTotal) * 100.0
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func readCPURaw() map[string]cpuRaw {
	out := make(map[string]cpuRaw)
	f, err := os.Open("/proc/stat")
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			if i == 4 || i == 5 { // idle + iowait
				idle += v
			}
		}
		out[fields[0]] = cpuRaw{idle: idle, total: total}
	}
	return out
}

func readMemInfo() (totalMB, usedMB uint64, pct float64, swapTotalMB, swapUsedMB uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0, 0
	}
	defer f.Close()

	vals := map[string]uint64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) >= 2 {
			key := strings.TrimSuffix(parts[0], ":")
			v, _ := strconv.ParseUint(parts[1], 10, 64)
			vals[key] = v
		}
	}
	memTotalKB := vals["MemTotal"]
	memAvailKB := vals["MemAvailable"]
	if memAvailKB == 0 {
		memAvailKB = vals["MemFree"] + vals["Buffers"] + vals["Cached"]
	}
	var memUsedKB uint64
	if memTotalKB > memAvailKB {
		memUsedKB = memTotalKB - memAvailKB
	}
	totalMB = memTotalKB / 1024
	usedMB = memUsedKB / 1024
	if memTotalKB > 0 {
		pct = float64(memUsedKB) / float64(memTotalKB) * 100.0
	}
	swapTotalKB := vals["SwapTotal"]
	swapFreeKB := vals["SwapFree"]
	swapTotalMB = swapTotalKB / 1024
	if swapTotalKB > swapFreeKB {
		swapUsedMB = (swapTotalKB - swapFreeKB) / 1024
	}
	return
}

func readDiskUsage(path string) (totalGB, usedGB, pct float64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0
	}
	totalBytes := float64(st.Blocks) * float64(st.Bsize)
	availBytes := float64(st.Bavail) * float64(st.Bsize)
	usedBytes := totalBytes - availBytes
	totalGB = totalBytes / (1024 * 1024 * 1024)
	usedGB = usedBytes / (1024 * 1024 * 1024)
	if totalBytes > 0 {
		pct = (usedBytes / totalBytes) * 100.0
	}
	return
}

func readUptime() uint64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(fields[0], 64)
	return uint64(f)
}

func formatUptime(sec uint64) string {
	d := sec / 86400
	h := (sec % 86400) / 3600
	m := (sec % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm %ds", m, sec%60)
}

func readLoadAvg() string {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "0.00 0.00 0.00"
	}
	fields := strings.Fields(string(b))
	if len(fields) >= 3 {
		return strings.Join(fields[:3], " ")
	}
	return strings.TrimSpace(string(b))
}

func readNetBytes() (rx, tx uint64) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:colon])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) >= 9 {
			r, _ := strconv.ParseUint(fields[0], 10, 64)
			t, _ := strconv.ParseUint(fields[8], 10, 64)
			rx += r
			tx += t
		}
	}
	return rx, tx
}

func readCPUModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return runtime.GOARCH + " Processor"
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") {
			if idx := strings.IndexByte(line, ':'); idx >= 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return runtime.GOARCH + " Processor"
}

func readKernel() string {
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "Linux"
	}
	return strings.TrimSpace(string(b))
}

func readDistro() (string, bool) {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Arch Linux (KitsuneDE)", false
	}
	var pretty, id string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			pretty = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		} else if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
		}
	}
	isArch := strings.Contains(strings.ToLower(id), "arch")
	if isArch {
		return pretty, true
	}
	if pretty != "" {
		return "Arch Linux [KitsuneDE on " + pretty + "]", false
	}
	return "Arch Linux (KitsuneDE)", false
}

func readTopProcesses(limit int) []ProcEntry {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var list []ProcEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		statusBytes, err := os.ReadFile(filepath.Join("/proc", e.Name(), "status"))
		if err != nil {
			continue
		}
		var name, state string
		var rssKB uint64
		var threads int
		for _, line := range strings.Split(string(statusBytes), "\n") {
			switch {
			case strings.HasPrefix(line, "Name:"):
				name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			case strings.HasPrefix(line, "State:"):
				fields := strings.Fields(strings.TrimPrefix(line, "State:"))
				if len(fields) > 0 {
					state = fields[0]
				}
			case strings.HasPrefix(line, "VmRSS:"):
				fields := strings.Fields(strings.TrimPrefix(line, "VmRSS:"))
				if len(fields) > 0 {
					rssKB, _ = strconv.ParseUint(fields[0], 10, 64)
				}
			case strings.HasPrefix(line, "Threads:"):
				fields := strings.Fields(strings.TrimPrefix(line, "Threads:"))
				if len(fields) > 0 {
					threads, _ = strconv.Atoi(fields[0])
				}
			}
		}
		if name == "" {
			continue
		}
		list = append(list, ProcEntry{
			PID:     pid,
			Name:    name,
			State:   state,
			MemMB:   round1(float64(rssKB) / 1024.0),
			Threads: threads,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].MemMB == list[j].MemMB {
			return list[i].PID > list[j].PID
		}
		return list[i].MemMB > list[j].MemMB
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10.0
}
