// swaynotify es el daemon de notificaciones de swaydesk: implementa
// org.freedesktop.Notifications sobre una conexión D-Bus propia (sin libdbus)
// y dibuja las notificaciones como tarjetas en una superficie de
// wlr-layer-shell, píxel a píxel, con el tema activo.
package main

import (
	"fmt"
	"os"
	"strings"
)

const Version = "1.0.0"

func main() {
	args := os.Args[1:]
	cmd := "daemon"
	if len(args) > 0 {
		if strings.HasPrefix(args[0], "-") {
			// Opciones sueltas: --help, --version, --out… se tratan como la
			// orden equivalente sin guiones (y si no, arrancan el daemon).
			switch strings.TrimLeft(args[0], "-") {
			case "help", "h", "version", "v":
				cmd = strings.TrimLeft(args[0], "-")
				args = args[1:]
			}
		} else {
			cmd = args[0]
			args = args[1:]
		}
	}
	switch cmd {
	case "daemon", "run", "serve":
		runDaemon(args)
	case "send", "notify":
		runSend(args)
	case "dnd":
		runDND(args)
	case "status":
		runStatus(args)
	case "history", "list":
		runHistory(args)
	case "clear":
		runClear(args)
	case "reply", "responder":
		runReply(args)
	case "preview":
		runPreview(args)
	case "version", "--version", "-v":
		fmt.Printf("swaynotify %s (swaydesk)\n", Version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "swaynotify: orden desconocida %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `swaynotify · notificaciones de escritorio para sway, en Go

Uso: swaynotify <orden> [opciones]

  daemon                     arranca el servidor de notificaciones
  send [opciones] RESUMEN [CUERPO]
                             envía una notificación
  dnd [on|off|toggle]        modo «no molestar»
  status                     estado del daemon en JSON
  history [--json]           historial de notificaciones
  clear [--all]              cierra las visibles (--all también el historial)
  reply <id> [texto]         responde a una notificación (respuesta rápida)
  preview --out RUTA         PNG de ejemplo con las tarjetas

Opciones de send
  --app NOMBRE               aplicación que envía (por defecto swaydesk)
  --icon NOMBRE              icono (nombre o ruta)
  --reply                    añade la acción de responder (inline-reply)
  --reply-hint TEXTO         texto de ayuda del campo de respuesta
  --resident                 la tarjeta no se cierra al responder
  --group ETIQUETA           pila propia dentro de la misma aplicación
  --image RUTA               imagen (PNG o JPEG) dentro de la tarjeta
  --urgency baja|normal|urgente
  --timeout MS               milisegundos en pantalla (-1 = no caducar)
  --action clave=etiqueta    botón (se puede repetir)
  --category NOMBRE          categoría (transfer, device, im…)

El daemon implementa org.freedesktop.Notifications, así que cualquier
aplicación (Chromium, mpd, pacman…) muestra sus avisos sin configurar nada.
Sin daemon, "send" recurre a notify-send si está instalado.
`)
}

// parseFlags interpreta opciones con valor (--clave valor o --clave=valor).
// Devuelve el mapa y los argumentos posicionales.
func parseFlags(args []string, repeat map[string]bool) (map[string][]string, []string) {
	vals := map[string][]string{}
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		value := "true"
		if j := strings.IndexByte(name, '='); j >= 0 {
			name, value = name[:j], name[j+1:]
		} else if i+1 < len(args) && takesValue(name) {
			value = args[i+1]
			i++
		}
		if repeat[name] {
			vals[name] = append(vals[name], value)
		} else {
			vals[name] = []string{value}
		}
	}
	return vals, pos
}

// takesValue indica si la opción consume el argumento siguiente. Se enumeran
// las opciones con valor para que un resumen que empiece por "-" (o un valor
// negativo como -1) no se confunda con una opción.
func takesValue(name string) bool {
	switch name {
	case "app", "icon", "urgency", "timeout", "action", "category", "theme",
		"scale", "out", "body", "reply-hint", "group", "image":
		return true
	}
	return false
}

// last devuelve el último valor de una opción.
func last(vals map[string][]string, name string) string {
	if v, ok := vals[name]; ok && len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}
