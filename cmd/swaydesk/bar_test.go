package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
)

func TestParseBarOpacity(t *testing.T) {
	for raw, want := range map[string]float64{
		"0":    0,
		"0.68": 0.68,
		"1":    1,
	} {
		got, err := parseBarOpacity(raw)
		if err != nil {
			t.Errorf("parseBarOpacity(%q): %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("parseBarOpacity(%q) = %v; esperado %v", raw, got, want)
		}
	}

	for _, raw := range []string{"", "no", "NaN", "+Inf", "-0.01", "1.01"} {
		if got, err := parseBarOpacity(raw); err == nil {
			t.Errorf("parseBarOpacity(%q) = %v; debía rechazarlo", raw, got)
		}
	}
}

func TestRecordingModuleUsesSpanishLabel(t *testing.T) {
	binDir := t.TempDir()
	pgrep := filepath.Join(binDir, "pgrep")
	if err := os.WriteFile(pgrep, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	got := moduleOutput("recording", &statusData{}, config.Default())
	if !strings.HasSuffix(got, "GRABANDO") {
		t.Fatalf("indicador de grabación no español: %q", got)
	}
}
