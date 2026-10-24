// Package config carga la configuración de swaydesk desde ~/.config/swaydesk/
// config.toml usando un parser TOML mínimo incluido en el propio paquete.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config agrupa todos los ajustes de swaydesk.
type Config struct {
	// General
	Theme        string
	NerdFont     bool
	Terminal     string
	Launcher     string // comando del menú flotante
	Editor       string
	FileManager  string
	Browser      string
	Mod          string
	GTKTheme     string // auto, Adwaita-dark u otro tema GTK3 instalado
	GTKIconTheme string // auto, Papirus-Dark u otro tema de iconos instalado

	// Apariencia
	Font           string // vacío = detección automática
	FontSize       int
	BarFontSize    int
	KeyboardLayout string

	// Apariencia de las aplicaciones TUI
	Width  int
	Height int
	Border string

	// Fondo de pantalla animado
	WallpaperKind     string // scene, image, gif, video, none
	WallpaperName     string // nombre de la escena
	WallpaperPath     string // ruta del archivo
	WallpaperMode     string // fill, fit, stretch, center, tile
	WallpaperFPS      int
	WallpaperProto    string // auto, kitty, sixel, ansi
	WallpaperAdaptive bool
	WallpaperDir      string
	// Fondos por workspace: clave (nombre, número o sufijo tras ':') → fondo.
	WallpaperWorkspaces map[string]string
	// Transición al cambiar de workspace: fade, slide, wipe, zoom, none.
	WallpaperTransition   string
	WallpaperTransitionMS int

	// Animación de arranque
	BootEnabled  bool
	BootStyle    string // sakura, neon, matrix, minimal
	BootDuration int    // milisegundos; 0 = automática
	BootLogo     bool

	// Barra de estado propia (swaybar)
	BarEnabled  bool
	BarPosition string // top, bottom
	BarHeight   int    // alto en píxeles
	BarMargin   int    // margen con el borde de la pantalla
	BarPadding  int    // relleno interno de las islas
	BarGap      int    // separación entre islas
	BarRadius   int    // radio de esquina (0 = pastilla)
	// alfa del fondo de las islas (el de waybar es effects.bar_opacity)
	BarBackgroundOpacity float64
	BarIslands           bool     // islas separadas
	BarLogo              bool     // mostrar el nombre del tema a la izquierda
	BarFont              string   // ruta de una fuente PSF (vacío = automática)
	BarFontScale         int      // escala de la fuente (0 = automática)
	BarShadow            bool     // sombra bajo las islas
	BarExclusive         bool     // reservar espacio (las ventanas no se solapan)
	BarModules           []string // módulos de la derecha

	// Notificaciones (swaynotify)
	NotifyEnabled      bool
	NotifyPosition     string // top-right, top-left, bottom-right, bottom-left, center
	NotifyWidth        int
	NotifyMargin       int
	NotifyPadding      int
	NotifyGap          int
	NotifyRadius       int
	NotifyOpacity      float64
	NotifyTimeout      int // milisegundos para urgencia normal o baja
	NotifyCriticalTime int // milisegundos (0 = no caducan)
	NotifyMaxVisible   int
	NotifyHistory      int
	NotifyMaxLines     int
	NotifyDND          bool // arrancar en modo «no molestar»
	NotifyGroup        bool // agrupar por aplicación en pilas desplegables
	NotifyFont         string
	NotifyFontScale    int

	// Avisos en pantalla (swayosd)
	OSDEnabled      bool   // OSD de volumen
	OSDMediaEnabled bool   // tarjeta de reproducción MPRIS
	OSDMediaTimeout int    // milisegundos que se muestra una pista
	OSDPosition     string // bottom-center, bottom-right, bottom-left, center, top-center
	OSDWidth        int
	OSDMargin       int
	OSDPadding      int
	OSDRadius       int
	OSDOpacity      float64
	OSDTimeout      int // milisegundos que se queda en pantalla
	OSDFade         int // milisegundos de fundido de entrada y salida
	OSDSegments     int // marcas de la barra de nivel
	OSDFont         string
	OSDFontScale    int

	// Efectos de escritorio
	TerminalOpacity float64
	BarOpacity      float64
	ActiveOpacity   float64
	InactiveOpacity float64
	Blur            bool
	GapsInner       int
	GapsOuter       int
	Rounding        int

	// Comportamiento
	HistorySize  int
	ShowHidden   bool
	ConfirmPower bool

	// Rutas resueltas
	Path   string
	Dir    string
	Cache  string
	values map[string]any
}

