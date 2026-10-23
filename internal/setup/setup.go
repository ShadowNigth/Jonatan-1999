// Package setup genera e instala los archivos de configuración del escritorio
// a partir de las plantillas embebidas en el binario.
package setup

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/assets"
	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Result describe el resultado de escribir un archivo.
type Result struct {
	Path     string
	Action   string // "creado", "actualizado", "omitido", "respaldo"
	Modified bool
}

// archivosTema son los que dependen de la paleta y se regeneran con `apply`.
var archivosTema = []string{
	"waybar/style.css",
	"foot/foot.ini",
	"mako/config",
	"swaylock/config",
}

// preferredFonts lista las familias monoespaciadas buscadas, en orden.
var preferredFonts = []string{
	"JetBrainsMono Nerd Font",
	"CaskaydiaCove Nerd Font",
	"FiraCode Nerd Font",
	"Hack Nerd Font",
	"MesloLGS NF",
	"SauceCodePro Nerd Font",
	"UbuntuMono Nerd Font",
	"Iosevka Nerd Font",
	"Noto Sans Mono",
	"DejaVu Sans Mono",
	"Liberation Mono",
	"monospace",
}

// DetectFont devuelve la primera fuente disponible del sistema.
func DetectFont(preferred string, nerd bool) string {
	if preferred != "" {
		return preferred
	}
	out, err := exec.Command("fc-list", "--format", "%{family}\n").Output()
	if err != nil {
		return "monospace"
	}
	installed := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		// fc-list puede devolver varias familias separadas por comas.
		for _, fam := range strings.Split(line, ",") {
			installed[strings.TrimSpace(fam)] = true
		}
	}
	for _, f := range preferredFonts {
		if !nerd && strings.Contains(strings.ToLower(f), "nerd") {
			continue
		}
		if installed[f] {
			return f
		}
	}
	if nerd {
		// Cualquier fuente Nerd Font instalada con otro nombre.
		for fam := range installed {
			if strings.Contains(strings.ToLower(fam), "nerd font") {
				return fam
			}
		}
	}
	return "monospace"
}

// HasNerdFont indica si hay alguna Nerd Font instalada.
func HasNerdFont() bool {
	out, err := exec.Command("fc-list", "--format", "%{family}\n").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "nerd font")
}

