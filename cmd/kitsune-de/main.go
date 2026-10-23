package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
	"github.com/ShadowNigth/Jonatan-1999/internal/server"
)

func main() {
	defaultAddr := "0.0.0.0:8080"
	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		if strings.Contains(p, ":") {
			defaultAddr = p
		} else {
			defaultAddr = "0.0.0.0:" + p
		}
	}

	homeDir, _ := os.UserHomeDir()
	defaultCfg := ""
	if homeDir != "" {
		defaultCfg = filepath.Join(homeDir, ".config", "kitsune-de", "state.json")
	}
	wd, _ := os.Getwd()

	addrFlag := flag.String("addr", defaultAddr, "Dirección TCP donde escucha KitsuneDE (ej. 0.0.0.0:8080)")
	workDirFlag := flag.String("workdir", wd, "Directorio raíz para el explorador de archivos y terminal")
	cfgFlag := flag.String("config", defaultCfg, "Ruta del archivo JSON de estado persistente")
	themeFlag := flag.String("theme", "", "Tema anime inicial opcional (neon-tokyo, sakura-lofi, crimson-shonen, eva-midnight)")
	compFlag := flag.String("compositor", "", "Perfil de compositor inicial (swayfx, sway, hyprland)")
	kioskFlag := flag.Bool("kiosk", false, "Lanzar sesión gráfica nativa en pantalla completa en Arch Linux (Wayland/X11)")
	flag.Parse()

	store := config.NewStore(*cfgFlag)
	if *compFlag != "" {
		if _, err := store.SetCompositor(strings.ToLower(*compFlag)); err != nil {
			log.Printf("[aviso] %v", err)
		}
	}
	if *themeFlag != "" {
		if _, _, err := store.SetTheme(*themeFlag); err != nil {
			log.Printf("[aviso] %v", err)
		}
	}

	srv, err := server.New(store, *workDirFlag)
	if err != nil {
		log.Fatalf("error inicializando servidor KitsuneDE: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv.StartTelemetryLoop(ctx)

	httpServer := &http.Server{
		Addr:              *addrFlag,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	theme := store.ActiveTheme()
	fmt.Printf("\n╭──────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│  🦊 KitsuneDE // Arch Linux Sway & SwayFX Anime Engine (Go)  │\n")
	fmt.Printf("├──────────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│  Escuchando en : http://%s\n", *addrFlag)
	fmt.Printf("│  Compositor    : %s\n", strings.ToUpper(store.Settings().Compositor))
	fmt.Printf("│  Tema Activo   : %s (%s)\n", theme.Name, theme.JapaneseName)
	fmt.Printf("│  Modo Ventanas : %s (Tiling / Tabbed / Flotante)\n", strings.ToUpper(store.Settings().LayoutMode))
	fmt.Printf("╰──────────────────────────────────────────────────────────────╯\n\n")

	if *kioskFlag {
		go launchNativeKiosk(*addrFlag)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("Cerrando sesión de KitsuneDE limpiamente...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutCtx)
	case err := <-errCh:
		log.Fatalf("error fatal en servidor HTTP: %v", err)
	}
}

// launchNativeKiosk opens KitsuneDE as a fullscreen native desktop shell on Arch Linux (Wayland/X11).
func launchNativeKiosk(listenAddr string) {
	time.Sleep(450 * time.Millisecond)
	port := "8080"
	if idx := strings.LastIndex(listenAddr, ":"); idx >= 0 {
		port = listenAddr[idx+1:]
	}
	localURL := "http://127.0.0.1:" + port

	candidates := [][]string{
		{"chromium", "--kiosk", "--app=" + localURL, "--no-first-run"},
		{"brave", "--kiosk", "--app=" + localURL, "--no-first-run"},
		{"google-chrome-stable", "--kiosk", "--app=" + localURL},
		{"firefox", "--kiosk", localURL},
		{"xdg-open", localURL},
	}

	// If running from TTY without compositor and `cage` is installed on Arch Linux, wrap browser inside Cage Wayland kiosk
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		if cagePath, err := exec.LookPath("cage"); err == nil {
			for _, browser := range []string{"chromium", "firefox"} {
				if bPath, err2 := exec.LookPath(browser); err2 == nil {
					cmd := exec.Command(cagePath, "-s", "--", bPath, "--kiosk", localURL)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					_ = cmd.Run()
					return
				}
			}
		}
	}

	for _, c := range candidates {
		if bin, err := exec.LookPath(c[0]); err == nil {
			cmd := exec.Command(bin, c[1:]...)
			_ = cmd.Start()
			return
		}
	}
	log.Printf("[kiosk] No se detectó navegador local; abre %s manualmente", localURL)
}
