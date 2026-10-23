package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/host"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
)

// cmdOSD gestiona los avisos en pantalla (swayosd).
//
//	swaydesk osd                 configuración y estado actual
//	swaydesk osd show            muestra el OSD con el volumen actual
//	swaydesk osd preview RUTA    PNG del aviso
func cmdOSD(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
		os.Exit(1)
	}
	sub := ""
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	exe := "swayosd"
	if !launcher.HasCommand(exe) {
		exe = ""
	}

	switch sub {
	case "":
		fmt.Printf("enabled     = %v\n", cfg.OSDEnabled)
		fmt.Printf("position    = %s\n", cfg.OSDPosition)
		fmt.Printf("width       = %d\n", cfg.OSDWidth)
		fmt.Printf("margin      = %d\n", cfg.OSDMargin)
		fmt.Printf("padding     = %d\n", cfg.OSDPadding)
		fmt.Printf("radius      = %d\n", cfg.OSDRadius)
		fmt.Printf("opacity     = %.2f\n", cfg.OSDOpacity)
		fmt.Printf("timeout_ms  = %d\n", cfg.OSDTimeout)
		fmt.Printf("fade_ms     = %d\n", cfg.OSDFade)
		fmt.Printf("segments    = %d\n", cfg.OSDSegments)
		fmt.Printf("font        = %s\n", fontLabel(cfg.OSDFont))
		fmt.Printf("font_scale  = %d\n", cfg.OSDFontScale)
		if exe == "" {
			fmt.Printf("binario     = no instalado (usa el paquete o make install)\n")
		} else {
			fmt.Printf("binario     = %s\n", exe)
		}
		if vol, muted, ok := host.Audio(); ok {
			estado := fmt.Sprintf("%d%%", vol)
			if muted {
				estado += " (silenciado)"
			}
			fmt.Printf("volumen     = %s\n", estado)
			if d := host.AudioDevice(); d != "" {
				fmt.Printf("dispositivo = %s\n", d)
			}
		}
	case "show", "mostrar":
		if exe == "" {
			fmt.Fprintln(os.Stderr, "swaydesk: falta swayosd")
			os.Exit(1)
		}
		if err := launcher.ExecCommand(exe, "show"); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
			os.Exit(1)
		}
	case "preview":
		if exe == "" {
			fmt.Fprintln(os.Stderr, "swaydesk: falta swayosd")
			os.Exit(1)
		}
		out := "osd.png"
		if len(args) > 0 {
			out = args[0]
		}
		if err := launcher.ExecCommand(exe, "preview", "--out", out); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
			os.Exit(1)
		}
	case "volume", "volumen":
		if exe == "" || len(args) == 0 {
			fmt.Fprintln(os.Stderr, "uso: swaydesk osd volume <up|down|mute>")
			os.Exit(2)
		}
		step := ""
		if len(args) > 1 {
			if _, err := strconv.Atoi(args[1]); err == nil {
				step = args[1]
			}
		}
		cmdArgs := []string{"volume", args[0]}
		if step != "" {
			cmdArgs = append(cmdArgs, "--step", step)
		}
		if err := launcher.ExecCommand(exe, cmdArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "swaydesk: suborden de osd desconocida %q\n", sub)
		fmt.Fprintln(os.Stderr, "uso: swaydesk osd [show|preview [RUTA]|volume up|down|mute]")
		os.Exit(2)
	}
}

// fontLabel describe la fuente configurada.
func fontLabel(s string) string {
	if s == "" {
		return "automática"
	}
	return s
}
