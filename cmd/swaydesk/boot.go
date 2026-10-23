package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/boot"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// cmdBoot reproduce la animación de arranque del escritorio. Se lanza desde el
// autostart en una ventana flotante, o a mano para verla cuando quieras.
func cmdBoot(args []string) {
	cfg, err := config.Load()
	must(err)
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	if hasFlag(args, "--list") || hasFlag(args, "--styles") {
		fmt.Println("Estilos de arranque:")
		desc := map[string]string{
			"sakura":  "pétalos rosas cayendo (por defecto)",
			"neon":    "lluvia vertical de neón",
			"matrix":  "glifos y dígitos verdes",
			"minimal": "solo el logotipo y la barra, sin partículas",
		}
		for _, s := range boot.Styles() {
			fmt.Printf("  %-8s %s\n", s, ui.Dim(desc[s]))
		}
		fmt.Println()
		fmt.Println(ui.Dim("Usar: swaydesk boot --style=neon --duration=1800"))
		return
	}

	if hasFlag(args, "--quiet") || hasFlag(args, "--no-anim") {
		return
	}
	// Cambios puntuales sin tocar la configuración.
	style := cfg.BootStyle
	if s := stringFlag(args, "--style", ""); s != "" {
		style = s
	}
	if !boot.ValidStyle(style) {
		fmt.Fprintf(os.Stderr, "swaydesk: estilo de arranque desconocido %q\n", style)
		fmt.Fprintf(os.Stderr, "disponibles: %s\n", strings.Join(boot.Styles(), ", "))
		os.Exit(2)
	}
	dur := time.Duration(cfg.BootDuration) * time.Millisecond
	if ms := flagInt(args, "--duration", 0); ms > 0 {
		dur = time.Duration(ms) * time.Millisecond
	}
	intro := cfg.BootLogo
	if hasFlag(args, "--no-logo") {
		intro = false
	}
	if !cfg.BootEnabled && !hasFlag(args, "--force") && !hasFlag(args, "--preview") {
		return
	}

	// Desde sway no hay terminal: se abre una ventana flotante sin bordes.
	if relaunchInTerminal(cfg, "swaydesk-boot", 100, 30, append([]string{"boot", "--force"}, args...)) {
		return
	}

	steps := boot.DefaultSteps(cfg)
	if err := boot.Run(boot.Options{
		Theme:    theme,
		Style:    style,
		Duration: dur,
		Steps:    steps,
		Version:  Version,
		Intro:    intro,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
	}
}

// stringFlag devuelve el valor de --bandera=valor o --bandera valor.
func stringFlag(args []string, name, def string) string {
	for i, a := range args {
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
		if a == name && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			return args[i+1]
		}
	}
	return def
}
