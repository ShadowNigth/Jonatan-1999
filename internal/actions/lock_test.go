package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwaylockEffectsArgsUseCustomWallpaperAndClock(t *testing.T) {
	options := lockOptions{
		Color:              "24283b",
		Image:              "/cache/anime-lock.png",
		TimeFormat:         "%I:%M %p",
		DateFormat:         "%d/%m/%Y",
		ClockSize:          42,
		IndicatorRadius:    140,
		IndicatorThickness: 6,
	}
	args := swaylockEffectsArgs(options)
	joined := strings.Join(args, " ")
	for _, option := range []string{
		"--image /cache/anime-lock.png", "--scaling fill", "--effect-blur 5x3",
		"--effect-vignette 0.5:0.5", "--clock", "--indicator", "--fade-in 0.25",
		"--timestr %I:%M %p", "--datestr %d/%m/%Y", "--font-size 42",
		"--indicator-radius 140", "--indicator-thickness 6", "-c 24283b",
	} {
		if !strings.Contains(joined, option) {
			t.Errorf("falta la opción de bloqueo %q en %q", option, joined)
		}
	}
	if strings.Contains(joined, "--screenshots") {
		t.Fatal("el fondo de bloqueo no debe capturar las ventanas")
	}
}

func TestSwaylockEffectsWithoutWallpaperUsesAdaptiveSolidColor(t *testing.T) {
	options := lockOptions{
		Color:              "24283b",
		TimeFormat:         "%H:%M",
		DateFormat:         "%A %d de %B",
		ClockSize:          48,
		IndicatorRadius:    148,
		IndicatorThickness: 7,
	}
	args := swaylockEffectsArgs(options)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--image") || strings.Contains(joined, "--screenshots") || strings.Contains(joined, "--effect-blur") {
		t.Fatalf("sin wallpaper debe quedar un fondo sólido, sin capturas: %v", args)
	}
	if !strings.Contains(joined, "-c 24283b") {
		t.Fatalf("falta el color adaptativo: %v", args)
	}
}

func TestSwaylockFallbackUsesImageAndOnlyStandardArguments(t *testing.T) {
	options := lockOptions{
		Color:              "24283b",
		Image:              "/cache/anime-lock.png",
		IndicatorRadius:    148,
		IndicatorThickness: 7,
	}
	args := swaylockArgs(options)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--image /cache/anime-lock.png") || !strings.Contains(joined, "--scaling fill") {
		t.Fatalf("el fallback debe usar el fotograma generado: %v", args)
	}
	for _, unsupported := range []string{"--effect-", "--clock", "--timestr", "--screenshots"} {
		if strings.Contains(joined, unsupported) {
			t.Fatalf("el fallback estándar recibió la opción %q: %v", unsupported, args)
		}
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