// Default devuelve la configuración por defecto.
func Default() *Config {
	home, _ := os.UserHomeDir()
	cache := filepath.Join(home, ".cache", "swaydesk")
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		cache = filepath.Join(xdg, "swaydesk")
	}
	cfgDir := filepath.Join(home, ".config", "swaydesk")
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		cfgDir = filepath.Join(xdg, "swaydesk")
	}
	return &Config{
		Theme:          "catppuccin",
		NerdFont:       true,
		Terminal:       "foot",
		Launcher:       "swaymenu --float",
		Editor:         "nvim",
		FileManager:    "thunar",
		Browser:        "chromium",
		Mod:            "Super",
		GTKTheme:       "auto",
		GTKIconTheme:   "auto",
		FontSize:       11,
		BarFontSize:    13,
		KeyboardLayout: "latam",
		Width:          72,
		Height:         20,
		Border:         "rounded",
		HistorySize:    200,
		ShowHidden:     true,
		ConfirmPower:   true,

		// Fondo animado y efectos por defecto: translúcido pero legible.
		WallpaperKind:     "scene",
		WallpaperMode:     "fill",
		WallpaperFPS:      30,
		WallpaperProto:    "auto",
		WallpaperAdaptive: false,
		WallpaperDir:      filepath.Join(home, "Imágenes", "Fondos"),

		WallpaperWorkspaces:   map[string]string{},
		WallpaperTransition:   "fade",
		WallpaperTransitionMS: 450,
		TerminalOpacity:       0.94,
		BarOpacity:            0.68,
		ActiveOpacity:         1.0,
		InactiveOpacity:       0.94,
		Blur:                  false,
		GapsInner:             6,
		GapsOuter:             8,
		Rounding:              12,

		BarEnabled:           true,
		BarPosition:          "top",
		BarHeight:            38,
		BarMargin:            6,
		BarPadding:           14,
		BarGap:               10,
		BarRadius:            0,
		BarBackgroundOpacity: 0.68,
		BarIslands:           true,
		BarLogo:              true,
		BarFontScale:         0,
		BarShadow:            true,
		BarExclusive:         true,
		BarModules:           []string{"media", "notify", "volume", "network", "battery", "clock"},

		NotifyEnabled:      true,
		NotifyPosition:     "top-right",
		NotifyWidth:        300,
		NotifyMargin:       14,
		NotifyPadding:      10,
		NotifyGap:          6,
		NotifyRadius:       12,
		NotifyOpacity:      0.94,
		NotifyTimeout:      6000,
		NotifyCriticalTime: 0,
		NotifyMaxVisible:   4,
		NotifyHistory:      50,
		NotifyMaxLines:     2,
		NotifyDND:          false,
		NotifyGroup:        true,
		NotifyFontScale:    0,

		OSDEnabled:      true,
		OSDMediaEnabled: true,
		OSDMediaTimeout: 3200,
		OSDPosition:     "bottom-center",
		OSDWidth:        260,
		OSDMargin:       72,
		OSDPadding:      12,
		OSDRadius:       14,
		OSDOpacity:      0.95,
		OSDTimeout:      1400,
		OSDFade:         140,
		OSDSegments:     24,
		OSDFontScale:    0,

		BootEnabled:  true,
		BootStyle:    "sakura",
		BootDuration: 2600,
		BootLogo:     true,

		Dir:    cfgDir,
		Path:   filepath.Join(cfgDir, "config.toml"),
		Cache:  cache,
		values: map[string]any{},
	}
}

