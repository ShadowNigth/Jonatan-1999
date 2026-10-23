package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// opacityRule es una regla de translucidez para una aplicación concreta.
type opacityRule struct {
	Match string  // aplicación (app_id o clase)
	Class bool    // true si la regla usa [class=...] en lugar de [app_id=...]
	Value float64 // 0..1
}

// opacityPresets es el catálogo de aplicaciones con translucidez conocida. Es
// solo una ayuda: cualquier app_id o clase vale.
var opacityPresets = []struct {
	Match   string
	Class   bool
	Label   string
	Default float64
}{
	{"chromium", false, "Chromium", 0.96},
	{"google-chrome", false, "Google Chrome", 0.96},
	{"brave-browser", false, "Brave", 0.96},
	{"firefox", false, "Firefox", 0.96},
	{"foot", false, "foot", 0.94},
	{"kitty", false, "kitty", 0.94},
	{"Alacritty", false, "Alacritty", 0.94},
	{"org.wezfurlong.wezterm", false, "WezTerm", 0.94},
	{"com.raggesilver.BlackBox", false, "BlackBox", 0.94},
	{"code", false, "VS Code (OSS)", 0.94},
	{"Code", true, "VS Code", 0.94},
	{"obsidian", false, "Obsidian", 0.94},
	{"md.obsidian.Obsidian", false, "Obsidian (flatpak)", 0.94},
	{"org.telegram.desktop", false, "Telegram", 0.94},
	{"Signal", false, "Signal", 0.94},
	{"discord", false, "Discord", 0.94},
	{"webcord", false, "WebCord", 0.94},
	{"Element", false, "Element", 0.94},
	{"thunar", true, "Thunar", 0.94},
	{"org.gnome.Nautilus", true, "Nautilus", 0.94},
	{"dolphin", false, "Dolphin", 0.94},
	{"org.gnome.Console", false, "Consola de GNOME", 0.94},
	{"kitty-dropdown", false, "kitty (desplegable)", 0.92},
	{"pavucontrol", false, "Control de volumen", 0.94},
	{"blueman-manager", false, "Bluetooth", 0.94},
	{"nm-connection-editor", false, "Redes", 0.94},
	{"org.gnome.Calculator", false, "Calculadora", 0.94},
	{"zathura", false, "Zathura (PDF)", 0.96},
	{"org.gnome.Evince", false, "Evince (PDF)", 0.96},
	{"mpv", false, "mpv", 1.0},
	{"imv", false, "imv", 0.96},
	{"Spotify", true, "Spotify", 0.94},
	{"com.spotify.Client", false, "Spotify (flatpak)", 0.94},
	{"Cider", true, "Cider", 0.94},
	{"libreoffice", true, "LibreOffice", 0.97},
	{"org.gnome.TextEditor", false, "Editor de GNOME", 0.95},
	{"org.gnome.Calendar", false, "Calendario", 0.94},
	{"org.gnome.Music", false, "Música", 0.94},
	{"vlc", true, "VLC", 1.0},
	{"obs", false, "OBS Studio", 1.0},
	{"gimp", true, "GIMP", 1.0},
}

// cmdOpacity gestiona la translucidez por aplicación. Escribe reglas
// permanentes en config.d y las aplica en caliente por IPC.
func cmdOpacity(args []string) {
	cfg, err := config.Load()
	must(err)
	theme := ui.NewTheme(cfg.Theme, cfg.NerdFont)
	path := filepath.Join(configDir(), "sway", "config.d", "60-opacity.conf")

	if len(args) == 0 {
		listOpacity(cfg, theme, path)
		return
	}
	switch args[0] {
	case "list", "ls":
		listOpacity(cfg, theme, path)
	case "presets", "catalogo", "catálogo":
		listPresets(theme)
	case "add", "set", "aplicar":
		if len(args) < 2 {
			must(fmt.Errorf("uso: swaydesk opacity add <app_id|--class app> [valor]"))
		}
		match, isClass := args[1], false
		rest := args[2:]
		if match == "--class" && len(rest) > 0 {
			match, isClass, rest = rest[0], true, rest[1:]
		}
		val := presetValue(match)
		if len(rest) > 0 {
			f, err := strconv.ParseFloat(strings.ReplaceAll(rest[0], ",", "."), 64)
			must(err)
			val = f
		}
		must(addOpacity(path, match, isClass, val))
		applyOpacityRule(match, isClass, val)
		fmt.Printf("%s %s → %.0f %% de opacidad\n", ui.Style{Fg: theme.Palette.Ok}.Render("✔"), match, val*100)
		fmt.Printf("  %s\n", ui.Dim("guardado en "+path))
	case "remove", "rm", "quitar":
		if len(args) < 2 {
			must(fmt.Errorf("uso: swaydesk opacity remove <app_id>"))
		}
		must(removeOpacity(path, args[1]))
		fmt.Printf("✔ regla eliminada: %s\n", args[1])
	case "all", "todo":
		n, err := applyAllPresets(path)
		must(err)
		fmt.Printf("✔ %d reglas de translucidez escritas en %s\n", n, path)
		fmt.Printf("  %s\n", ui.Dim("las ventanas ya abiertas conservan su opacidad hasta que se reinicien"))
	case "reset", "limpiar":
		if err := os.Remove(path); err == nil {
			fmt.Println("✔ reglas personalizadas eliminadas")
		} else {
			fmt.Println("· no había reglas personalizadas")
		}
	default:
		must(fmt.Errorf("subcomando desconocido %q (list, presets, add, remove, all, reset)", args[0]))
	}
}

