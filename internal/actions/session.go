// Package actions agrupa las acciones del escritorio: energía, sesión,
// capturas, bloqueo y notificaciones; y las presenta como menús TUI.
package actions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// Action es una acción ejecutable.
type Action struct {
	ID     string
	Label  string
	Desc   string
	Icon   string
	Danger bool
	Run    func() error
}

// Notification envía una notificación de escritorio si hay un daemon disponible.
func Notification(summary, body string, urgency string) {
	if bin := companionCommand("swaynotify"); bin != "" {
		args := []string{"send", "--app", "swaydesk"}
		if urgency != "" {
			args = append(args, "--urgency", urgency)
		}
		args = append(args, summary, body)
		if launcher.StartDetached(bin, args...) == nil {
			return
		}
	}
	if launcher.HasCommand("notify-send") {
		args := []string{"-a", "swaydesk"}
		if urgency != "" {
			args = append(args, "-u", urgency)
		}
		args = append(args, summary, body)
		if launcher.StartDetached("notify-send", args...) == nil {
			return
		}
	}
	if launcher.HasCommand("swaynotify-send") {
		_ = launcher.StartDetached("swaynotify-send", summary, body)
	}
}

// Lock bloquea la sesión con la mejor presentación disponible y conserva un
// fallback compatible con swaylock estándar. Los efectos son opcionales: no se
// instala ni se requiere swaylock-effects para poder bloquear la sesión.
func Lock() error {
	bg, clockFontSize := lockAppearance()
	if launcher.HasCommand("swaylock-effects") {
		return launcher.StartDetached("swaylock-effects", swaylockEffectsArgs(bg, clockFontSize)...)
	}
	if launcher.HasCommand("swaylock") {
		if swaylockSupportsEffects("swaylock") {
			return launcher.StartDetached("swaylock", swaylockEffectsArgs(bg, clockFontSize)...)
		}
		return launcher.StartDetached("swaylock", swaylockArgs(bg)...)
	}
	if launcher.HasCommand("gtklock") {
		return launcher.StartDetached("gtklock")
	}
	if launcher.HasCommand("loginctl") {
		return launcher.StartDetached("loginctl", "lock-session")
	}
	return fmt.Errorf("no se encontró un bloqueador de pantalla (instala swaylock)")
}

// swaylockEffectsArgs añade desenfoque, viñeta, reloj y fade-in solo para una
// compilación de swaylock-effects; el swaylock estándar nunca recibe estas flags.
func swaylockEffectsArgs(bg string, clockFontSize int) []string {
	return []string{
		"-f", "--screenshots",
		"--effect-scale", "0.5", "--effect-blur", "7x5", "--effect-scale", "2",
		"--effect-vignette", "0.5:0.5", "--clock", "--indicator",
		"--timestr", "%H:%M", "--datestr", "%A %d de %B",
		"--font-size", strconv.Itoa(clockFontSize), "--fade-in", "0.25", "-c", bg,
	}
}

// swaylockArgs conserva el bloqueo básico, la paleta y la compatibilidad con
// swaylock upstream, que no incorpora reloj ni efectos de imagen.
func swaylockArgs(bg string) []string {
	return []string{"-f", "-c", bg}
}

// swaylockSupportsEffects identifica el fork habitual de Arch, que instala el
// binario como "swaylock" y no como "swaylock-effects".
func swaylockSupportsEffects(binary string) bool {
	path, err := exec.LookPath(binary)
	if err != nil {
		return false
	}
	output, _ := exec.Command(path, "--help").CombinedOutput()
	return strings.Contains(string(output), "--effect-blur")
}

// lockAppearance obtiene la paleta activa y el tamaño del reloj de la
// configuración. El color se devuelve sin '#', como espera --color; la
// plantilla mantiene el formato #RRGGBB requerido por swaylock/config.
func lockAppearance() (bg string, clockFontSize int) {
	cfg, err := config.Load()
	fontSize := 11
	if err == nil {
		bg = strings.TrimPrefix(string(ui.PaletteByName(cfg.Theme).Bg), "#")
		if cfg.FontSize > 0 {
			fontSize = cfg.FontSize
		}
	} else {
		bg = "1a1b26"
	}
	return bg, fontSize*4 + 4
}

// PowerActions devuelve las acciones del menú de energía.
func PowerActions(confirm bool) []Action {
	return []Action{
		{ID: "lock", Label: "Bloquear pantalla", Desc: "swaylock", Icon: "\uf023",
			Run: Lock},
		{ID: "suspend", Label: "Suspender", Desc: "systemctl suspend", Icon: "\uf186",
			Run: func() error {
				_ = Lock()
				if launcher.HasCommand("systemctl") {
					return launcher.StartDetached("systemctl", "suspend")
				}
				return launcher.StartDetached("loginctl", "suspend")
			}},
		{ID: "logout", Label: "Cerrar sesión de sway", Desc: "swaymsg exit", Icon: "\uf08b", Danger: true,
			Run: func() error {
				if launcher.HasCommand("swaymsg") {
					return launcher.StartDetached("swaymsg", "exit")
				}
				return swayipcDial(func(c *swayipc.Conn) error {
					return c.CommandOK("exit")
				})
			}},
		{ID: "reboot", Label: "Reiniciar", Desc: "systemctl reboot", Icon: "\uf021", Danger: true,
			Run: func() error { return launcher.StartDetached("systemctl", "reboot") }},
		{ID: "poweroff", Label: "Apagar", Desc: "systemctl poweroff", Icon: "\uf011", Danger: true,
			Run: func() error { return launcher.StartDetached("systemctl", "poweroff") }},
	}
}

