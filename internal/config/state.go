package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Theme defines a complete Anime visual and terminal color palette for KitsuneDE & Arch dotfiles.
type Theme struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	JapaneseName string   `json:"japaneseName"`
	Subtitle     string   `json:"subtitle"`
	Wallpaper    string   `json:"wallpaper"`
	Avatar       string   `json:"avatar"`
	Primary      string   `json:"primary"`
	Secondary    string   `json:"secondary"`
	Accent       string   `json:"accent"`
	BgBase       string   `json:"bgBase"`
	BgSurface    string   `json:"bgSurface"`
	BgOverlay    string   `json:"bgOverlay"`
	TextMain     string   `json:"textMain"`
	TextMuted    string   `json:"textMuted"`
	Success      string   `json:"success"`
	Warning      string   `json:"warning"`
	Danger       string   `json:"danger"`
	ParticleMode string   `json:"particleMode"` // "rain", "sakura", "embers", "hud"
	TermColors   []string `json:"termColors"`   // 8 standard ANSI colors
}

// DesktopSettings holds live configurable window manager & visual parameters.
type DesktopSettings struct {
	ActiveTheme       string `json:"activeTheme"`
	Compositor        string `json:"compositor"` // "swayfx", "sway", "hyprland"
	LayoutMode        string `json:"layoutMode"` // "tiling", "tabbed", "floating", "monocle"
	ActiveWorkspace   int    `json:"activeWorkspace"`
	GapsIn            int    `json:"gapsIn"`
	GapsOut           int    `json:"gapsOut"`
	BorderSize        int    `json:"borderSize"`
	BorderRadius      int    `json:"borderRadius"`
	WindowOpacity     int    `json:"windowOpacity"` // percentage 70..100
	BlurEnabled       bool   `json:"blurEnabled"`
	ParticlesEnabled  bool   `json:"particlesEnabled"`
	AnimationsEnabled bool   `json:"animationsEnabled"`
	BorderGlow        bool   `json:"borderGlow"`
	DockVisible       bool   `json:"dockVisible"`
}

