package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
	"github.com/ShadowNigth/Jonatan-1999/internal/ricer"
)

func main() {
	baseURL := strings.TrimRight(os.Getenv("KITSUNE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}

	args := os.Args[1:]
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return
	}

	switch args[0] {
	case "status":
		resp, err := sendCtl(baseURL, "status", "", "")
		if err != nil {
			fatalErr(err)
		}
		fmt.Println(prettyJSON(resp))

	case "compositor", "comp":
		val := "toggle"
		if len(args) >= 2 {
			val = strings.ToLower(args[1])
		}
		resp, err := sendCtl(baseURL, "set_compositor", val, "")
		if err != nil {
			fatalErr(err)
		}
		fmt.Println("✓ Compositor actualizado:")
		fmt.Println(prettyJSON(resp))

	case "theme":
		if len(args) < 2 || args[1] == "list" {
			store := config.NewStore("")
			fmt.Println("🦊 Temas Anime incluidos en KitsuneDE (Sway / SwayFX / Hyprland):")
			for _, t := range store.Themes() {
				fmt.Printf("  • %-16s  %-18s  %s\n", t.ID, t.JapaneseName, t.Name)
			}
			return
		}
		if args[1] == "next" {
			resp, err := sendCtl(baseURL, "next_theme", "", "")
			if err != nil {
				fatalErr(err)
			}
			fmt.Println("✓ Tema cambiado en vivo:")
			fmt.Println(prettyJSON(resp))
			return
		}
		if args[1] == "set" && len(args) >= 3 {
			resp, err := sendCtl(baseURL, "set_theme", args[2], "")
			if err != nil {
				fatalErr(err)
			}
			fmt.Println("✓ Tema aplicado:")
			fmt.Println(prettyJSON(resp))
			return
		}
		fatalMsg("Uso: kitsune-ctl theme [list | next | set <id>]")

	case "layout":
		val := "toggle"
		if len(args) >= 2 {
			val = args[1]
		}
		resp, err := sendCtl(baseURL, "set_layout", val, "")
		if err != nil {
			fatalErr(err)
		}
		fmt.Println("✓ Modo de ventanas actualizado:")
		fmt.Println(prettyJSON(resp))

	case "workspace", "ws":
		if len(args) < 2 {
			fatalMsg("Uso: kitsune-ctl workspace <1..5>")
		}
		resp, err := sendCtl(baseURL, "set_workspace", args[1], "")
		if err != nil {
			fatalErr(err)
		}
		fmt.Println("✓ Workspace activo actualizado:")
		fmt.Println(prettyJSON(resp))

	case "notify":
		if len(args) < 2 {
			fatalMsg("Uso: kitsune-ctl notify <mensaje>")
		}
		msg := strings.Join(args[1:], " ")
		if _, err := sendCtl(baseURL, "notify", msg, "狐 Kitsune SwayNC"); err != nil {
			fatalErr(err)
		}
		fmt.Printf("✓ Notificación enviada: %s\n", msg)

	case "open":
		if len(args) < 2 {
			fatalMsg("Uso: kitsune-ctl open <terminal|sysmon|files|settings|music|pacman|nvim>")
		}
		if _, err := sendCtl(baseURL, "open_app", args[1], ""); err != nil {
			fatalErr(err)
		}
		fmt.Printf("✓ Abriendo aplicación en escritorio: %s\n", args[1])

	case "export":
		themeID := "neon-tokyo"
		compositor := "swayfx"
		outDir := "./dotfiles-sway"
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--theme", "-t":
				if i+1 < len(args) {
					themeID = args[i+1]
					i++
				}
			case "--compositor", "-c":
				if i+1 < len(args) {
					compositor = strings.ToLower(args[i+1])
					i++
				}
			case "--out", "-o":
				if i+1 < len(args) {
					outDir = args[i+1]
					i++
				}
			default:
				if !strings.HasPrefix(args[i], "-") {
					outDir = args[i]
				}
			}
		}
		store := config.NewStore("")
		theme, ok := store.GetTheme(themeID)
		if !ok {
			fatalMsg("Tema desconocido: " + themeID)
		}
		settings := store.Settings()
		if compositor == "swayfx" || compositor == "sway" || compositor == "hyprland" {
			settings.Compositor = compositor
		}
		written, err := ricer.ExportToDir(outDir, theme, settings)
		if err != nil {
			fatalErr(err)
		}
		fmt.Printf("✓ Exportados %d archivos de configuración Arch Linux [%s • %s] en %s:\n",
			len(written), strings.ToUpper(settings.Compositor), theme.Name, outDir)
		for _, f := range written {
			fmt.Println("  •", f)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func sendCtl(baseURL, action, value, title string) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{
		"action": action,
		"value":  value,
		"title":  title,
	})
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Post(baseURL+"/api/ctl", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con kitsune-de en %s (%v)", baseURL, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func prettyJSON(raw []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}

func printUsage() {
	fmt.Println(`🦊 kitsune-ctl — Controlador IPC y Generador de Rice Sway / SwayFX / Arch Linux (Go)

Uso:
  kitsune-ctl status                                                Estado actual del escritorio y telemetría
  kitsune-ctl compositor <swayfx|sway|hyprland|toggle>              Cambiar perfil de compositor activo
  kitsune-ctl theme [list | next | set <id>]                        Gestionar temas anime incluidos
  kitsune-ctl layout <tiling|tabbed|floating|monocle|toggle>        Cambiar modo de contenedores Sway
  kitsune-ctl workspace <1..5>                                      Cambiar al workspace indicado
  kitsune-ctl open <terminal|sysmon|files|settings|music|pacman|nvim>
  kitsune-ctl notify "<mensaje>"                                    Mostrar notificación anime estilo SwayNC
  kitsune-ctl export [--compositor swayfx|sway] [--theme <id>] [--out <dir>]`)
}

func fatalErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func fatalMsg(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
