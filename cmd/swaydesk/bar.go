package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// cmdBar gestiona la barra de estado propia (swaybar).
func cmdBar(args []string) {
	cfg, err := config.Load()
	must(err)
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)

	// --enabled se usa desde el autostart: código 0 = usar swaybar.
	if hasFlag(args, "--enabled") {
		if cfg.BarEnabled {
			return
		}
		os.Exit(1)
	}
	if len(args) == 0 {
		barStatus(cfg, theme)
		return
	}
	switch args[0] {
	case "status", "estado":
		barStatus(cfg, theme)
	case "on", "activar":
		cfg.BarEnabled = true
		must(saveBarConfig(cfg))
		fmt.Println("✔ barra propia activada · aplica el cambio con: swaymsg reload")
	case "off", "desactivar":
		cfg.BarEnabled = false
		must(saveBarConfig(cfg))
		fmt.Println("✔ barra propia desactivada · Waybar se iniciará al recargar Sway: swaymsg reload")
	case "preview", "vista":
		must(barPreview(cfg, args))
	case "modules", "modulos", "módulos":
		barModules(cfg, theme, args)
	case "config", "ajustes":
		barConfig(cfg, theme)
	default:
		fmt.Fprintf(os.Stderr, "swaydesk: subcomando de barra desconocido %q\n", args[0])
		fmt.Fprintln(os.Stderr, "usa: swaydesk bar [status|on|off|preview|modules|config]")
		os.Exit(2)
	}
}

func saveBarConfig(cfg *config.Config) error { return cfg.Save() }

func withoutPowerModule(modules []string) []string {
	clean := make([]string, 0, len(modules))
	for _, module := range modules {
		if module != "power" {
			clean = append(clean, module)
		}
	}
	return clean
}

func barStatus(cfg *config.Config, theme *ui.Theme) {
	p := theme.Palette
	state := "waybar"
	if cfg.BarEnabled {
		state = "swaydesk (propia)"
	}
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("barra de estado"), ui.Dim("· "+state))
	fmt.Printf("  %-12s %s\n", ui.Dim("posición"), cfg.BarPosition)
	fmt.Printf("  %-12s %d px (margen %d, relleno %d)\n", ui.Dim("alto"), cfg.BarHeight, cfg.BarMargin, cfg.BarPadding)
	fmt.Printf("  %-12s %.0f %% · islas %v\n", ui.Dim("opacidad"), cfg.BarBackgroundOpacity*100, cfg.BarIslands)
	modules := withoutPowerModule(cfg.BarModules)
	fmt.Printf("  %-12s %s\n", ui.Dim("módulos"), strings.Join(modules, " · "))
	if !launcher.HasCommand("swaybar") {
		fmt.Printf("  %s\n", ui.Style{Fg: p.Warn}.Render("• swaybar no está en el PATH: instala con make install"))
	}
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Dim("vista previa"), ui.Dim("swaydesk bar preview"))
	fmt.Printf("  %s %s\n\n", ui.Dim("desactivar"), ui.Dim("swaydesk bar off (vuelve waybar)"))
}

func barModules(cfg *config.Config, theme *ui.Theme, args []string) {
	p := theme.Palette
	available := []string{"media", "recording", "cpu", "ram", "disk", "temp", "uptime", "volume", "network", "battery", "updates", "title", "clock", "date"}
	if len(args) > 1 {
		list := strings.Split(strings.Join(args[1:], ","), ",")
		var clean []string
		for _, m := range list {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			clean = append(clean, m)
		}
		clean = withoutPowerModule(clean)
		cfg.BarModules = clean
		if err := saveBarConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✔ módulos: %s\n", strings.Join(clean, " · "))
		fmt.Println(ui.Dim("reinicia swaybar para verlos"))
		return
	}
	fmt.Printf("\n  %s\n\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("módulos disponibles"))
	activeModules := withoutPowerModule(cfg.BarModules)
	for _, m := range available {
		active := " "
		for _, cur := range activeModules {
			if cur == m {
				active = ui.Style{Fg: p.Ok}.Render("●")
			}
		}
		fmt.Printf("  %s %s\n", active, m)
	}
	fmt.Println()
	fmt.Println(ui.Dim("uso: swaydesk bar modules cpu ram volume clock"))
	fmt.Println()
}

func barConfig(cfg *config.Config, theme *ui.Theme) {
	p := theme.Palette
	fmt.Printf("\n  %s\n\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("sección [bar] de config.toml"))
	fmt.Printf("  enabled    = %v\n", cfg.BarEnabled)
	fmt.Printf("  position   = %q\n", cfg.BarPosition)
	fmt.Printf("  height     = %d\n", cfg.BarHeight)
	fmt.Printf("  margin     = %d\n", cfg.BarMargin)
	fmt.Printf("  padding    = %d\n", cfg.BarPadding)
	fmt.Printf("  gap        = %d\n", cfg.BarGap)
	fmt.Printf("  radius     = %d\n", cfg.BarRadius)
	fmt.Printf("  opacity    = %.2f\n", cfg.BarBackgroundOpacity)
	fmt.Printf("  islands    = %v\n", cfg.BarIslands)
	fmt.Printf("  font       = %q\n", cfg.BarFont)
	fmt.Printf("  shadow     = %v\n", cfg.BarShadow)
	fmt.Printf("  exclusive  = %v\n", cfg.BarExclusive)
	fmt.Printf("  modules    = [%s]\n\n", strings.Join(withoutPowerModule(cfg.BarModules), ", "))
	fmt.Println(ui.Dim("edita ~/.config/swaydesk/config.toml y ejecuta `swaydesk apply`"))
	fmt.Println()
}

// barPreview genera un PNG de la barra con los datos actuales.
func barPreview(cfg *config.Config, args []string) error {
	exe, err := exec.LookPath("swaybar")
	if err != nil {
		// Si no está instalado, se usa el que esté junto a este binario.
		self, err2 := os.Executable()
		if err2 != nil {
			return fmt.Errorf("no se encontró swaybar: %w", err)
		}
		exe = filepath.Join(filepath.Dir(self), "swaybar")
		if _, statErr := os.Stat(exe); statErr != nil {
			return fmt.Errorf("compila swaybar con `make build` o instálalo en el PATH")
		}
	}
	out := filepath.Join(cfg.Cache, "barra.png")
	for i, a := range args {
		if a == "--out" && i+1 < len(args) {
			out = args[i+1]
		}
		if strings.HasPrefix(a, "--out=") {
			out = strings.TrimPrefix(a, "--out=")
		}
	}
	width := "1920"
	for i, a := range args {
		if a == "--width" && i+1 < len(args) {
			width = args[i+1]
		}
		if strings.HasPrefix(a, "--width=") {
			width = strings.TrimPrefix(a, "--width=")
		}
	}
	cmd := exec.Command(exe, "--preview", "--out", out, "--width", width)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	fmt.Printf("  %s\n", ui.Dim("PNG guardado en "+out))
	return nil
}