// BuiltinThemes returns all built-in Anime Arch Rice themes.
func BuiltinThemes() []Theme {
	return []Theme{
		{
			ID:           "neon-tokyo",
			Name:         "Cyberpunk Neon Tokyo",
			JapaneseName: "ネオ東京 // 2099",
			Subtitle:     "Lluvia nocturna, neón cian/magenta y bordes brillantes estilo Edgerunners",
			Wallpaper:    "/wallpapers/neon-tokyo.jpg",
			Avatar:       "/wallpapers/kitsune-avatar.jpg",
			Primary:      "#00f0ff",
			Secondary:    "#ff2a85",
			Accent:       "#bd00ff",
			BgBase:       "#0b0916",
			BgSurface:    "#131026",
			BgOverlay:    "#1e1938",
			TextMain:     "#eef6ff",
			TextMuted:    "#8f9bb8",
			Success:      "#00ffa3",
			Warning:      "#ffd166",
			Danger:       "#ff2a6d",
			ParticleMode: "rain",
			TermColors:   []string{"#131026", "#ff2a85", "#00ffa3", "#ffd166", "#00f0ff", "#bd00ff", "#05d9e8", "#eef6ff"},
		},
		{
			ID:           "sakura-lofi",
			Name:         "Sakura Catppuccin",
			JapaneseName: "桜ローファイ // 宵",
			Subtitle:     "Pétalos de cerezo, rosa pastel y lavanda sobre Catppuccin Mocha acogedor",
			Wallpaper:    "/wallpapers/sakura-lofi.jpg",
			Avatar:       "/wallpapers/kitsune-avatar.jpg",
			Primary:      "#f5c2e7",
			Secondary:    "#cba6f7",
			Accent:       "#fab387",
			BgBase:       "#181825",
			BgSurface:    "#1e1e2e",
			BgOverlay:    "#313244",
			TextMain:     "#cdd6f4",
			TextMuted:    "#9399b2",
			Success:      "#a6e3a1",
			Warning:      "#f9e2af",
			Danger:       "#f38ba8",
			ParticleMode: "sakura",
			TermColors:   []string{"#1e1e2e", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#cdd6f4"},
		},
		{
			ID:           "crimson-shonen",
			Name:         "Crimson Shonen",
			JapaneseName: "紅蓮の刃 // 月蝕",
			Subtitle:     "Negro obsidiana, luna carmesí, brasas flotantes y contraste afilado",
			Wallpaper:    "/wallpapers/crimson-shonen.jpg",
			Avatar:       "/wallpapers/kitsune-avatar.jpg",
			Primary:      "#ff1e42",
			Secondary:    "#ff7b54",
			Accent:       "#f8fafc",
			BgBase:       "#09090d",
			BgSurface:    "#131118",
			BgOverlay:    "#221c26",
			TextMain:     "#f8fafc",
			TextMuted:    "#94a3b8",
			Success:      "#4ade80",
			Warning:      "#fb923c",
			Danger:       "#ff1e42",
			ParticleMode: "embers",
			TermColors:   []string{"#131118", "#ff1e42", "#4ade80", "#fb923c", "#60a5fa", "#f43f5e", "#f97316", "#f8fafc"},
		},
		{
			ID:           "eva-midnight",
			Name:         "Midnight Evangelion",
			JapaneseName: "第3新東京市 // NERV",
			Subtitle:     "Estética táctica mecha de los 90 con verde ácido EVA-01, ámbar y violeta",
			Wallpaper:    "/wallpapers/eva-midnight.jpg",
			Avatar:       "/wallpapers/kitsune-avatar.jpg",
			Primary:      "#39ff14",
			Secondary:    "#ff9e00",
			Accent:       "#9d4edd",
			BgBase:       "#10081c",
			BgSurface:    "#190e2c",
			BgOverlay:    "#281845",
			TextMain:     "#f0fdf4",
			TextMuted:    "#a78bfa",
			Success:      "#39ff14",
			Warning:      "#ff9e00",
			Danger:       "#ff0055",
			ParticleMode: "hud",
			TermColors:   []string{"#190e2c", "#ff0055", "#39ff14", "#ff9e00", "#7209b7", "#9d4edd", "#00f5d4", "#f0fdf4"},
		},
	}
}

// DefaultSettings returns the initial desktop settings.
func DefaultSettings() DesktopSettings {
	return DesktopSettings{
		ActiveTheme:       "neon-tokyo",
		Compositor:        "swayfx",
		LayoutMode:        "tiling",
		ActiveWorkspace:   1,
		GapsIn:            8,
		GapsOut:           14,
		BorderSize:        2,
		BorderRadius:      12,
		WindowOpacity:     90,
		BlurEnabled:       true,
		ParticlesEnabled:  true,
		AnimationsEnabled: true,
		BorderGlow:        true,
		DockVisible:       true,
	}
}

// Store manages the active desktop state in a thread-safe manner.
type Store struct {
	mu       sync.RWMutex
	themes   map[string]Theme
	order    []Theme
	settings DesktopSettings
	savePath string
}

// NewStore creates a state store and optionally loads persisted config.
func NewStore(savePath string) *Store {
	list := BuiltinThemes()
	tmap := make(map[string]Theme, len(list))
	for _, t := range list {
		tmap[t.ID] = t
	}
	s := &Store{
		themes:   tmap,
		order:    list,
		settings: DefaultSettings(),
		savePath: savePath,
	}
	if savePath != "" {
		s.loadFromDisk()
	}
	return s
}

func (s *Store) loadFromDisk() {
	data, err := os.ReadFile(s.savePath)
	if err != nil {
		return
	}
	var loaded DesktopSettings
	if err := json.Unmarshal(data, &loaded); err == nil {
		if _, ok := s.themes[loaded.ActiveTheme]; ok {
			s.settings = sanitizeSettings(loaded)
		}
	}
}

func (s *Store) persistLocked() {
	if s.savePath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.savePath), 0o755)
	if data, err := json.MarshalIndent(s.settings, "", "  "); err == nil {
		_ = os.WriteFile(s.savePath, data, 0o644)
	}
}

