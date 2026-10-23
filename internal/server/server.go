package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
	"github.com/ShadowNigth/Jonatan-1999/internal/fsapi"
	"github.com/ShadowNigth/Jonatan-1999/internal/ricer"
	"github.com/ShadowNigth/Jonatan-1999/internal/swayipc"
	"github.com/ShadowNigth/Jonatan-1999/internal/sysinfo"
	"github.com/ShadowNigth/Jonatan-1999/internal/terminal"
	"github.com/ShadowNigth/Jonatan-1999/internal/ui"
	"github.com/ShadowNigth/Jonatan-1999/internal/ws"
)

type ArchPackage struct {
	Repo        string `json:"repo"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Category    string `json:"category"`
	Installed   bool   `json:"installed"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

type Server struct {
	store     *config.Store
	collector *sysinfo.Collector
	sway      *swayipc.Client
	hub       *ws.Hub
	fsHandler *fsapi.Handler
	term      *terminal.Handler
	workDir   string
	mux       *http.ServeMux
}

func New(store *config.Store, workDir string) (*Server, error) {
	collector := sysinfo.NewCollector()
	hub := ws.NewHub()
	swayCli := swayipc.NewFromEnv()
	s := &Server{
		store:     store,
		collector: collector,
		sway:      swayCli,
		hub:       hub,
		fsHandler: fsapi.NewHandler(workDir),
		term:      terminal.NewHandler(store, collector, swayCli, hub, workDir),
		workDir:   workDir,
		mux:       http.NewServeMux(),
	}
	if err := s.registerRoutes(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) StartTelemetryLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap := s.collector.Collect()
				s.hub.Broadcast(ws.Event{
					Type:    "telemetry",
					Payload: snap,
				})
			}
		}
	}()
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) registerRoutes() error {
	staticFS, err := ui.StaticFS()
	if err != nil {
		return err
	}

	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":         "ok",
			"service":        "kitsune-de",
			"compositor":     s.store.Settings().Compositor,
			"swayIpcPresent": s.sway.Available(),
		})
	})

	s.mux.HandleFunc("/api/bootstrap", s.handleBootstrap)
	s.mux.HandleFunc("/api/system", s.handleSystem)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/ctl", s.handleCtl)
	s.mux.HandleFunc("/api/ricer/preview", s.handleRicerPreview)
	s.mux.HandleFunc("/api/ricer/export", s.handleRicerExport)
	s.mux.HandleFunc("/api/native/launch", s.handleNativeLaunch)

	s.mux.HandleFunc("/api/fs/list", s.fsHandler.HandleList)
	s.mux.HandleFunc("/api/fs/read", s.fsHandler.HandleRead)
	s.mux.HandleFunc("/api/fs/write", s.fsHandler.HandleWrite)

	serveReleaseZip := func(filename string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			zipPath := filepath.Join(s.workDir, "release", filename)
			if _, err := os.Stat(zipPath); err != nil {
				zipPath = filepath.Join(s.workDir, "release", "kitsune-de-sway-arch-linux.zip")
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeFile(w, r, zipPath)
		}
	}
	s.mux.HandleFunc("/download/kitsune-de-sway-arch-linux.zip", serveReleaseZip("kitsune-de-sway-arch-linux.zip"))
	s.mux.HandleFunc("/download/kitsune-de-arch-linux.zip", serveReleaseZip("kitsune-de-arch-linux.zip"))

	s.mux.HandleFunc("/ws/events", s.handleEventsWS)
	s.mux.HandleFunc("/ws/terminal", s.handleTerminalWS)

	fileServer := http.FileServer(http.FS(staticFS))
	s.mux.Handle("/", fileServer)
	return nil
}

