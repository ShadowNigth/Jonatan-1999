package setup

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

// dialSway abre una conexión IPC con la sesión de sway actual.
func dialSway() (*swayipc.Conn, error) {
	return swayipc.Dial()
}

// ─── Comprobaciones de configuración instalada ──────────────────────────────

// FootConfigIssue revisa el foot.ini instalado y devuelve una descripción del
// problema, o "" si está bien. Detecta las dos roturas típicas al actualizar
// foot: la sección [colors] que desapareció en 1.28 y los colores con '#'.
func FootConfigIssue() string {
	path := filepath.Join(configDir(), "foot", "foot.ini")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	texto := string(data)
	seccion := footColorsSection()
	if strings.Contains(texto, "\n[colors]\n") && seccion == "[colors-dark]" {
		return "usa [colors] y este foot pide [colors-dark]"
	}
	for _, line := range strings.Split(texto, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		partes := strings.SplitN(line, "=", 2)
		if len(partes) != 2 {
			continue
		}
		clave := strings.TrimSpace(partes[0])
		if !isFootColorKey(clave) {
			continue
		}
		if strings.Contains(partes[1], "#") {
			return clave + " lleva '#' y foot no lo acepta"
		}
	}
	return ""
}

func isFootColorKey(k string) bool {
	switch k {
	case "background", "foreground", "cursor", "selection-background", "selection-foreground":
		return true
	}
	return strings.HasPrefix(k, "regular") || strings.HasPrefix(k, "bright")
}

// SwayDuplicateBinding devuelve la primera tecla asignada dos veces en la
// configuración de sway instalada ("" si no hay duplicados). Con duplicados
// sway se queda con la última y el usuario pierde un atajo en silencio.
func SwayDuplicateBinding() string {
	data, err := os.ReadFile(filepath.Join(configDir(), "sway", "config"))
	if err != nil {
		return ""
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "bindsym") {
			continue
		}
		campos := strings.Fields(line)
		if len(campos) < 3 {
			continue
		}
		if seen[campos[1]] {
			return campos[1]
		}
		seen[campos[1]] = true
	}
	return ""
}

// configDir devuelve el directorio de configuración XDG.
func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}