func swayipcDial(fn func(*swayipc.Conn) error) error {
	c, err := swayipc.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(c)
}

// Screenshot captura la pantalla completa o una región según el modo, guarda
// el PNG, lo copia como image/png y envía una notificación ligera.
func Screenshot(mode string) error {
	if !launcher.HasCommand("grim") {
		return fmt.Errorf("se necesita grim para capturas de pantalla")
	}
	var args []string
	switch mode {
	case "region":
		if !launcher.HasCommand("slurp") {
			return fmt.Errorf("se necesita slurp para seleccionar una región")
		}
		geometry, err := launcher.Output("slurp")
		if err != nil {
			return fmt.Errorf("selección cancelada")
		}
		geometry = strings.TrimSpace(geometry)
		if geometry == "" {
			return fmt.Errorf("selección vacía")
		}
		args = []string{"-g", geometry}
	case "", "full":
		// Sin geometría, grim captura todos los outputs.
	default:
		return fmt.Errorf("modo de captura desconocido: %q (usa region o full)", mode)
	}

	dir, err := screenshotDir()
	if err != nil {
		return err
	}
	name := filepath.Join(dir, "captura-"+time.Now().Format("20060102-150405.000")+".png")
	args = append(args, name)
	cmd := exec.Command("grim", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("grim: %w: %s", err, detail)
		}
		return fmt.Errorf("grim: %w", err)
	}

	copyErr := copyScreenshotToClipboard(name)
	if copyErr != nil {
		screenshotNotification(name, false, copyErr)
		return fmt.Errorf("captura guardada en %s, pero no se copió al portapapeles: %w", name, copyErr)
	}
	screenshotNotification(name, true, nil)
	return nil
}

// copyScreenshotToClipboard ofrece el PNG con el tipo MIME adecuado para que
// editores y aplicaciones reciban una imagen, no una ruta o texto.
func copyScreenshotToClipboard(path string) error {
	if !launcher.HasCommand("wl-copy") {
		return fmt.Errorf("no se encontró wl-copy; instala wl-clipboard")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("abriendo la captura: %w", err)
	}
	defer file.Close()

	cmd := exec.Command("wl-copy", "--type", "image/png")
	cmd.Stdin = file
	if output, err := cmd.CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("wl-copy: %w: %s", err, detail)
		}
		return fmt.Errorf("wl-copy: %w", err)
	}
	return nil
}

// screenshotNotification envía un aviso breve, sin cargar ni dibujar la imagen;
// así la notificación no espera a decodificar una captura grande.
func screenshotNotification(path string, copied bool, copyErr error) {
	summary := "Captura guardada"
	body := "Guardada en " + path
	urgency := "low"
	if copied {
		summary = "Captura copiada"
		body += " · Copiada al portapapeles"
	} else if copyErr != nil {
		body += " · No se pudo copiar: " + copyErr.Error()
		urgency = "critical"
	}

	if bin := companionCommand("swaynotify"); bin != "" {
		if launcher.StartDetached(bin, "send", "--app", "swaydesk", "--urgency", urgency,
			"--timeout", "3500", summary, body) == nil {
			return
		}
	}
	if launcher.HasCommand("notify-send") {
		_ = launcher.StartDetached("notify-send", "-a", "swaydesk", "-u", urgency,
			"-t", "3500", summary, body)
	}
}

// companionCommand encuentra un binario instalado junto a swaydesk, aunque
// ~/.local/bin no forme parte del PATH heredado por sway.
func companionCommand(name string) string {
	if bin, err := exec.LookPath(name); err == nil {
		return bin
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	bin := filepath.Join(filepath.Dir(exe), name)
	info, err := os.Stat(bin)
	if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
		return bin
	}
	return ""
}

func screenshotDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no se pudo determinar la carpeta personal: %w", err)
	}
	dir := filepath.Join(home, "Capturas")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creando la carpeta de capturas %s: %w", dir, err)
	}
	return dir, nil
}

// ToggleIdle gestiona el inhibidor de suspensión (swayidle).
func ToggleIdle() error {
	if launcher.HasCommand("swayidle") {
		if launcher.HasCommand("pkill") && launcher.StartDetached("pkill", "-USR1", "swayidle") == nil {
			return nil
		}
	}
	return fmt.Errorf("swayidle no está disponible")
}

// Volume ajusta el volumen mediante pactl, wpctl o amixer.
func Volume(delta int, mute bool) error {
	switch {
	case launcher.HasCommand("wpctl"):
		if mute {
			return launcher.StartDetached("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle")
		}
		arg := "5%+"
		if delta < 0 {
			arg = "5%-"
		}
		return launcher.StartDetached("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", arg)
	case launcher.HasCommand("pactl"):
		if mute {
			return launcher.StartDetached("pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle")
		}
		arg := "+" + strconv.Itoa(delta) + "%"
		if delta < 0 {
			arg = strconv.Itoa(delta) + "%"
		}
		return launcher.StartDetached("pactl", "set-sink-volume", "@DEFAULT_SINK@", arg)
	case launcher.HasCommand("amixer"):
		if mute {
			return launcher.StartDetached("amixer", "set", "Master", "toggle")
		}
		arg := strconv.Itoa(delta) + "%+"
		if delta < 0 {
			arg = strconv.Itoa(-delta) + "%-"
		}
		return launcher.StartDetached("amixer", "set", "Master", arg)
	}
	return fmt.Errorf("no se encontró herramienta de audio (wpctl, pactl o amixer)")
}
