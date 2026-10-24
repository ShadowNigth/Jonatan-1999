package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// TerminalSpec describe cómo arrancar una ventana de terminal concreta.
type TerminalSpec struct {
	Binary string
	// Args construye los argumentos para lanzar cmd dentro del terminal.
	Args func(appID, title string, width, height int, command []string) []string
}

// terminals contiene los emuladores soportados, en orden de preferencia.
var terminals = map[string]TerminalSpec{
	"foot": {
		Binary: "foot",
		Args: func(appID, title string, w, h int, cmd []string) []string {
			args := []string{"--app-id=" + appID}
			if title != "" {
				args = append(args, "--title="+title)
			}
			if w > 0 && h > 0 {
				args = append(args, "-W", fmt.Sprintf("%dx%d", w, h))
			}
			if len(cmd) > 0 {
				args = append(args, "-e")
				args = append(args, cmd...)
			}
			return args
		},
	},
	"kitty": {
		Binary: "kitty",
		Args: func(appID, title string, w, h int, cmd []string) []string {
			args := []string{"--class", appID}
			if title != "" {
				args = append(args, "--title", title)
			}
			if w > 0 && h > 0 {
				args = append(args, "-o", fmt.Sprintf("initial_window_width=%dc", w),
					"-o", fmt.Sprintf("initial_window_height=%dc", h))
			}
			args = append(args, cmd...)
			return args
		},
	},
	"alacritty": {
		Binary: "alacritty",
		Args: func(appID, title string, w, h int, cmd []string) []string {
			args := []string{"--class", appID}
			if title != "" {
				args = append(args, "--title", title)
			}
			if w > 0 && h > 0 {
				args = append(args, "--option", fmt.Sprintf("window.dimensions.columns=%d", w),
					"--option", fmt.Sprintf("window.dimensions.lines=%d", h))
			}
			if len(cmd) > 0 {
				args = append(args, "-e")
				args = append(args, cmd...)
			}
			return args
		},
	},
	"wezterm": {
		Binary: "wezterm",
		Args: func(appID, title string, w, h int, cmd []string) []string {
			args := []string{"start", "--class", appID}
			if title != "" {
				args = append(args, "--", cmd[0])
				cmd = cmd[1:]
				args = append(args, cmd...)
				return args
			}
			args = append(args, "--")
			args = append(args, cmd...)
			return args
		},
	},
	"ghostty": {
		Binary: "ghostty",
		Args: func(appID, title string, w, h int, cmd []string) []string {
			args := []string{"--class=" + appID}
			if title != "" {
				args = append(args, "--title="+title)
			}
			if w > 0 && h > 0 {
				args = append(args, fmt.Sprintf("--window-width=%d", w), fmt.Sprintf("--window-height=%d", h))
			}
			args = append(args, "-e")
			args = append(args, cmd...)
			return args
		},
	},
	"xterm": {
		Binary: "xterm",
		Args: func(appID, title string, w, h int, cmd []string) []string {
			args := []string{"-class", appID}
			if title != "" {
				args = append(args, "-title", title)
			}
			if len(cmd) > 0 {
				args = append(args, "-e")
				args = append(args, cmd...)
			}
			return args
		},
	},
	"st": {
		Binary: "st",
		Args: func(appID, title string, w, h int, cmd []string) []string {
			args := []string{"-c", appID}
			if title != "" {
				args = append(args, "-t", title)
			}
			if len(cmd) > 0 {
				args = append(args, "-e")
				args = append(args, cmd...)
			}
			return args
		},
	},
}

// PreferredTerminal devuelve el terminal configurado si existe, o el primero
// disponible del sistema.
func PreferredTerminal(preferred string) string {
	if preferred != "" {
		if _, ok := terminals[preferred]; ok && HasCommand(preferred) {
			return preferred
		}
		if HasCommand(preferred) {
			return preferred
		}
	}
	for _, name := range []string{"foot", "kitty", "alacritty", "wezterm", "ghostty", "xterm", "st"} {
		if HasCommand(name) {
			return name
		}
	}
	return ""
}

// HasCommand indica si el binario está en PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// TerminalCommand construye el ejecutable y los argumentos para abrir cmd en
// una ventana del terminal seleccionado.
func TerminalCommand(term, appID, title string, width, height int, cmd []string) (string, []string, error) {
	if term == "" {
		term = PreferredTerminal("")
	}
	if term == "" {
		return "", nil, fmt.Errorf("no se encontró ningún emulador de terminal (instala foot, kitty o alacritty)")
	}
	spec, ok := terminals[term]
	if !ok {
		return "", nil, fmt.Errorf("terminal no soportado: %s", term)
	}
	binary, err := exec.LookPath(spec.Binary)
	if err != nil {
		return "", nil, fmt.Errorf("comando no encontrado: %s", spec.Binary)
	}
	return binary, spec.Args(appID, title, width, height, cmd), nil
}

// LaunchTerminal abre una ventana de terminal ejecutando cmd.
func LaunchTerminal(term, appID, title string, width, height int, cmd []string) error {
	binary, args, err := TerminalCommand(term, appID, title, width, height, cmd)
	if err != nil {
		return err
	}
	return StartDetached(binary, args...)
}

// RelaunchInTerminalIfNeeded relanza el ejecutable actual en una terminal
// flotante cuando el llamador no tiene TTY (por ejemplo, un atajo de sway).
// args son los argumentos originales que debe recibir el proceso relanzado.
func RelaunchInTerminalIfNeeded(interactive bool, term, appID, title string, width, height int, args []string) (bool, error) {
	if interactive {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return true, fmt.Errorf("obteniendo la ruta del ejecutable: %w", err)
	}
	command := append([]string{exe}, args...)
	return true, LaunchTerminal(term, appID, title, width, height, command)
}

// ShellQuoteArgs convierte argumentos en una cadena segura para /bin/sh, por
// ejemplo al enviarlos al comando `exec` de sway.
func ShellQuoteArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
}

// StartDetached lanza un programa en su propia sesión, sin heredar la
// terminal actual, para que sobreviva al cierre del menú.
func StartDetached(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("comando no encontrado: %s", name)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// StartInTerminal ejecuta un comando dentro de una terminal nueva.
func StartInTerminal(term string, cmd []string) error {
	return LaunchTerminal(term, "swaydesk-run", strings.Join(cmd, " "), 0, 0, cmd)
}

// ExecCommand lanza un comando sustituyendo al proceso actual (útil para
// lanzar desde un menú y no dejar procesos zombis).
func ExecCommand(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("comando no encontrado: %s", name)
	}
	return syscall.Exec(path, append([]string{name}, args...), os.Environ())
}
