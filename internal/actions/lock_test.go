package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwaylockEffectsArgs(t *testing.T) {
	args := swaylockEffectsArgs("24283b", 48)
	effects := strings.Join(args, " ")
	for _, option := range []string{
		"--screenshots", "--effect-blur 7x5", "--effect-vignette 0.5:0.5",
		"--clock", "--indicator", "--fade-in 0.25", "--font-size 48",
		"--timestr %H:%M", "--datestr %A %d de %B", "-c 24283b",
	} {
		if !strings.Contains(effects, option) {
			t.Errorf("falta la opción de bloqueo %q en %q", option, effects)
		}
	}
}

func TestSwaylockFallbackUsesOnlyStandardArguments(t *testing.T) {
	args := swaylockArgs("24283b")
	if len(args) != 3 || args[0] != "-f" || args[1] != "-c" || args[2] != "24283b" {
		t.Fatalf("argumentos swaylock inesperados: %v", args)
	}
	if strings.Contains(strings.Join(args, " "), "--effect-") || strings.Contains(strings.Join(args, " "), "--clock") {
		t.Fatalf("el fallback estándar recibió flags de swaylock-effects: %v", args)
	}
}

func TestSwaylockEffectsDetection(t *testing.T) {
	for _, tc := range []struct {
		name string
		help string
		want bool
	}{
		{name: "fork con efectos", help: "Usage: swaylock --effect-blur <radius>x<times>", want: true},
		{name: "swaylock estándar", help: "Usage: swaylock --indicator-radius <size>", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			binary := filepath.Join(binDir, "swaylock")
			script := "#!/bin/sh\nprintf '%s\\n' '" + tc.help + "'\n"
			if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir)
			if got := swaylockSupportsEffects("swaylock"); got != tc.want {
				t.Errorf("swaylockSupportsEffects() = %t; want %t", got, tc.want)
			}
		})
	}
}