func sanitizeSettings(cfg DesktopSettings) DesktopSettings {
	if cfg.Compositor != "swayfx" && cfg.Compositor != "sway" && cfg.Compositor != "hyprland" {
		cfg.Compositor = "swayfx"
	}
	if cfg.LayoutMode != "tiling" && cfg.LayoutMode != "tabbed" && cfg.LayoutMode != "floating" && cfg.LayoutMode != "monocle" {
		cfg.LayoutMode = "tiling"
	}
	if cfg.ActiveWorkspace < 1 || cfg.ActiveWorkspace > 5 {
		cfg.ActiveWorkspace = 1
	}
	if cfg.GapsIn < 0 {
		cfg.GapsIn = 0
	} else if cfg.GapsIn > 32 {
		cfg.GapsIn = 32
	}
	if cfg.GapsOut < 0 {
		cfg.GapsOut = 0
	} else if cfg.GapsOut > 48 {
		cfg.GapsOut = 48
	}
	if cfg.BorderSize < 0 {
		cfg.BorderSize = 0
	} else if cfg.BorderSize > 6 {
		cfg.BorderSize = 6
	}
	if cfg.BorderRadius < 0 {
		cfg.BorderRadius = 0
	} else if cfg.BorderRadius > 28 {
		cfg.BorderRadius = 28
	}
	if cfg.WindowOpacity < 65 {
		cfg.WindowOpacity = 65
	} else if cfg.WindowOpacity > 100 {
		cfg.WindowOpacity = 100
	}
	return cfg
}

func (s *Store) Themes() []Theme {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Theme, len(s.order))
	copy(out, s.order)
	return out
}

func (s *Store) GetTheme(id string) (Theme, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.themes[id]
	return t, ok
}

func (s *Store) ActiveTheme() Theme {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.themes[s.settings.ActiveTheme]; ok {
		return t
	}
	return s.order[0]
}

func (s *Store) Settings() DesktopSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) UpdateSettings(next DesktopSettings) (DesktopSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.themes[next.ActiveTheme]; !ok {
		return s.settings, errors.New("tema desconocido: " + next.ActiveTheme)
	}
	s.settings = sanitizeSettings(next)
	s.persistLocked()
	return s.settings, nil
}

func (s *Store) SetTheme(themeID string) (DesktopSettings, Theme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.themes[themeID]
	if !ok {
		return s.settings, Theme{}, errors.New("tema no encontrado: " + themeID)
	}
	s.settings.ActiveTheme = themeID
	s.persistLocked()
	return s.settings, t, nil
}

func (s *Store) SetLayout(mode string) (DesktopSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode == "toggle" {
		switch s.settings.LayoutMode {
		case "tiling":
			mode = "tabbed"
		case "tabbed":
			mode = "floating"
		case "floating":
			mode = "monocle"
		default:
			mode = "tiling"
		}
	}
	if mode != "tiling" && mode != "tabbed" && mode != "floating" && mode != "monocle" {
		return s.settings, errors.New("modo de layout inválido: " + mode)
	}
	s.settings.LayoutMode = mode
	s.persistLocked()
	return s.settings, nil
}

func (s *Store) SetCompositor(comp string) (DesktopSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if comp == "toggle" {
		switch s.settings.Compositor {
		case "swayfx":
			comp = "sway"
		case "sway":
			comp = "hyprland"
		default:
			comp = "swayfx"
		}
	}
	if comp != "swayfx" && comp != "sway" && comp != "hyprland" {
		return s.settings, errors.New("compositor inválido (usa swayfx, sway o hyprland): " + comp)
	}
	s.settings.Compositor = comp
	s.persistLocked()
	return s.settings, nil
}

func (s *Store) SetWorkspace(ws int) (DesktopSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ws < 1 || ws > 5 {
		return s.settings, errors.New("workspace debe estar entre 1 y 5")
	}
	s.settings.ActiveWorkspace = ws
	s.persistLocked()
	return s.settings, nil
}