// templateData construye los valores que sustituyen a los marcadores {{...}}.
func templateData(cfg *config.Config) map[string]string {
	p := ui.PaletteByName(cfg.Theme)
	ic := ui.IconsFor(cfg.NerdFont)
	font := DetectFont(cfg.Font, cfg.NerdFont)
	fontSize := cfg.FontSize
	if fontSize <= 0 {
		fontSize = 11
	}
	barSize := cfg.BarFontSize
	if barSize <= 0 {
		barSize = 13
	}
	return map[string]string{
		"THEME_NAME":        p.Name,
		"THEME_BG_HEX":      string(p.Bg),
		"THEME_BG_HEX_Q":    "'" + string(p.Bg) + "'",
		"THEME_BG_ALT_HEX":  string(p.BgAlt),
		"THEME_BG_SEL_HEX":  string(p.BgSel),
		"THEME_FG_HEX":      string(p.Fg),
		"THEME_FG_DIM_HEX":  string(p.FgDim),
		"THEME_MUTED_HEX":   string(p.Muted),
		"THEME_ACCENT_HEX":  string(p.Accent),
		"THEME_ACCENT2_HEX": string(p.Accent2),
		"THEME_OK_HEX":      string(p.Ok),
		"THEME_WARN_HEX":    string(p.Warn),
		"THEME_ERR_HEX":     string(p.Err),
		"THEME_BORDER_HEX":  string(p.Border),

		"FONT":               font,
		"FONT_SIZE":          itoa(fontSize - 1),
		"TERM_FONT_SIZE":     itoa(fontSize),
		"BAR_FONT_SIZE":      itoa(barSize),
		"BAR_FONT_SIZE_LOGO": itoa(barSize + 2),
		"NOTIF_FONT_SIZE":    fmt.Sprintf("%.1f", float64(fontSize)-0.5),
		"LOCK_FONT_SIZE":     itoa(fontSize + 5),
		"LOCK_CLOCK_SIZE":    itoa(fontSize*4 + 4),

		"TERMINAL":        cfg.Terminal,
		"LAUNCHER":        cfg.Launcher,
		"BROWSER":         cfg.Browser,
		"FILES":           cfg.FileManager,
		"EDITOR":          cfg.Editor,
		"KEYBOARD_LAYOUT": cfg.KeyboardLayout,
		"SWAYDESK_BIN":    "swaydesk",
		"ICON_LOGO":       ic.Workspace,
		"ICON_POWER":      ic.Power,
		"ICON_VOLUME":     ic.Volume,
		"ICON_MUTED":      ic.Muted,
		"ICON_WIFI":       ic.Wifi,
		"ICON_NET":        ic.Network,
		"ICON_CPU":        ic.CPU,
		"ICON_RAM":        ic.RAM,
		"ICON_TEMP":       ic.Warn,
		"ICON_BATTERY":    ic.Battery,
		"ICON_CHARGING":   ic.Charging,
		"ICON_PLUG":       ic.Plug,
		"ICON_CLOCK":      ic.Clock,
		"ICON_CALENDAR":   ic.Calendar,
	}
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// replaceTokens sustituye todos los marcadores {{CLAVE}} del texto.
func replaceTokens(text string, data map[string]string) string {
	pairs := make([]string, 0, len(data)*2)
	for k, v := range data {
		pairs = append(pairs, "{{"+k+"}}", v)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// Render devuelve el contenido de todas las plantillas ya sustituidas,
// indexado por su ruta relativa al directorio de configuración.
func Render(cfg *config.Config) (map[string]string, error) {
	data := templateData(cfg)
	paths, err := assets.List()
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		content, err := fs.ReadFile(assets.Files(), p)
		if err != nil {
			return nil, err
		}
		out[p] = replaceTokens(string(content), data)
	}
	// La configuración de swaydesk se genera con el paquete config para
	// mantener una única fuente de verdad.
	out["swaydesk/config.toml"] = config.DefaultFile(cfg)
	return out, nil
}

// Install escribe las plantillas en el directorio de configuración destino.
//
// Si un archivo ya existe se respalda como .bak-<fecha> y se sobrescribe solo
// cuando force es verdadero; en caso contrario se omite.
func Install(cfg *config.Config, dest string, force bool, only []string) ([]Result, error) {
	files, err := Render(cfg)
	if err != nil {
		return nil, err
	}
	// El archivo de configuración propio nunca se sobrescribe salvo --force.
	onlySet := map[string]bool{}
	for _, o := range only {
		onlySet[o] = true
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sortStrings(paths)

	var results []Result
	for _, rel := range paths {
		if len(onlySet) > 0 && !matchOnly(rel, onlySet) {
			continue
		}
		target := filepath.Join(dest, rel)
		content := files[rel]
		if _, err := os.Stat(target); err == nil {
			if !force {
				results = append(results, Result{Path: target, Action: "omitido (ya existe)"})
				continue
			}
			backup := target + ".bak-" + time.Now().Format("20060102-150405")
			if data, err := os.ReadFile(target); err == nil {
				if err := os.WriteFile(backup, data, 0o644); err == nil {
					results = append(results, Result{Path: backup, Action: "respaldo", Modified: true})
				}
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return results, err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(target, []byte(content), mode); err != nil {
			return results, err
		}
		// os.WriteFile no cambia los permisos de un archivo ya existente.
		if mode != 0o644 {
			_ = os.Chmod(target, mode)
		}
		results = append(results, Result{Path: target, Action: "escrito", Modified: true})
	}
	return results, nil
}

func matchOnly(rel string, only map[string]bool) bool {
	for prefix := range only {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Apply regenera los archivos dependientes del tema que ya estén instalados y
// avisa a waybar y a sway para que recarguen.
func Apply(cfg *config.Config, dest string) ([]Result, error) {
	files, err := Render(cfg)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, rel := range archivosTema {
		target := filepath.Join(dest, rel)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		if err := os.WriteFile(target, []byte(files[rel]), 0o644); err != nil {
			return results, err
		}
		results = append(results, Result{Path: target, Action: "actualizado", Modified: true})
	}
	ReloadWaybar()
	ApplySwayColors(cfg)
	return results, nil
}

// ReloadWaybar pide a waybar que recargue su hoja de estilos.
func ReloadWaybar() {
	if _, err := exec.LookPath("pkill"); err != nil {
		return
	}
	_ = exec.Command("pkill", "-SIGUSR2", "-x", "waybar").Run()
}

// ApplySwayColors aplica los colores del tema a la sesión de sway en curso.
func ApplySwayColors(cfg *config.Config) error {
	p := ui.PaletteByName(cfg.Theme)
	c, err := dialSway()
	if err != nil {
		return err
	}
	defer c.Close()
	cmds := []string{
		fmt.Sprintf(`client.focused %s %s %s %s %s`, p.Accent, p.Accent, p.Bg, p.Accent, p.Accent),
		fmt.Sprintf(`client.focused_inactive %s %s %s %s %s`, p.BgSel, p.BgSel, p.Fg, p.BgSel, p.BgSel),
		fmt.Sprintf(`client.unfocused %s %s %s %s %s`, p.BgAlt, p.BgAlt, p.FgDim, p.BgAlt, p.BgAlt),
		fmt.Sprintf(`client.urgent %s %s %s %s %s`, p.Err, p.Err, p.Bg, p.Err, p.Err),
		fmt.Sprintf(`default_border_color %s`, p.Border),
		fmt.Sprintf(`floating_border_color %s`, p.Border),
		fmt.Sprintf(`focused_border_color %s`, p.Accent),
		fmt.Sprintf(`urgent_border_color %s`, p.Err),
	}
	return c.CommandOK(strings.Join(cmds, "; "))
}
