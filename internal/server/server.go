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
	hub       *ws.Hub
	fsHandler *fsapi.Handler
	term      *terminal.Handler
	workDir   string
	mux       *http.ServeMux
}

func New(store *config.Store, workDir string) (*Server, error) {
	collector := sysinfo.NewCollector()
	hub := ws.NewHub()
	s := &Server{
		store:     store,
		collector: collector,
		hub:       hub,
		fsHandler: fsapi.NewHandler(workDir),
		term:      terminal.NewHandler(store, collector, hub, workDir),
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
			"status":     "ok",
			"service":    "kitsune-go-wayland",
			"engine":     "Kitsune Go Wayland",
			"layoutMode": s.store.Settings().LayoutMode,
			"dock":       false,
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
				zipPath = filepath.Join(s.workDir, "release", "kitsune-go-wayland-arch-linux.zip")
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeFile(w, r, zipPath)
		}
	}
	s.mux.HandleFunc("/download/kitsune-go-wayland-arch-linux.zip", serveReleaseZip("kitsune-go-wayland-arch-linux.zip"))

	s.mux.HandleFunc("/ws/events", s.handleEventsWS)
	s.mux.HandleFunc("/ws/terminal", s.handleTerminalWS)

	fileServer := http.FileServer(http.FS(staticFS))
	s.mux.Handle("/", fileServer)
	return nil
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"engine":       "Kitsune Go Wayland",
		"settings":     s.store.Settings(),
		"activeTheme":  s.store.ActiveTheme(),
		"themes":       s.store.Themes(),
		"system":       s.collector.Collect(),
		"archPackages": builtinArchCatalog(),
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
			"engine":   "Kitsune Go Wayland",
			"settings": s.store.Settings(),
			"theme":    s.store.ActiveTheme(),
			"system":   s.collector.Collect(),
		})
	case "set_theme":
		settings, theme, err := s.store.SetTheme(req.Value)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
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
		payload := map[string]any{"settings": settings, "theme": theme}
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
		payload := map[string]any{"settings": settings, "theme": s.store.ActiveTheme()}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "set_blur":
		cur := s.store.Settings()
		if req.Value == "toggle" || req.Value == "" {
			cur.BlurEnabled = !cur.BlurEnabled
		} else if px, err := strconv.Atoi(req.Value); err == nil {
			cur.BlurEnabled = px > 0
			cur.BlurRadius = px
		}
		settings, _ := s.store.UpdateSettings(cur)
		payload := map[string]any{"settings": settings, "theme": s.store.ActiveTheme()}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "set_glow":
		cur := s.store.Settings()
		if req.Value == "toggle" || req.Value == "" {
			cur.BorderGlow = !cur.BorderGlow
		} else if val, err := strconv.Atoi(req.Value); err == nil {
			cur.BorderGlow = val > 0
			cur.GlowIntensity = val
		}
		settings, _ := s.store.UpdateSettings(cur)
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
		payload := map[string]any{"settings": settings, "theme": s.store.ActiveTheme()}
		s.hub.Broadcast(ws.Event{Type: "state_updated", Payload: payload})
		writeJSON(w, http.StatusOK, payload)

	case "notify":
		title := req.Title
		if title == "" {
			title = "狐 Kitsune Go Wayland"
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
	theme := s.store.ActiveTheme()
	if themeID != "" {
		if t, ok := s.store.GetTheme(themeID); ok {
			theme = t
		}
	}
	bundle := ricer.Generate(theme, s.store.Settings())
	writeJSON(w, http.StatusOK, bundle)
}

func (s *Server) handleRicerExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "usa POST"})
		return
	}
	var req struct {
		ThemeID string `json:"themeId"`
		OutDir  string `json:"outDir"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 32*1024)).Decode(&req)
	theme := s.store.ActiveTheme()
	if req.ThemeID != "" {
		if t, ok := s.store.GetTheme(req.ThemeID); ok {
			theme = t
		}
	}
	outRel := strings.TrimSpace(req.OutDir)
	if outRel == "" || strings.Contains(outRel, "..") || filepath.IsAbs(outRel) {
		outRel = "dotfiles-kitsune-wayland"
	}
	target := filepath.Join(s.workDir, outRel)
	files, err := ricer.ExportToDir(target, theme, s.store.Settings())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	s.hub.Broadcast(ws.Event{
		Type: "notification",
		Payload: map[string]any{
			"title": "Dotfiles Kitsune Go Wayland Generados",
			"body":  "Exportados en ./" + outRel + " (" + theme.Name + ")",
			"icon":  "check",
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"engine": "kitsune-go-wayland",
		"outDir": outRel,
		"files":  files,
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
		"btop":        true,
		"yazi":        true,
		"nvim":        true,
		"pavucontrol": true,
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
		{Repo: "kitsune", Name: "kitsune-go-wayland", Version: "2.0.0-1", Category: "Escritorio Go Wayland", Installed: true, Description: "Escritorio Anime Tabbed puro en Go para Wayland (Waybar + Rofi + Blur + Bordes luminosos, sin Sway/Hyprland ni Dock)", Command: "makepkg -si (en ./arch)"},
		{Repo: "extra", Name: "cage", Version: "0.2.0-1", Category: "Servidor Wayland", Installed: true, Description: "Servidor de pantalla Wayland ligero para arrancar Kitsune Go Wayland directo desde SDDM/GDM/TTY", Command: "sudo pacman -S cage"},
		{Repo: "extra", Name: "chromium", Version: "131.0-1", Category: "Motor Gráfico GPU", Installed: true, Description: "Renderizador GPU Ozone Wayland para cristal blur en tiempo real, partículas 60fps y audio", Command: "sudo pacman -S chromium"},
		{Repo: "extra", Name: "btop", Version: "1.4.0-1", Category: "Btop++ Monitor", Installed: true, Description: "Monitor de recursos de Linux sincronizado con los 4 temas Anime de Kitsune Rice Studio", Command: "sudo pacman -S btop"},
		{Repo: "extra", Name: "yazi", Version: "0.4.2-1", Category: "Yazi File Manager", Installed: true, Description: "Gestor de archivos TUI ultrarrápido con vista previa y paleta Anime sincronizada", Command: "sudo pacman -S yazi"},
		{Repo: "extra", Name: "neovim", Version: "0.10.3-1", Category: "Neovim Go Studio", Installed: true, Description: "Editor modal para Go y Arch Linux con configuración Lua Anime autogenerada", Command: "sudo pacman -S neovim"},
		{Repo: "extra", Name: "kitty", Version: "0.38.1-1", Category: "Terminal GPU", Installed: true, Description: "Terminal acelerada por GPU con colores ANSI sincronizados por tema", Command: "sudo pacman -S kitty fastfetch"},
		{Repo: "extra", Name: "noto-fonts-cjk", Version: "20230817-2", Category: "Tipografía Kanji", Installed: true, Description: "Fuentes japonesas Kanji (一 二 三 四 五 / ネオ東京) + JetBrainsMono Nerd Font + Emojis", Command: "sudo pacman -S noto-fonts-cjk noto-fonts-emoji ttf-jetbrains-mono-nerd inter-font"},
		{Repo: "aur", Name: "bibata-cursor-theme-bin", Version: "2.0.7-1", Category: "Iconos & Cursor", Installed: true, Description: "Cursor Bibata-Modern-Ice + iconos Papirus-Dark para Kitsune Go Wayland", Command: "yay -S bibata-cursor-theme-bin && sudo pacman -S papirus-icon-theme"},
	}
}
