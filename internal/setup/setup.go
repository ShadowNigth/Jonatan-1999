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
	// foot exige colores RRGGBB sin '#' y la sección [colors-dark] desde la
	// versión 1.26 (en 1.28 [colors] ya ni siquiera existe).
	rgb := func(c ui.Color) string { return strings.TrimPrefix(string(c), "#") }
	return map[string]string{
		"THEME_NAME":          p.Name,
		"FOOT_COLORS_SECTION": footColorsSection(),
		"THEME_BG_RGB":        rgb(p.Bg),
		"THEME_BG_ALT_RGB":    rgb(p.BgAlt),
		"THEME_BG_SEL_RGB":    rgb(p.BgSel),
		"THEME_FG_RGB":        rgb(p.Fg),
		"THEME_FG_DIM_RGB":    rgb(p.FgDim),
		"THEME_MUTED_RGB":     rgb(p.Muted),
		"THEME_ACCENT_RGB":    rgb(p.Accent),
		"THEME_ACCENT2_RGB":   rgb(p.Accent2),
		"THEME_OK_RGB":        rgb(p.Ok),
		"THEME_WARN_RGB":      rgb(p.Warn),
		"THEME_ERR_RGB":       rgb(p.Err),
		"THEME_BORDER_RGB":    rgb(p.Border),
		"THEME_BG_HEX":        string(p.Bg),
		"THEME_BG_HEX_Q":      "'" + string(p.Bg) + "'",
		"THEME_BG_ALT_HEX":    string(p.BgAlt),
		"THEME_BG_SEL_HEX":    string(p.BgSel),
		"THEME_FG_HEX":        string(p.Fg),
		"THEME_FG_DIM_HEX":    string(p.FgDim),
		"THEME_MUTED_HEX":     string(p.Muted),
		"THEME_ACCENT_HEX":    string(p.Accent),
		"THEME_ACCENT2_HEX":   string(p.Accent2),
		"THEME_OK_HEX":        string(p.Ok),
		"THEME_WARN_HEX":      string(p.Warn),
		"THEME_ERR_HEX":       string(p.Err),
		"THEME_BORDER_HEX":    string(p.Border),

		"TERM_OPACITY":        opacity(cfg.TerminalOpacity, 0.94),
		"TERM_ALPHA_HEX":      alphaHex(cfg.TerminalOpacity, 0.94),
		"BAR_OPACITY":         opacity(cfg.BarOpacity, 0.86),
		"BAR_ALPHA_HEX":       alphaHex(cfg.BarOpacity, 0.86),
		"ACTIVE_OPACITY":      opacity(cfg.ActiveOpacity, 1.0),
		"ACTIVE_ALPHA_HEX":    alphaHex(cfg.ActiveOpacity, 1.0),
		"INACTIVE_OPACITY":    opacity(cfg.InactiveOpacity, 0.94),
		"INACTIVE_ALPHA_HEX":  alphaHex(cfg.InactiveOpacity, 0.94),
		"GAPS_INNER":          itoa(defaultInt(cfg.GapsInner, 6)),
		"GAPS_OUTER":          itoa(defaultInt(cfg.GapsOuter, 8)),
		"ROUNDING":            itoa(defaultInt(cfg.Rounding, 12)),
		"FX_ROUNDING_LINE":    fxRounding(cfg),
		"FX_BLUR_LINE":        fxBlur(cfg),
		"BORDER_COLORS_BLOCK": borderColorsBlock(cfg),

		"FONT":               font,
		"FONT_SIZE":          itoa(fontSize - 1),
		"TERM_FONT_SIZE":     itoa(fontSize),
		"BAR_FONT_SIZE":      itoa(barSize),
		"BAR_FONT_SIZE_LOGO": itoa(barSize + 2),
		"NOTIF_FONT_SIZE":    fmt.Sprintf("%.1f", float64(fontSize)-0.5),
		"LOCK_FONT_SIZE":     itoa(fontSize + 5),
		"LOCK_CLOCK_SIZE":    itoa(fontSize*4 + 4),

		"TERMINAL":        pickProgram(cfg.Terminal, terminalCandidates),
		"BROWSER":         pickProgram(cfg.Browser, browserCandidates),
		"FILES":           pickProgram(cfg.FileManager, filesCandidates),
		"LAUNCHER":        cfg.Launcher,
		"EDITOR":          pickProgram(cfg.Editor, editorCandidates),
		"KEYBOARD_LAYOUT": cfg.KeyboardLayout,
		// Rutas absolutas de los binarios propios. Si ~/.local/bin no está en
		// el PATH de la sesión (lo normal al arrancar sway desde un TTY o un
		// gestor de sesión), los atajos no harían nada: mejor la ruta completa.
		"SWAYDESK_BIN":  binPath("swaydesk"),
		"SWAYMENU_BIN":  binPath("swaymenu"),
		"SWAYCTL_BIN":   binPath("swayctl"),
		"SWAYCLIP_BIN":  binPath("swayclip"),
		"ICON_LOGO":     ic.Workspace,
		"ICON_POWER":    ic.Power,
		"ICON_VOLUME":   ic.Volume,
		"ICON_MUTED":    ic.Muted,
		"ICON_WIFI":     ic.Wifi,
		"ICON_NET":      ic.Network,
		"ICON_CPU":      ic.CPU,
		"ICON_RAM":      ic.RAM,
		"ICON_TEMP":     ic.Warn,
		"ICON_BATTERY":  ic.Battery,
		"ICON_CHARGING": ic.Charging,
		"ICON_PLUG":     ic.Plug,
		"ICON_CLOCK":    ic.Clock,
		"ICON_CALENDAR": ic.Calendar,
	}
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func defaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// opacity normaliza un valor 0..1 y lo formatea para foot/waybar/sway.
func opacity(v, def float64) string {
	if v <= 0 || v > 1 {
		v = def
	}
	return fmt.Sprintf("%.2f", v)
}