// Load lee la configuración. Si el archivo no existe devuelve la configuración
// por defecto sin error.
func Load() (*Config, error) {
	c := Default()
	data, err := os.ReadFile(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	vals, err := parseTOML(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.Path, err)
	}
	c.values = vals
	c.apply()
	return c, nil
}

// LoadFile carga una configuración desde una ruta concreta.
func LoadFile(path string) (*Config, error) {
	c := Default()
	c.Path = path
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	vals, err := parseTOML(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.values = vals
	c.apply()
	return c, nil
}

func (c *Config) apply() {
	str := func(key string, dst *string) {
		if v, ok := c.values[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				*dst = s
			}
		}
	}
	num := func(key string, dst *int) {
		if v, ok := c.values[key]; ok {
			switch n := v.(type) {
			case int:
				if n > 0 {
					*dst = n
				}
			}
		}
	}
	flo := func(key string, dst *float64) {
		if v, ok := c.values[key]; ok {
			switch n := v.(type) {
			case float64:
				if n >= 0 && n <= 1 {
					*dst = n
				}
			case int:
				if n >= 0 && n <= 1 {
					*dst = float64(n)
				}
			}
		}
	}
	bl := func(key string, dst *bool) {
		if v, ok := c.values[key]; ok {
			if b, ok := v.(bool); ok {
				*dst = b
			}
		}
	}
	str("general.theme", &c.Theme)
	str("general.terminal", &c.Terminal)
	str("general.launcher", &c.Launcher)
	str("general.editor", &c.Editor)
	str("general.file_manager", &c.FileManager)
	str("general.browser", &c.Browser)
	str("general.mod", &c.Mod)
	str("gtk.theme", &c.GTKTheme)
	str("gtk.icon_theme", &c.GTKIconTheme)
	bl("general.nerd_font", &c.NerdFont)
	bl("general.confirm_power", &c.ConfirmPower)
	str("ui.font", &c.Font)
	num("ui.font_size", &c.FontSize)
	num("ui.bar_font_size", &c.BarFontSize)
	str("input.keyboard_layout", &c.KeyboardLayout)
	num("ui.width", &c.Width)
	str("wallpaper.kind", &c.WallpaperKind)
	str("wallpaper.name", &c.WallpaperName)
	str("wallpaper.path", &c.WallpaperPath)
	str("wallpaper.mode", &c.WallpaperMode)
	str("wallpaper.dir", &c.WallpaperDir)
	str("wallpaper.protocol", &c.WallpaperProto)
	num("wallpaper.fps", &c.WallpaperFPS)
	bl("wallpaper.adaptive", &c.WallpaperAdaptive)
	str("wallpaper.transition", &c.WallpaperTransition)
	if v, ok := c.values["wallpaper.transition_ms"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.WallpaperTransitionMS = n
		}
	}
	c.WallpaperWorkspaces = parseWorkspaceRules(c.values)
	bl("bar.enabled", &c.BarEnabled)
	str("bar.position", &c.BarPosition)
	num("bar.height", &c.BarHeight)
	num("bar.margin", &c.BarMargin)
	num("bar.padding", &c.BarPadding)
	num("bar.gap", &c.BarGap)
	if v, ok := c.values["bar.radius"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.BarRadius = n
		}
	}
	flo("bar.opacity", &c.BarBackgroundOpacity)
	bl("bar.islands", &c.BarIslands)
	bl("bar.logo", &c.BarLogo)
	str("bar.font", &c.BarFont)
	bl("bar.shadow", &c.BarShadow)
	bl("bar.exclusive", &c.BarExclusive)
	if v, ok := c.values["bar.modules"]; ok {
		c.BarModules = toStringList(v)
	}
	if v, ok := c.values["bar.font_scale"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.BarFontScale = n
		}
	}
	bl("notify.enabled", &c.NotifyEnabled)
	str("notify.position", &c.NotifyPosition)
	num("notify.width", &c.NotifyWidth)
	num("notify.margin", &c.NotifyMargin)
	num("notify.padding", &c.NotifyPadding)
	num("notify.gap", &c.NotifyGap)
	if v, ok := c.values["notify.radius"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.NotifyRadius = n
		}
	}
	flo("notify.opacity", &c.NotifyOpacity)
	num("notify.timeout_ms", &c.NotifyTimeout)
	if v, ok := c.values["notify.critical_timeout_ms"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.NotifyCriticalTime = n
		}
	}
	num("notify.max_visible", &c.NotifyMaxVisible)
	num("notify.history", &c.NotifyHistory)
	num("notify.max_lines", &c.NotifyMaxLines)
	bl("notify.dnd", &c.NotifyDND)
	bl("notify.group", &c.NotifyGroup)
	str("notify.font", &c.NotifyFont)
	if v, ok := c.values["notify.font_scale"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.NotifyFontScale = n
		}
	}
	bl("osd.enabled", &c.OSDEnabled)
	bl("osd.media_enabled", &c.OSDMediaEnabled)
	num("osd.media_timeout_ms", &c.OSDMediaTimeout)
	str("osd.position", &c.OSDPosition)
	num("osd.width", &c.OSDWidth)
	num("osd.margin", &c.OSDMargin)
	num("osd.padding", &c.OSDPadding)
	if v, ok := c.values["osd.radius"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.OSDRadius = n
		}
	}
	flo("osd.opacity", &c.OSDOpacity)
	num("osd.timeout_ms", &c.OSDTimeout)
	if v, ok := c.values["osd.fade_ms"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.OSDFade = n
		}
	}
	num("osd.segments", &c.OSDSegments)
	str("osd.font", &c.OSDFont)
	if v, ok := c.values["osd.font_scale"]; ok {
		if n, ok := v.(int); ok && n >= 0 {
			c.OSDFontScale = n
		}
	}
	bl("boot.enabled", &c.BootEnabled)
	str("boot.style", &c.BootStyle)
	num("boot.duration_ms", &c.BootDuration)
	bl("boot.logo", &c.BootLogo)
	num("effects.gaps_inner", &c.GapsInner)
	num("effects.gaps_outer", &c.GapsOuter)
	num("effects.rounding", &c.Rounding)
	bl("effects.blur", &c.Blur)
	flo("effects.terminal_opacity", &c.TerminalOpacity)
	flo("effects.bar_opacity", &c.BarOpacity)
	flo("effects.active_opacity", &c.ActiveOpacity)
	flo("effects.inactive_opacity", &c.InactiveOpacity)
	num("ui.height", &c.Height)
	str("ui.border", &c.Border)
	num("launcher.history_size", &c.HistorySize)
	bl("launcher.show_hidden", &c.ShowHidden)
}

