package sysinfo

import (
	"testing"
	"time"
)

func TestCollectorCollect(t *testing.T) {
	c := NewCollector()
	time.Sleep(30 * time.Millisecond)
	snap := c.Collect()

	if snap.CPUCores <= 0 {
		t.Fatalf("expected CPUCores > 0, got %d", snap.CPUCores)
	}
	if snap.MemTotalMB == 0 {
		t.Fatalf("expected MemTotalMB > 0")
	}
	if snap.Kernel == "" {
		t.Fatalf("expected non-empty Kernel")
	}
	if len(snap.Processes) == 0 {
		t.Fatalf("expected non-empty process list")
	}
}
