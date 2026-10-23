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

	case "theme":
		if len(args) < 2 || args[1] == "list" {
			store := config.NewStore("")
			fmt.Println("🦊 Temas Anime incluidos en Kitsune Go Wayland:")
			for _, t := range store.Themes() {
				fmt.Printf("  • %-16s  %-18s  %s — %s\n", t.ID, t.JapaneseName, t.Name, t.Subtitle)
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
		fmt.Println("✓ Modo de escritorio actualizado:")
		fmt.Println(prettyJSON(resp))

	case "blur":
		val := "toggle"
		if len(args) >= 2 {
			val = args[1]
		}
		resp, err := sendCtl(baseURL, "set_blur", val, "")
		if err != nil {
			fatalErr(err)
		}
		fmt.Println("✓ Desenfoque Blur actualizado en vivo:")
		fmt.Println(prettyJSON(resp))

	case "glow":
		val := "toggle"
		if len(args) >= 2 {
			val = args[1]
		}
		resp, err := sendCtl(baseURL, "set_glow", val, "")
		if err != nil {
			fatalErr(err)
		}
		fmt.Println("✓ Bordes luminosos actualizados en vivo:")
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
		if _, err := sendCtl(baseURL, "notify", msg, "狐 Kitsune Go Wayland"); err != nil {
			fatalErr(err)
		}
		fmt.Printf("✓ Notificación enviada: %s\n", msg)

	case "open":
		if len(args) < 2 {
			fatalMsg("Uso: kitsune-ctl open <settings|sysmon|files|nvim|terminal|music|pacman>")
		}
		if _, err := sendCtl(baseURL, "open_app", args[1], ""); err != nil {
			fatalErr(err)
		}
		fmt.Printf("✓ Abriendo pestaña en escritorio: %s\n", args[1])

	case "export":
		themeID := "neon-tokyo"
		outDir := "./dotfiles-kitsune-wayland"
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--theme", "-t":
				if i+1 < len(args) {
					themeID = args[i+1]
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
		written, err := ricer.ExportToDir(outDir, theme, store.Settings())
		if err != nil {
			fatalErr(err)
		}
		fmt.Printf("✓ Exportados %d archivos de Kitsune Go Wayland [%s] en %s:\n", len(written), theme.Name, outDir)
		for _, f := range written {
			fmt.Printf("  • %s\n", f)
		}

	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`🦊 kitsune-ctl — Utilidad CLI para Kitsune Go Wayland (Tabbed Desktop)

Uso:
  kitsune-ctl status                            Ver estado de Kitsune Go Wayland
  kitsune-ctl theme list                        Listar los 4 temas Anime incluidos
  kitsune-ctl theme next                        Cambiar al siguiente tema en caliente
  kitsune-ctl theme set <id>                    Aplicar neon-tokyo | sakura-lofi | crimson-shonen | eva-midnight
  kitsune-ctl layout [tabbed|tiling|floating]   Cambiar modo de ventanas (por defecto: tabbed)
  kitsune-ctl blur <0..36|toggle>               Ajustar intensidad de Blur en vivo (px)
  kitsune-ctl glow <0..100|toggle>              Ajustar brillo de bordes luminosos en vivo (%)
  kitsune-ctl workspace <1..5>                  Cambiar al workspace Kanji (一 二 三 四 五)
  kitsune-ctl open <app>                        Abrir pestaña (settings, sysmon, files, nvim, terminal)
  kitsune-ctl notify "<texto>"                  Enviar notificación al escritorio
  kitsune-ctl export --theme <id> --out <dir>   Exportar configuraciones Kitsune Go Wayland`)
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
		return nil, fmt.Errorf("no se pudo conectar a kitsune-de en %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("error del servidor (%d): %s", resp.StatusCode, string(data))
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

func fatalErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func fatalMsg(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