// Get devuelve un valor crudo de la configuración (útil para extensiones).
func (c *Config) Get(key string) (any, bool) {
	v, ok := c.values[key]
	return v, ok
}

// String devuelve un valor de texto o el valor por defecto.
func (c *Config) String(key, def string) string {
	if v, ok := c.values[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

// Int devuelve un valor entero o el valor por defecto.
func (c *Config) Int(key string, def int) int {
	if v, ok := c.values[key]; ok {
		if n, ok := v.(int); ok {
			return n
		}
		if s, ok := v.(string); ok {
			if n, err := strconv.Atoi(s); err == nil {
				return n
			}
		}
	}
	return def
}

// Bool devuelve un valor booleano o el valor por defecto.
func (c *Config) Bool(key string, def bool) bool {
	if v, ok := c.values[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

// EnsureDir crea el directorio de caché si no existe.
func (c *Config) EnsureCache() error {
	return os.MkdirAll(c.Cache, 0o755)
}

// Save escribe la configuración actual en su archivo.
func (c *Config) Save() error {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.Path, []byte(DefaultFile(c)), 0o644)
}

// Set permite fijar valores desde la CLI (formato clave=valor).
func (c *Config) Set(pair string) error {
	eq := strings.Index(pair, "=")
	if eq < 0 {
		return fmt.Errorf("formato esperado clave=valor, recibido %q", pair)
	}
	key := strings.TrimSpace(pair[:eq])
	raw := strings.TrimSpace(pair[eq+1:])
	val := any(raw)
	if b, err := strconv.ParseBool(raw); err == nil {
		val = b
	} else if n, err := strconv.Atoi(raw); err == nil {
		val = n
	}
	if c.values == nil {
		c.values = map[string]any{}
	}
	c.values[key] = val
	c.apply()
	return nil
}

// DefaultFile devuelve el contenido del archivo de configuración de ejemplo.
func DefaultFile(c *Config) string {
	return fmt.Sprintf(`# Configuración de swaydesk
# Documentación: https://github.com/ShadowNigth/swaydesk

[general]
# Paleta: %s
theme = %q
# Usar glifos de Nerd Font (true) o símbolos ASCII/Unicode (false)
nerd_font = %t
# Terminal usado para los menús flotantes y acciones
terminal = %q
# Comando del lanzador flotante
launcher = %q
editor = %q
file_manager = %q
browser = %q
# Pedir confirmación en el menú de apagado
confirm_power = %t

[gtk]
# auto sigue el modo claro/oscuro de la paleta; también puedes indicar un tema instalado.
theme = %q
# auto usa Papirus/Papirus-Dark si está instalado; si no, Adwaita.
icon_theme = %q

[ui]
# Fuente monoespaciada; vacío = detección automática entre las instaladas.
font = %q
font_size = %d
bar_font_size = %d
width = %d
height = %d
# Borde de los paneles: rounded, square, double, thick, ascii
border = %q

[launcher]
history_size = %d
show_hidden = %t
`,
		strings.Join(paletteNames(), ", "),
		c.Theme, c.NerdFont, c.Terminal, c.Launcher, c.Editor,
		c.FileManager, c.Browser, c.ConfirmPower, c.GTKTheme, c.GTKIconTheme,
		c.Font, c.FontSize, c.BarFontSize,
		c.Width, c.Height, c.Border, c.HistorySize, c.ShowHidden) + fmt.Sprintf(`
[input]
# Distribución del teclado: latam, us, es, de, fr...
keyboard_layout = %q

[wallpaper]
# tipo: scene (animado procedural), image, gif, video, none
kind = %q
# escena animada: petals, snow, stars, rain, aurora, wave, city, embers, grid, meadow, bubbles
name = %q
# ruta para image/gif/video
path = %q
mode = %q
fps = %d
# protocolo de dibujado: auto, kitty, sixel, ansi
protocol = %q
# generar una paleta adaptada al fondo
adaptive = %t
# carpeta donde se buscan fondos
dir = %q
# transición al cambiar de workspace: fade, slide, wipe, zoom, none
transition = %q
transition_ms = %d
%s
[bar]
# barra de estado propia (swaybar). Pon enabled = false para usar waybar.
enabled = %t
# top o bottom
position = %q
height = %d
margin = %d
padding = %d
gap = %d
# radio de las esquinas en píxeles (0 = pastilla completa)
radius = %d
# opacidad del fondo de las islas (0-1)
opacity = %.2f
# islas separadas (izquierda, centro y derecha) o una barra continua
islands = %t
logo = %t
# fuente PSF de consola (vacío = la mejor disponible del sistema)
font = %q
# escala de la fuente (0 = automática según el alto)
font_scale = %d
shadow = %t
# reservar espacio para que las ventanas no se solapen
exclusive = %t
# módulos de la derecha: media, recording, notify, cpu, ram, disk, temp,
# uptime, volume, network, battery, updates, title, clock, date
modules = [%s]

[notify]
# daemon de notificaciones propio (swaynotify). Pon enabled = false para usar mako.
enabled = %t
# esquina donde se apilan: top-right, top-left, bottom-right, bottom-left
position = %q
# ancho de cada tarjeta en píxeles
width = %d
margin = %d
padding = %d
gap = %d
# radio de las esquinas (0 = pastilla)
radius = %d
# opacidad del fondo de las tarjetas (0-1)
opacity = %.2f
# tiempo en pantalla de las notificaciones normales (ms)
timeout_ms = %d
# las urgentes no caducan salvo que se indique lo contrario (0 = no caducan)
critical_timeout_ms = %d
# cuántas se apilan a la vez
max_visible = %d
# entradas que se conservan en el historial
history = %d
# líneas máximas del cuerpo
max_lines = %d
# arrancar en modo «no molestar»
dnd = %t
# agrupar por aplicación en pilas que se despliegan con un clic
group = %t
# fuente PSF de consola (vacío = la mejor disponible del sistema)
font = %q
# escala de la fuente (0 = automática)
font_scale = %d

[boot]
# animación que se muestra al iniciar la sesión
enabled = %t
# estilo: sakura (pétalos), neon (lluvia), matrix (glifos), minimal (sin partículas)
style = %q
# duración en milisegundos (0 = automática según el estilo)
duration_ms = %d
# dibujar el logotipo grande
logo = %t

[osd]
# aviso flotante con el nivel de volumen (swayosd)
enabled = %t
# tarjeta de reproducción al cambiar de pista (MPRIS por D-Bus)
media_enabled = %t
# duración de la tarjeta multimedia en milisegundos
media_timeout_ms = %d
# bottom-center, bottom-right, bottom-left, top-center o center
position = %q
# ancho de la tarjeta en píxeles lógicos
width = %d
# separación con el borde de la pantalla
margin = %d
padding = %d
radius = %d
opacity = %.2f
# cuánto tiempo se queda en pantalla (milisegundos)
timeout_ms = %d
# fundido de entrada y salida (milisegundos, 0 = sin animación)
fade_ms = %d
# marcas de la barra de nivel
segments = %d
# fuente PSF de consola (vacío = la mejor disponible del sistema)
font = %q
# escala de la fuente (0 = automática: tamaño compacto, escala 1)
font_scale = %d

[effects]
# transparencia de las ventanas (0 = opaco, 1 = invisible)
terminal_opacity = %.2f
bar_opacity = %.2f
active_opacity = %.2f
inactive_opacity = %.2f
blur = %t
gaps_inner = %d
gaps_outer = %d
rounding = %d
`, c.KeyboardLayout,
		c.WallpaperKind, c.WallpaperName, c.WallpaperPath, c.WallpaperMode,
		c.WallpaperFPS, c.WallpaperProto, c.WallpaperAdaptive, c.WallpaperDir,
		c.WallpaperTransition, c.WallpaperTransitionMS, workspaceSection(c),
		c.BarEnabled, c.BarPosition, c.BarHeight, c.BarMargin, c.BarPadding,
		c.BarGap, c.BarRadius, c.BarBackgroundOpacity, c.BarIslands, c.BarLogo, c.BarFont,
		c.BarFontScale, c.BarShadow, c.BarExclusive, tomlList(c.BarModules),
		c.NotifyEnabled, c.NotifyPosition, c.NotifyWidth, c.NotifyMargin,
		c.NotifyPadding, c.NotifyGap, c.NotifyRadius, c.NotifyOpacity,
		c.NotifyTimeout, c.NotifyCriticalTime, c.NotifyMaxVisible, c.NotifyHistory,
		c.NotifyMaxLines, c.NotifyDND, c.NotifyGroup, c.NotifyFont, c.NotifyFontScale,
		c.BootEnabled, c.BootStyle, c.BootDuration, c.BootLogo,
		c.OSDEnabled, c.OSDMediaEnabled, c.OSDMediaTimeout,
		c.OSDPosition, c.OSDWidth, c.OSDMargin, c.OSDPadding,
		c.OSDRadius, c.OSDOpacity, c.OSDTimeout, c.OSDFade, c.OSDSegments,
		c.OSDFont, c.OSDFontScale,
		c.TerminalOpacity, c.BarOpacity, c.ActiveOpacity, c.InactiveOpacity,
		c.Blur, c.GapsInner, c.GapsOuter, c.Rounding)
}

// parseWorkspaceRules recoge las reglas de [wallpaper.workspaces] (una clave
// por workspace) y la forma compacta workspaces = ["1=scene:petals", ...].
func parseWorkspaceRules(values map[string]any) map[string]string {
	out := map[string]string{}
	add := func(k, v string) {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	for key, raw := range values {
		if strings.HasPrefix(key, "wallpaper.workspaces.") {
			if s, ok := raw.(string); ok {
				add(strings.TrimPrefix(key, "wallpaper.workspaces."), s)
			}
			continue
		}
		if key != "wallpaper.workspaces" {
			continue
		}
		for _, item := range toStringList(raw) {
			if i := strings.Index(item, "="); i > 0 {
				add(item[:i], item[i+1:])
			}
		}
	}
	return out
}

// workspaceSection genera el bloque [wallpaper.workspaces] del archivo.
func workspaceSection(c *Config) string {
	var b strings.Builder
	b.WriteString("\n[wallpaper.workspaces]\n")
	b.WriteString("# un fondo por workspace: nombre, número o sufijo tras ':'\n")
	b.WriteString("# 1 = \"scene:petals\"\n")
	b.WriteString("# 3:web = \"image:~/Imágenes/Fondos/ciudad.png\"\n")
	keys := make([]string, 0, len(c.WallpaperWorkspaces))
	for k := range c.WallpaperWorkspaces {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s = %q\n", strconv.Quote(k), c.WallpaperWorkspaces[k])
	}
	return b.String()
}

// toStringList normaliza una lista de cadenas leída del TOML.
func toStringList(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		var out []string
		for _, part := range strings.Split(t, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// tomlList serializa una lista de cadenas para el archivo de configuración.
func tomlList(list []string) string {
	parts := make([]string, 0, len(list))
	for _, s := range list {
		parts = append(parts, strconv.Quote(s))
	}
	return strings.Join(parts, ", ")
}

func paletteNames() []string {
	return []string{"catppuccin", "tokyonight", "nord", "gruvbox", "dracula", "kanagawa", "rosepine", "everforest"}
}

// BorderStyle devuelve el número de estilo de borde configurado.
func (c *Config) BorderStyle() int {
	switch strings.ToLower(c.Border) {
	case "square":
		return 2
	case "double":
		return 3
	case "thick":
		return 4
	case "ascii":
		return 5
	case "none":
		return 0
	default:
		return 1
	}
}

// SaveTheme cambia el tema en el archivo de configuración conservando el resto
// del contenido, y actualiza el valor en memoria.
func (c *Config) SaveTheme(name string) error {
	c.Theme = name
	data, err := os.ReadFile(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Save()
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	section := ""
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = strings.Trim(trimmed, "[]")
			continue
		}
		if section == "general" && strings.HasPrefix(trimmed, "theme") && strings.Contains(trimmed, "=") {
			lines[i] = fmt.Sprintf("theme = %q", name)
			replaced = true
			break
		}
	}
	if !replaced {
		out := fmt.Sprintf("[general]\ntheme = %q\n\n", name) + string(data)
		return os.WriteFile(c.Path, []byte(out), 0o644)
	}
	return os.WriteFile(c.Path, []byte(strings.Join(lines, "\n")), 0o644)
}
