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

	addrFlag := flag.String("addr", defaultAddr, "Dirección TCP donde escucha Kitsune Go Wayland (ej. 0.0.0.0:8080)")
	workDirFlag := flag.String("workdir", wd, "Directorio raíz para Yazi File Manager, Neovim Go Studio y Terminal")
	cfgFlag := flag.String("config", defaultCfg, "Ruta del archivo JSON de estado persistente")
	themeFlag := flag.String("theme", "", "Tema anime inicial opcional (neon-tokyo, sakura-lofi, crimson-shonen, eva-midnight)")
	layoutFlag := flag.String("layout", "", "Modo de escritorio inicial (tabbed por defecto | tiling | floating | monocle)")
	kioskFlag := flag.Bool("kiosk", false, "Lanzar sesión gráfica nativa Wayland en pantalla completa en Arch Linux")
	flag.Parse()

	store := config.NewStore(*cfgFlag)
	if *layoutFlag != "" {
		if _, err := store.SetLayout(strings.ToLower(*layoutFlag)); err != nil {
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
		log.Fatalf("error inicializando servidor Kitsune Go Wayland: %v", err)
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
	st := store.Settings()
	fmt.Printf("\n╭──────────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│  🦊 Kitsune Go Wayland v2.0.0 // Escritorio Anime Tabbed Puro     │\n")
	fmt.Printf("├──────────────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│  Escuchando en : http://%s\n", *addrFlag)
	fmt.Printf("│  Motor         : Kitsune Go Wayland (Sin Sway / Sin Dock inferior)\n")
	fmt.Printf("│  Tema Activo   : %s (%s)\n", theme.Name, theme.JapaneseName)
	fmt.Printf("│  Modo Ventanas : %s • Blur %dpx • Glow %d%%\n", strings.ToUpper(st.LayoutMode), st.BlurRadius, st.GlowIntensity)
	fmt.Printf("╰──────────────────────────────────────────────────────────────────╯\n\n")

	if *kioskFlag {
		go launchNativeWaylandShell(*addrFlag)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("Cerrando sesión de Kitsune Go Wayland limpiamente...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutCtx)
	case err := <-errCh:
		log.Fatalf("error fatal en servidor HTTP: %v", err)
	}
}

func launchNativeWaylandShell(listenAddr string) {
	time.Sleep(450 * time.Millisecond)
	port := "8080"
	if idx := strings.LastIndex(listenAddr, ":"); idx >= 0 {
		port = listenAddr[idx+1:]
	}
	localURL := "http://127.0.0.1:" + port

	candidates := [][]string{
		{"chromium", "--ozone-platform-hint=auto", "--autoplay-policy=no-user-gesture-required", "--kiosk", "--app=" + localURL, "--no-first-run"},
		{"brave", "--ozone-platform-hint=auto", "--kiosk", "--app=" + localURL, "--no-first-run"},
		{"firefox", "--kiosk", localURL},
		{"xdg-open", localURL},
	}

	for _, args := range candidates {
		bin, err := exec.LookPath(args[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(bin, args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err == nil {
			return
		}
	}
}