func presetValue(match string) float64 {
	for _, p := range opacityPresets {
		if strings.EqualFold(p.Match, match) {
			return p.Default
		}
	}
	return 0.94
}

func listPresets(theme *ui.Theme) {
	p := theme.Palette
	fmt.Printf("\n  %s %s\n\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("aplicaciones con translucidez"),
		ui.Dim("(swaydesk opacity add <app> [0-1])"))
	for _, pr := range opacityPresets {
		tag := "app_id"
		if pr.Class {
			tag = "class "
		}
		fmt.Printf("  %s %s %s %s\n", ui.Dim(tag), ui.PadRight(pr.Match, 28),
			ui.PadRight(pr.Label, 22), ui.Dim(fmt.Sprintf("%.0f %%", pr.Default*100)))
	}
	fmt.Println()
}

func listOpacity(cfg *config.Config, theme *ui.Theme, path string) {
	p := theme.Palette
	rules, _ := readOpacity(path)
	fmt.Printf("\n  %s %s\n\n", ui.Style{Fg: p.Accent, Bg: p.Bg}.B().Render("translucidez por aplicación"),
		ui.Dim(fmt.Sprintf("· %d reglas personalizadas", len(rules))))
	if len(rules) == 0 {
		fmt.Printf("  %s\n", ui.Dim("sin reglas propias: las de la plantilla de sway ya están activas"))
	}
	for _, r := range rules {
		fmt.Printf("  %s %s\n", ui.PadRight(r.Match, 30), fmt.Sprintf("%.0f %%", r.Value*100))
	}
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Dim("aplicar"), ui.Dim("swaydesk opacity all"))
	fmt.Printf("  %s %s\n", ui.Dim("añadir "), ui.Dim("swaydesk opacity add obsidian 0.92"))
	fmt.Printf("  %s %s\n\n", ui.Dim("ver    "), ui.Dim("swaydesk opacity presets"))
	_ = cfg
}

var opacityLineRe = regexp.MustCompile(`^\s*(?:for_window|)\s*\[(app_id|class)="([^"]+)"\]\s*opacity\s+([0-9.]+)`)

// readOpacity lee las reglas guardadas en el archivo de config.d.
func readOpacity(path string) ([]opacityRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []opacityRule
	for _, line := range strings.Split(string(data), "\n") {
		m := opacityLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			continue
		}
		out = append(out, opacityRule{Match: m[2], Class: m[1] == "class", Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Match < out[j].Match })
	return out, nil
}

// writeOpacity guarda las reglas y devuelve el texto generado.
func writeOpacity(path string, rules []opacityRule) error {
	var b strings.Builder
	b.WriteString("# swaydesk · translucidez por aplicación\n")
	b.WriteString("# Generado con `swaydesk opacity`. Se incluye al final de la configuración de sway.\n")
	b.WriteString("# Borra este archivo o usa `swaydesk opacity reset` para volver a la plantilla.\n\n")
	for _, r := range rules {
		kind := "app_id"
		if r.Class {
			kind = "class"
		}
		fmt.Fprintf(&b, "for_window [%s=\"%s\"] opacity %.2f\n", kind, r.Match, r.Value)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	if c, err := swayipc.Dial(); err == nil {
		_ = c.CommandOK("reload")
		c.Close()
	}
	return nil
}

func addOpacity(path, match string, isClass bool, value float64) error {
	rules, err := readOpacity(path)
	if err != nil {
		return err
	}
	replaced := false
	for i := range rules {
		if rules[i].Match == match {
			rules[i].Value, rules[i].Class, replaced = value, isClass, true
		}
	}
	if !replaced {
		rules = append(rules, opacityRule{Match: match, Class: isClass, Value: value})
	}
	return writeOpacity(path, rules)
}

func removeOpacity(path, match string) error {
	rules, err := readOpacity(path)
	if err != nil {
		return err
	}
	kept := rules[:0]
	for _, r := range rules {
		if r.Match != match {
			kept = append(kept, r)
		}
	}
	return writeOpacity(path, kept)
}

// applyAllPresets escribe todas las reglas del catálogo de golpe.
func applyAllPresets(path string) (int, error) {
	rules := make([]opacityRule, 0, len(opacityPresets))
	for _, p := range opacityPresets {
		rules = append(rules, opacityRule{Match: p.Match, Class: p.Class, Value: p.Default})
	}
	if err := writeOpacity(path, rules); err != nil {
		return 0, err
	}
	for _, r := range rules {
		applyOpacityRule(r.Match, r.Class, r.Value)
	}
	return len(rules), nil
}

// applyOpacityRule aplica la regla a las ventanas actuales y a las futuras.
func applyOpacityRule(match string, isClass bool, value float64) {
	c, err := swayipc.Dial()
	if err != nil {
		return
	}
	defer c.Close()
	kind := "app_id"
	if isClass {
		kind = "class"
	}
	sel := fmt.Sprintf(`[%s="%s"]`, kind, match)
	// Criterio para aplicarlo ya; for_window para las que se abran después.
	_ = c.CommandOK(sel + " opacity " + strconv.FormatFloat(value, 'f', 2, 64))
	_ = c.CommandOK("for_window " + sel + " opacity " + strconv.FormatFloat(value, 'f', 2, 64))
}
