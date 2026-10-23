package host

import (
	"strings"
	"testing"
	"time"
)

func TestSnapshot(t *testing.T) {
	c := NewCollector()
	// Dos lecturas separadas para que el delta de CPU sea significativo.
	_ = c.Snapshot()
	time.Sleep(60 * time.Millisecond)
	st := c.Snapshot()

	if st.MemTotal == 0 {
		t.Fatal("no se pudo leer /proc/meminfo")
	}
	if st.MemUsed > st.MemTotal {
		t.Fatalf("memoria usada (%d) > total (%d)", st.MemUsed, st.MemTotal)
	}
	if st.CPU < 0 || st.CPU > 100 {
		t.Fatalf("CPU fuera de rango: %f", st.CPU)
	}
	if st.DiskTotal == 0 {
		t.Fatal("no se pudo leer el uso de disco")
	}
	if st.DiskUsed > st.DiskTotal {
		t.Fatal("disco usado > total")
	}
	if st.Time.IsZero() {
		t.Fatal("hora vacía")
	}
}

func TestHostYUsuario(t *testing.T) {
	if Hostname() == "" {
		t.Fatal("hostname vacío")
	}
	if User() == "" {
		t.Fatal("usuario vacío")
	}
	if Distro() == "" {
		t.Fatal("distro vacía")
	}
}

func TestDiskUsage(t *testing.T) {
	used, total := DiskUsage("/")
	if total == 0 {
		t.Fatal("total = 0")
	}
	if used > total {
		t.Fatalf("usado %d > total %d", used, total)
	}
}

func TestExec(t *testing.T) {
	out, err := Exec(2*time.Second, "sh", "-c", "echo hola")
	if err != nil || strings.TrimSpace(out) != "hola" {
		t.Fatalf("Exec = %q, %v", out, err)
	}
	if _, err := Exec(2*time.Second, "comando-que-no-existe-xyz"); err == nil {
		t.Fatal("Exec debía fallar con un binario inexistente")
	}
}

func TestClampPercent(t *testing.T) {
	if clampPercent(-5) != 0 || clampPercent(150) != 100 || clampPercent(50) != 50 {
		t.Fatal("clampPercent incorrecto")
	}
}