// alphaHex devuelve el sufijo hexadecimal de alfa para colores #RRGGBBAA.
func alphaHex(v, def float64) string {
	s := opacity(v, def)
	var f float64
	fmt.Sscanf(s, "%f", &f)
	a := int(f*255 + 0.5)
	if a < 0 {
		a = 0
	}
	if a > 255 {
		a = 255
	}
	return fmt.Sprintf("%02x", a)
}

// Programas que se prueban cuando el configurado no está instalado. Es lo que
// evita atajos muertos: si no tienes thunar, se usa el gestor que sí exista.
var (
	terminalCandidates = []string{"foot", "kitty", "alacritty", "wezterm", "ghostty", "xterm", "st"}
	browserCandidates  = []string{"chromium", "google-chrome-stable", "brave", "firefox", "epiphany", "qutebrowser"}
	filesCandidates    = []string{"thunar", "nautilus", "dolphin", "nemo", "pcmanfm", "caja", "xdg-open"}
	editorCandidates   = []string{"nvim", "vim", "vi", "nano", "micro", "helix"}
)

// pickProgram devuelve el programa configurado si existe; si no, el primero de
// los candidatos que esté instalado (o el configurado, para no dejarlo vacío).
func pickProgram(want string, candidates []string) string {
	want = strings.TrimSpace(want)
	if want == "" {
		for _, c := range candidates {
			if _, err := exec.LookPath(c); err == nil {
				return c
			}
		}
		return ""
	}
	if _, err := exec.LookPath(binName(want)); err == nil {
		return want
	}
	// El valor puede traer argumentos ("foot -e ranger"): solo el binario.
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return want
}

// binName devuelve el ejecutable de una orden con argumentos.
func binName(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// binPath devuelve la ruta absoluta de un binario de swaydesk, o su nombre si
// no se encuentra (para no romper la configuración en instalaciones parciales).
func binPath(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	// Al instalar con scripts/install.sh los binarios van a ~/.local/bin, que
	// puede no estar en PATH todavía.
	home, err := os.UserHomeDir()
	if err != nil {
		return name
	}
	candidate := filepath.Join(home, ".local", "bin", name)
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
		return candidate
	}
	return name
}

// footColorsSection devuelve el nombre de la sección de colores que acepta el
// foot instalado: [colors-dark] desde 1.26 y [colors] en versiones anteriores.
func footColorsSection() string {
	out, err := exec.Command("foot", "--version").Output()
	if err != nil {
		return "[colors-dark]"
	}
	// La salida es «foot version 1.28.0-...» o «foot 1.16.2».
	fields := strings.Fields(string(out))
	for _, f := range fields {
		if v, ok := parseVersion(f); ok {
			if v[0] > 1 || (v[0] == 1 && v[1] >= 26) {
				return "[colors-dark]"
			}
			return "[colors]"
		}
	}
	return "[colors-dark]"
}

// parseVersion lee «1.28.0» o «1.2» y devuelve mayor y menor.
func parseVersion(s string) ([2]int, bool) {
	var out [2]int
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return out, false
	}
	for i := 0; i < 2; i++ {
		n := 0
		for _, r := range parts[i] {
			if r < '0' || r > '9' {
				if n == 0 && i == 0 {
					return out, false
				}
				break
			}
			n = n*10 + int(r-'0')
		}
		out[i] = n
	}
	return out, true
}

// IsSwayfx indica si el compositor es swayfx (sway con blur y esquinas).
func IsSwayfx() bool {
	out, err := exec.Command("sway", "--version").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "swayfx")
}

// fxRounding genera la línea de esquinas redondeadas solo en swayfx.
func fxRounding(cfg *config.Config) string {
	if !IsSwayfx() {
		return "# esquinas redondeadas: requiere swayfx (corner_radius)"
	}
	return fmt.Sprintf("corner_radius %d", defaultInt(cfg.Rounding, 12))
}

// borderColorsBlock escribe los colores de borde que entiende el compositor
// instalado: en swayfx son directivas globales; en sway puro esas cuatro no
// existen (sway las rechaza) y los colores van por clase de ventana con
// client.*, que ya está más abajo en la plantilla.
func borderColorsBlock(cfg *config.Config) string {
	if !IsSwayfx() {
		return "# sway puro: sin default_border_color; los bordes se tiñen con client.* (más abajo)"
	}
	p := ui.PaletteByName(cfg.Theme)
	return fmt.Sprintf("default_border_color %s\nfloating_border_color %s\nfocused_border_color %s\nurgent_border_color %s",
		p.Border, p.Border, p.Accent, p.Err)
}

// fxBlur genera la línea de desenfoque de fondo solo en swayfx.
func fxBlur(cfg *config.Config) string {
	if !IsSwayfx() || !cfg.Blur {
		return "# desenfoque de fondo: requiere swayfx (blur enable)"
	}
	return "blur enable\nblur_radius 6\nblur_saturation 1.1"
}

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
		// Coincidencia por archivo exacto o por directorio ("sway" no debe
		// arrastrar también "swaydesk/config.toml").
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
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
	}
	// Estas cuatro solo existen en swayfx: en sway puro darían error.
	if IsSwayfx() {
		cmds = append(cmds,
			fmt.Sprintf(`default_border_color %s`, p.Border),
			fmt.Sprintf(`floating_border_color %s`, p.Border),
			fmt.Sprintf(`focused_border_color %s`, p.Accent),
			fmt.Sprintf(`urgent_border_color %s`, p.Err),
		)
	}
	return c.CommandOK(strings.Join(cmds, "; "))
}
