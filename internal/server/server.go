package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
		// Allow any preview proxy host & iframe embedding cleanly
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
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "kitsune-de"})
	})

	s.mux.HandleFunc("/api/bootstrap", s.handleBootstrap)
	s.mux.HandleFunc("/api/system", s.handleSystem)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/ctl", s.handleCtl)
	s.mux.HandleFunc("/api/ricer/preview", s.handleRicerPreview)
	s.mux.HandleFunc("/api/ricer/export", s.handleRicerExport)

	s.mux.HandleFunc("/api/fs/list", s.fsHandler.HandleList)
	s.mux.HandleFunc("/api/fs/read", s.fsHandler.HandleRead)
	s.mux.HandleFunc("/api/fs/write", s.fsHandler.HandleWrite)

	s.mux.HandleFunc("/download/kitsune-de-arch-linux.zip", func(w http.ResponseWriter, r *http.Request) {
		zipPath := filepath.Join(s.workDir, "release", "kitsune-de-arch-linux.zip")
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="kitsune-de-arch-linux.zip"`)
		http.ServeFile(w, r, zipPath)
	})

	s.mux.HandleFunc("/ws/events", s.handleEventsWS)
	s.mux.HandleFunc("/ws/terminal", s.handleTerminalWS)

	fileServer := http.FileServer(http.FS(staticFS))
	s.mux.Handle("/", fileServer)
	return nil
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
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
			title = "狐 KitsuneDE IPC"
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
		outRel = "dotfiles-export"
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
			"title": "Dotfiles Arch Generados",
			"body":  "Exportados en ./" + outRel + " (" + theme.Name + ")",
			"icon":  "check",
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
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

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func builtinArchCatalog() []ArchPackage {
	return []ArchPackage{
		{Repo: "kitsune", Name: "kitsune-de", Version: "1.0.0-1", Category: "Desktop Shell", Installed: true, Description: "Entorno de escritorio híbrido Tiling/Flotante estilo Anime para Arch Linux escrito en Go", Command: "makepkg -si (en ./arch)"},
		{Repo: "extra", Name: "hyprland", Version: "0.46.2-1", Category: "Compositor Wayland", Installed: true, Description: "Compositor Wayland dinámico con bordes degradados, blur y animaciones fluidas", Command: "sudo pacman -S hyprland"},
		{Repo: "extra", Name: "waybar", Version: "0.11.0-2", Category: "Barra Superior", Installed: true, Description: "Barra de estado altamente configurable para compositores wlroots/Hyprland", Command: "sudo pacman -S waybar"},
		{Repo: "extra", Name: "swww", Version: "0.9.5-1", Category: "Wallpapers Anime", Installed: true, Description: "Demonio de fondos de pantalla animados con transiciones suaves en Wayland", Command: "sudo pacman -S swww"},
		{Repo: "extra", Name: "kitty", Version: "0.38.1-1", Category: "Terminal GPU", Installed: true, Description: "Emulador de terminal veloz acelerado por GPU con soporte de imágenes anime", Command: "sudo pacman -S kitty"},
		{Repo: "aur", Name: "rofi-wayland", Version: "1.7.5-3", Category: "App Launcher", Installed: true, Description: "Lanzador de aplicaciones y menú dmenu estilizable con temas .rasi", Command: "yay -S rofi-wayland"},
		{Repo: "extra", Name: "fastfetch", Version: "2.33.0-1", Category: "System Fetch", Installed: true, Description: "Muestra información de Arch Linux con arte ASCII o avatares anime", Command: "sudo pacman -S fastfetch"},
		{Repo: "extra", Name: "cage", Version: "0.2.0-1", Category: "Wayland Kiosk", Installed: false, Description: "Compositor Wayland ligero ideal para ejecutar KitsuneDE como sesión nativa en SDDM", Command: "sudo pacman -S cage"},
		{Repo: "extra", Name: "btop", Version: "1.4.0-1", Category: "System Monitor", Installed: true, Description: "Monitor de recursos en terminal para CPU, memoria, discos y procesos", Command: "sudo pacman -S btop"},
		{Repo: "extra", Name: "yazi", Version: "0.4.2-1", Category: "File Manager", Installed: false, Description: "Gestor de archivos ultrarrápido con previsualización de imágenes y código", Command: "sudo pacman -S yazi"},
		{Repo: "aur", Name: "catppuccin-gtk-theme-mocha", Version: "1.0.3-1", Category: "Temas Anime", Installed: false, Description: "Tema GTK oscuro pastel combinable con el modo Sakura Catppuccin", Command: "yay -S catppuccin-gtk-theme-mocha"},
		{Repo: "aur", Name: "swaync", Version: "0.10.1-2", Category: "Notificaciones", Installed: true, Description: "Centro de notificaciones elegante para escritorios Wayland estilo anime", Command: "yay -S swaync"},
	}
}