func (s *Server) syncLiveCompositor() {
	if s.sway.Available() {
		s.sway.SyncNativeDesktop(s.store.ActiveTheme(), s.store.Settings())
	}
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":       s.store.Settings(),
		"activeTheme":    s.store.ActiveTheme(),
		"themes":         s.store.Themes(),
		"system":         s.collector.Collect(),
		"swayIpcPresent": s.sway.Available(),
		"archPackages":   builtinArchCatalog(),
	})
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.collector.Collect())
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"settings": s.store.Settings(),
			"theme":    s.store.ActiveTheme(),
		})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "método no permitido"})
		return
	}

	cur := s.store.Settings()
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&cur); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "JSON inválido"})
		return
	}
	updated, err := s.store.UpdateSettings(cur)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	s.syncLiveCompositor()
	theme := s.store.ActiveTheme()
	payload := map[string]any{"settings": updated, "theme": theme}
	s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleCtl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "usa POST"})
		return
	}
	var req struct {
		Action string `json:"action"`
		Value  string `json:"value"`
		Title  string `json:"title"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32*1024)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "JSON inválido"})
		return
	}

	switch req.Action {
	case "status":
		writeJSON(w, http.StatusOK, map[string]any{
			"settings":       s.store.Settings(),
			"theme":          s.store.ActiveTheme(),
			"swayIpcPresent": s.sway.Available(),
			"system":         s.collector.Collect(),
		})
	case "set_theme":
		settings, theme, err := s.store.SetTheme(req.Value)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		s.syncLiveCompositor()
		payload := map[string]any{"settings": settings, "theme": theme}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "next_theme":
		themes := s.store.Themes()
		cur := s.store.Settings().ActiveTheme
		nextID := themes[0].ID
		for i, t := range themes {
			if t.ID == cur {
				nextID = themes[(i+1)%len(themes)].ID
				break
			}
		}
		settings, theme, _ := s.store.SetTheme(nextID)
		s.syncLiveCompositor()
		payload := map[string]any{"settings": settings, "theme": theme}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "set_compositor":
		comp := req.Value
		if comp == "" {
			comp = "toggle"
		}
		settings, err := s.store.SetCompositor(comp)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		s.syncLiveCompositor()
		payload := map[string]any{"settings": settings, "theme": s.store.ActiveTheme()}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "set_layout":
		mode := req.Value
		if mode == "" {
			mode = "toggle"
		}
		settings, err := s.store.SetLayout(mode)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if s.sway.Available() && mode == "tabbed" {
			_, _ = s.sway.RunCommand("layout tabbed")
		} else if s.sway.Available() && mode == "tiling" {
			_, _ = s.sway.RunCommand("layout splith")
		}
		payload := map[string]any{"settings": settings, "theme": s.store.ActiveTheme()}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "set_workspace":
		n, _ := strconv.Atoi(req.Value)
		settings, err := s.store.SetWorkspace(n)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if s.sway.Available() {
			_, _ = s.sway.RunCommand("workspace number " + strconv.Itoa(n))
		}
		payload := map[string]any{"settings": settings, "theme": s.store.ActiveTheme()}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "notify":
		title := req.Title
		if title == "" {
			title = "狐 KitsuneDE Sway IPC"
		}
		evtPayload := map[string]any{"title": title, "body": req.Value, "icon": "sparkles"}
		s.hub.Broadcast(ws.Event{Type: "notification", Payload: evtPayload})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case "open_app":
		s.hub.Broadcast(ws.Event{Type: "open_app", Payload: map[string]any{"appId": req.Value}})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "acción desconocida: " + req.Action})
	}
}

func (s *Server) handleRicerPreview(w http.ResponseWriter, r *http.Request) {
	themeID := strings.TrimSpace(r.URL.Query().Get("theme"))
	comp := strings.TrimSpace(r.URL.Query().Get("compositor"))
	theme := s.store.ActiveTheme()
	if themeID != "" {
		if t, ok := s.store.GetTheme(themeID); ok {
			theme = t
		}
	}
	cfg := s.store.Settings()
	if comp == "swayfx" || comp == "sway" || comp == "hyprland" {
		cfg.Compositor = comp
	}
	bundle := ricer.Generate(theme, cfg)
	writeJSON(w, http.StatusOK, bundle)
}

func (s *Server) handleRicerExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "usa POST"})
		return
	}
	var req struct {
		ThemeID    string `json:"themeId"`
		Compositor string `json:"compositor"`
		OutDir     string `json:"outDir"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 32*1024)).Decode(&req)
	theme := s.store.ActiveTheme()
	if req.ThemeID != "" {
		if t, ok := s.store.GetTheme(req.ThemeID); ok {
			theme = t
		}
	}
	cfg := s.store.Settings()
	if req.Compositor == "swayfx" || req.Compositor == "sway" || req.Compositor == "hyprland" {
		cfg.Compositor = req.Compositor
	}
	outRel := strings.TrimSpace(req.OutDir)
	if outRel == "" || strings.Contains(outRel, "..") || filepath.IsAbs(outRel) {
		outRel = "dotfiles-sway-export"
	}
	target := filepath.Join(s.workDir, outRel)
	files, err := ricer.ExportToDir(target, theme, cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	s.hub.Broadcast(ws.Event{
		Type: "notification",
		Payload: map[string]any{
			"title": "Dotfiles Sway/SwayFX Generados",
			"body":  "Exportados en ./" + outRel + " (" + strings.ToUpper(cfg.Compositor) + " • " + theme.Name + ")",
			"icon":  "check",
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"compositor": cfg.Compositor,
		"outDir":     outRel,
		"files":      files,
	})
}

func (s *Server) handleEventsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.Upgrade(w, r)
	if err != nil {
		return
	}
	s.hub.Register(conn)
	defer s.hub.Unregister(conn)

	_ = conn.WriteJSON(ws.Event{
		Type: "welcome",
		Payload: map[string]any{
			"settings": s.store.Settings(),
			"theme":    s.store.ActiveTheme(),
		},
	})

	for {
		if _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.Upgrade(w, r)
	if err != nil {
		return
	}
	s.term.ServeWS(conn)
}

func (s *Server) handleNativeLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		App string `json:"app"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	allowed := map[string]bool{
		"kitty":       true,
		"thunar":      true,
		"firefox":     true,
		"chromium":    true,
		"pavucontrol": true,
		"nwg-look":    true,
	}
	appName := strings.TrimSpace(req.App)
	if !allowed[appName] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "aplicación nativa no permitida"})
		return
	}
	binPath, err := exec.LookPath(appName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": appName + " no instalado en el host actual"})
		return
	}
	cmd := exec.Command(binPath)
	if err := cmd.Start(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	go func() { _ = cmd.Wait() }()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "launched": appName})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func builtinArchCatalog() []ArchPackage {
	return []ArchPackage{
		{Repo: "kitsune", Name: "kitsune-de", Version: "1.2.0-1", Category: "Desktop & Sway IPC", Installed: true, Description: "Escritorio Full Anime Shell + demonio sway-ipc en Go idéntico al ejemplo web", Command: "makepkg -si (en ./arch)"},
		{Repo: "aur", Name: "swayfx", Version: "0.4.0-2", Category: "Compositor Anime", Installed: true, Description: "Fork de Sway con esquinas redondeadas (corner_radius), blur, shadow_color y layer_effects", Command: "yay -S swayfx"},
		{Repo: "extra", Name: "nwg-dock", Version: "0.4.2-1", Category: "Dock Flotante", Installed: true, Description: "Dock inferior flotante para Sway/SwayFX con blur cristalino y glow neón estilo Web Preview", Command: "sudo pacman -S nwg-dock"},
		{Repo: "extra", Name: "swww", Version: "0.9.5-1", Category: "Animaciones 60fps", Installed: true, Description: "Transiciones circulares animadas a 60 FPS al cambiar entre los 4 temas anime", Command: "sudo pacman -S swww"},
		{Repo: "extra", Name: "waybar", Version: "0.11.0-2", Category: "Barra Superior", Installed: true, Description: "Barra cristalina con píldoras Kanji (一 二 三 四 五), telemetría, layout y selector de tema", Command: "sudo pacman -S waybar"},
		{Repo: "extra", Name: "cage", Version: "0.2.0-1", Category: "Sesión Web-Nativa", Installed: true, Description: "Compositor Wayland kiosco GPU para correr KitsuneDE Full Anime Shell 100% píxel-idéntico", Command: "sudo pacman -S cage chromium"},
		{Repo: "aur", Name: "rofi-wayland", Version: "1.7.5-3", Category: "App Launcher", Installed: true, Description: "Lanzador drun centrado con 2 columnas, íconos Papirus-Dark y layer_effects blur", Command: "yay -S rofi-wayland"},
		{Repo: "aur", Name: "swaync", Version: "0.10.1-2", Category: "Notificaciones", Installed: true, Description: "Centro de notificaciones Wayland con tarjetas glassmorphism y hot-reload de colores", Command: "yay -S swaync"},
		{Repo: "aur", Name: "swaylock-effects-git", Version: "1.7.0-1", Category: "Bloqueo Anime", Installed: true, Description: "Pantalla de bloqueo con blur dinámico, reloj central e indicador circular neón", Command: "yay -S swaylock-effects-git"},
		{Repo: "extra", Name: "kitty", Version: "0.38.1-1", Category: "Terminal GPU", Installed: true, Description: "Terminal GPU con recarga en vivo SIGUSR1 de paleta ANSI al cambiar de tema", Command: "sudo pacman -S kitty fastfetch"},
		{Repo: "extra", Name: "noto-fonts-cjk", Version: "20230817-2", Category: "Tipografía Kanji", Installed: true, Description: "Fuentes japonesas Kanji + Noto Color Emoji + JetBrainsMono Nerd Font para cero cuadros vacíos", Command: "sudo pacman -S noto-fonts-cjk noto-fonts-emoji ttf-jetbrains-mono-nerd inter-font"},
		{Repo: "aur", Name: "bibata-cursor-theme-bin", Version: "2.0.7-1", Category: "Iconos & GTK", Installed: true, Description: "Cursor Bibata-Modern-Ice + iconos Papirus-Dark + temas oscuros GTK3/GTK4", Command: "yay -S bibata-cursor-theme-bin nwg-look && sudo pacman -S papirus-icon-theme"},
	}
}
