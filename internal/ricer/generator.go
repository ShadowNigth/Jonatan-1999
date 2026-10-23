package ricer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

// Bundle holds generated Arch Linux configuration files for Sway, SwayFX, Hyprland & Waybar.
type Bundle struct {
	ThemeID    string            `json:"themeId"`
	ThemeName  string            `json:"themeName"`
	Compositor string            `json:"compositor"`
	Files      map[string]string `json:"files"`
}

func stripHex(c string) string {
	return strings.TrimPrefix(strings.TrimSpace(c), "#")
}

// Generate creates a complete set of Arch Linux dotfiles for Sway, SwayFX, Waybar, Swaylock, SwayNC, Kitty, Rofi & Hyprland.
func Generate(theme config.Theme, s config.DesktopSettings) Bundle {
	comp := s.Compositor
	if comp == "" {
		comp = "swayfx"
	}

	p := stripHex(theme.Primary)
	sec := stripHex(theme.Secondary)
	bg := stripHex(theme.BgBase)
	surf := stripHex(theme.BgSurface)
	over := stripHex(theme.BgOverlay)
	txt := stripHex(theme.TextMain)
	muted := stripHex(theme.TextMuted)
	danger := stripHex(theme.Danger)
	opacity := fmt.Sprintf("%.2f", float64(s.WindowOpacity)/100.0)

	blurEnable := "enable"
	blurBool := "true"
	if !s.BlurEnabled {
		blurEnable = "disable"
		blurBool = "false"
	}
	shadowEnable := "enable"
	if !s.BorderGlow {
		shadowEnable = "disable"
	}
	animStr := "yes"
	if !s.AnimationsEnabled {
		animStr = "no"
	}
	wallFile := filepath.Base(theme.Wallpaper)

	// SwayFX eye-candy block (used directly in swayfx/config and conditionally in sway/config)
	swayFXBlock := fmt.Sprintf(`# ── Efectos Exclusivos de SwayFX ( Anime Eye-Candy ) ─────────────────────────
corner_radius %d
smart_corner_radius off

blur %s
blur_xray disable
blur_passes 3
blur_radius 6
blur_noise 0.02
blur_brightness 0.95
blur_contrast 1.05
blur_saturation 1.15

shadows %s
shadows_on_csd enable
shadow_blur_radius 22
shadow_color #%s66
shadow_inactive_color #%s88

default_dim_inactive 0.10
dim_inactive_colors.unfocused #%sAA
dim_inactive_colors.urgent #%sAA

layer_effects "waybar" "blur %s; shadows %s; corner_radius %d"
layer_effects "rofi" "blur %s; shadows enable; corner_radius %d"
layer_effects "notifications" "blur %s; corner_radius %d"
`,
		s.BorderRadius,
		blurEnable,
		shadowEnable, p, bg,
		bg, danger,
		blurEnable, shadowEnable, s.BorderRadius,
		blurEnable, s.BorderRadius,
		blurEnable, s.BorderRadius,
	)

	fxIncludeLine := "# Modo Sway estándar (sin directivas exclusivas de SwayFX para evitar avisos)"
	if comp == "swayfx" {
		fxIncludeLine = "include ~/.config/sway/swayfx-effects.conf"
	}

	swayBaseConf := fmt.Sprintf(`# ============================================================================
# 🦊 KitsuneDE // Configuración para Sway & SwayFX generada en Go
# Tema Anime : %s (%s)
# Perfil WM  : %s
# ============================================================================

set $mod Mod4
set $left h
set $down j
set $up k
set $right l
set $term kitty
set $menu rofi -show drun

# Fuente global para títulos y barras de pestañas (Tabbed/Stacked)
font pango:JetBrainsMono Nerd Font SemiBold 10

# Fondo de pantalla Anime (soporta swaybg nativo o swww animado)
output * bg ~/.config/kitsune-de/wallpapers/%s fill
exec_once swww-daemon
exec swww img ~/.config/kitsune-de/wallpapers/%s --transition-type grow --transition-fps 60

# Demonios del entorno KitsuneDE + Waybar + SwayNC + Autotiling
exec waybar -c ~/.config/waybar/config-sway.jsonc -s ~/.config/waybar/style.css
exec swaync
exec autotiling
exec kitsune-de --addr 127.0.0.1:8080

# Configuración de teclado e input (Español Latam / Inglés + Touchpad natural)
input "type:keyboard" {
    xkb_layout "latam,us"
    xkb_options "grp:alt_shift_toggle"
    repeat_delay 260
    repeat_rate 38
}

input "type:touchpad" {
    tap enabled
    natural_scroll enabled
    dwt enabled
}

# Gaps y Bordes estilo Anime
gaps inner %d
gaps outer %d
default_border pixel %d
default_floating_border pixel %d
hide_edge_borders none
smart_gaps off

# Paleta de Colores de Ventanas (border | background | text | indicator | child_border)
client.focused          #%s #%s #%s #%s #%s
client.focused_inactive #%s #%s #%s #%s #%s
client.unfocused        #%s #%s #%s #%s #%s
client.urgent           #%s #%s #%s #%s #%s

# Reglas para ventanas flotantes comunes
for_window [app_id="pavucontrol"] floating enable, resize set 720 480
for_window [app_id="blueman-manager"] floating enable
for_window [window_role="pop-up"] floating enable
for_window [window_role="dialog"] floating enable
for_window [app_id="kitty"] opacity %s

# ── Atajos Principales ($mod = Super) ────────────────────────────────────────
bindsym $mod+Return exec $term
bindsym $mod+q kill
bindsym $mod+space exec $menu
bindsym $mod+d exec $menu
bindsym $mod+e exec kitty -e yazi
bindsym $mod+n exec swaync-client -t -sw
bindsym $mod+x exec swaylock -C ~/.config/swaylock/config
bindsym $mod+t exec kitsune-ctl theme next
bindsym $mod+Shift+d exec kitsune-de --kiosk

floating_modifier $mod normal
bindsym $mod+Shift+c reload
bindsym $mod+Shift+e exec swaynag -t warning -m '¿Salir de KitsuneDE Sway/SwayFX?' -B 'Cerrar sesión' 'swaymsg exit'

# ── Navegación y Movimiento de Contenedores ──────────────────────────────────
bindsym $mod+$left focus left
bindsym $mod+$down focus down
bindsym $mod+$up focus up
bindsym $mod+$right focus right
bindsym $mod+Left focus left
bindsym $mod+Down focus down
bindsym $mod+Up focus up
bindsym $mod+Right focus right

bindsym $mod+Shift+$left move left
bindsym $mod+Shift+$down move down
bindsym $mod+Shift+$up move up
bindsym $mod+Shift+$right move right
bindsym $mod+Shift+Left move left
bindsym $mod+Shift+Down move down
bindsym $mod+Shift+Up move up
bindsym $mod+Shift+Right move right

# ── Espacios de Trabajo Kanji (1..5) ─────────────────────────────────────────
set $ws1 "1"
set $ws2 "2"
set $ws3 "3"
set $ws4 "4"
set $ws5 "5"

bindsym $mod+1 workspace number $ws1
bindsym $mod+2 workspace number $ws2
bindsym $mod+3 workspace number $ws3
bindsym $mod+4 workspace number $ws4
bindsym $mod+5 workspace number $ws5

bindsym $mod+Shift+1 move container to workspace number $ws1
bindsym $mod+Shift+2 move container to workspace number $ws2
bindsym $mod+Shift+3 move container to workspace number $ws3
bindsym $mod+Shift+4 move container to workspace number $ws4
bindsym $mod+Shift+5 move container to workspace number $ws5

# ── Modos de Layout de Sway (Split / Stacking / Tabbed / Floating) ──────────
bindsym $mod+b splith
bindsym $mod+v splitv
bindsym $mod+s layout stacking
bindsym $mod+w layout tabbed
bindsym $mod+a layout toggle split
bindsym $mod+f fullscreen
bindsym $mod+Shift+space floating toggle
bindsym $mod+Ctrl+space focus mode_toggle

# Scratchpad rápido (Terminal flotante oculta)
bindsym $mod+Shift+minus move scratchpad
bindsym $mod+minus scratchpad show

# Modo Redimensionar (Super+R)
mode "resize" {
    bindsym $left resize shrink width 20px
    bindsym $down resize grow height 20px
    bindsym $up resize shrink height 20px
    bindsym $right resize grow width 20px
    bindsym Left resize shrink width 20px
    bindsym Down resize grow height 20px
    bindsym Up resize shrink height 20px
    bindsym Right resize grow width 20px
    bindsym Return mode "default"
    bindsym Escape mode "default"
}
bindsym $mod+r mode "resize"

# Inclusión de efectos SwayFX (si se instaló con perfil SwayFX)
%s
`,
		theme.Name, theme.JapaneseName, strings.ToUpper(comp),
		wallFile, wallFile,
		s.GapsIn, s.GapsOut, s.BorderSize, s.BorderSize,
		p, surf, txt, sec, p,
		over, bg, muted, over, over,
		over, bg, muted, over, over,
		danger, surf, txt, danger, danger,
		opacity,
		fxIncludeLine,
	)

	// Full standalone SwayFX config (for ~/.config/swayfx/config or direct swayfx usage)
	swayFXFullConf := strings.Replace(swayBaseConf, fxIncludeLine, swayFXBlock, 1)

	// Waybar config specifically using sway/workspaces, sway/mode & sway/window
	waybarSwayConfig := `{
  "layer": "top",
  "position": "top",
  "height": 38,
  "margin-top": 8,
  "margin-left": 14,
  "margin-right": 14,
  "modules-left": ["custom/arch", "sway/workspaces", "sway/mode", "sway/window"],
  "modules-center": ["clock"],
  "modules-right": ["pulseaudio", "cpu", "memory", "network", "custom/swaync", "custom/kitsune"],
  "custom/arch": {
    "format": "󰣇  Kitsune Sway",
    "on-click": "rofi -show drun",
    "tooltip": false
  },
  "sway/workspaces": {
    "disable-scroll": false,
    "all-outputs": true,
    "format": "{icon}",
    "format-icons": {
      "1": "一",
      "2": "二",
      "3": "三",
      "4": "四",
      "5": "五",
      "urgent": "",
      "focused": "狐",
      "default": "○"
    }
  },
  "sway/mode": {
    "format": "󰩨  {}",
    "tooltip": false
  },
  "sway/window": {
    "max-length": 42,
    "rewrite": {
      "(.*) — Mozilla Firefox": "󰈹 $1",
      "(.*) - nvim": " $1"
    }
  },
  "clock": {
    "format": "󰥔  {:%H:%M  •  %d/%m}",
    "tooltip-format": "<big>{:%Y %B}</big>\n<tt>{calendar}</tt>"
  },
  "cpu": { "format": "󰻠 {usage}%" },
  "memory": { "format": "󰍛 {percentage}%" },
  "pulseaudio": { "format": "󰕾 {volume}%" },
  "network": { "format-wifi": "󰤨 {essid}", "format-ethernet": "󰈀 ETH", "format-disconnected": "󰤭 Offline" },
  "custom/swaync": {
    "format": "🔔",
    "on-click": "swaync-client -t -sw",
    "tooltip": false
  },
  "custom/kitsune": {
    "format": "狐 Rice",
    "on-click": "kitsune-ctl theme next"
  }
}
`

	waybarHyprConfig := `{
  "layer": "top",
  "position": "top",
  "height": 38,
  "margin-top": 8,
  "margin-left": 14,
  "margin-right": 14,
  "modules-left": ["custom/arch", "hyprland/workspaces", "hyprland/window"],
  "modules-center": ["clock"],
  "modules-right": ["pulseaudio", "cpu", "memory", "network", "custom/kitsune"],
  "custom/arch": {
    "format": "󰣇  KitsuneDE",
    "on-click": "rofi -show drun",
    "tooltip": false
  },
  "hyprland/workspaces": {
    "format": "{icon}",
    "format-icons": {
      "1": "一",
      "2": "二",
      "3": "三",
      "4": "四",
      "5": "五",
      "default": "狐"
    }
  },
  "clock": {
    "format": "󰥔  {:%H:%M  •  %d/%m}"
  },
  "cpu": { "format": "󰻠 {usage}%" },
  "memory": { "format": "󰍛 {percentage}%" },
  "pulseaudio": { "format": "󰕾 {volume}%" },
  "network": { "format-wifi": "󰤨 {essid}", "format-ethernet": "󰈀 ETH", "format-disconnected": "󰤭 Offline" },
  "custom/kitsune": {
    "format": "狐 Rice",
    "on-click": "kitsune-ctl theme next"
  }
}
`

	defaultWaybar := waybarSwayConfig
	if comp == "hyprland" {
		defaultWaybar = waybarHyprConfig
	}

	waybarCSS := fmt.Sprintf(`/* KitsuneDE Waybar CSS (Sway / SwayFX / Hyprland) - %s */
* {
  font-family: "JetBrainsMono Nerd Font", "Noto Sans CJK JP", monospace;
  font-size: 13px;
  font-weight: 600;
  border: none;
}

window#waybar {
  background: rgba(15, 13, 28, 0.74);
  color: #%s;
  border-radius: %dpx;
  border: 1px solid #%s;
}

#custom-arch {
  background: linear-gradient(135deg, #%s, #%s);
  color: #%s;
  padding: 4px 14px;
  margin: 4px 6px;
  border-radius: 10px;
  font-weight: 800;
}

#workspaces button {
  color: #%s;
  padding: 2px 10px;
  margin: 4px 3px;
  border-radius: 8px;
  background: #%s;
}

#workspaces button.focused,
#workspaces button.active {
  color: #%s;
  background: #%s;
  box-shadow: 0 0 12px #%s;
}

#workspaces button.urgent {
  background: #%s;
  color: #ffffff;
}

#mode {
  background: #%s;
  color: #%s;
  padding: 4px 10px;
  margin: 4px 4px;
  border-radius: 8px;
  font-weight: 800;
}

#window, #clock, #cpu, #memory, #pulseaudio, #network, #custom-swaync, #custom-kitsune {
  background: #%s;
  color: #%s;
  padding: 4px 12px;
  margin: 4px 4px;
  border-radius: 9px;
}
`,
		theme.Name, txt, s.BorderRadius, p, p, sec, bg, muted, surf, bg, p, p, danger, sec, bg, surf, txt,
	)

	swaylockConf := fmt.Sprintf(`# KitsuneDE Swaylock / Swaylock-Effects Anime Config - %s
image=~/.config/kitsune-de/wallpapers/%s
scaling=fill
font=JetBrainsMono Nerd Font
font-size=24
indicator-radius=110
indicator-thickness=10

inside-color=%sCC
ring-color=%sFF
line-color=00000000
key-hl-color=%sFF
bs-hl-color=%sFF
separator-color=00000000
text-color=%sFF

inside-ver-color=%sCC
ring-ver-color=%sFF
text-ver-color=%sFF

inside-wrong-color=%sCC
ring-wrong-color=%sFF
text-wrong-color=%sFF
`,
		theme.Name, wallFile,
		surf, p, sec, danger, txt,
		surf, sec, txt,
		surf, danger, txt,
	)

	swayncCSS := fmt.Sprintf(`/* KitsuneDE SwayNC Anime Notification Theme - %s */
* {
  font-family: "JetBrainsMono Nerd Font", sans-serif;
}

.notification-row .notification-background .notification {
  background: #%s;
  color: #%s;
  border-radius: %dpx;
  border: 2px solid #%s;
  padding: 10px;
}

.control-center {
  background: #%sEE;
  color: #%s;
  border-radius: %dpx;
  border: 2px solid #%s;
  padding: 14px;
}

.widget-title > button {
  background: #%s;
  color: #%s;
  border-radius: 8px;
  padding: 4px 10px;
}
`,
		theme.Name,
		surf, txt, s.BorderRadius, p,
		bg, txt, s.BorderRadius, p,
		p, bg,
	)

	hyprlandConf := fmt.Sprintf(`# ============================================================================
# KitsuneDE // Hyprland Config (Perfil Alternativo) — %s (%s)
# ============================================================================

monitor=,preferred,auto,1
exec-once = waybar
exec-once = swww-daemon && swww img ~/.config/kitsune-de/wallpapers/%s --transition-type grow --transition-fps 60
exec-once = swaync

general {
    gaps_in = %d
    gaps_out = %d
    border_size = %d
    col.active_border = rgba(%see) rgba(%see) 45deg
    col.inactive_border = rgba(%s88)
    layout = dwindle
}

decoration {
    rounding = %d
    active_opacity = %s
    inactive_opacity = 0.86
    blur {
        enabled = %s
        size = 8
        passes = 3
    }
}

animations {
    enabled = %s
}

$mainMod = SUPER
bind = $mainMod, Return, exec, kitty
bind = $mainMod, Q, killactive,
bind = $mainMod, Space, exec, rofi -show drun
bind = $mainMod, V, togglefloating,
bind = $mainMod, T, exec, kitsune-ctl theme next
bind = $mainMod, 1, workspace, 1
bind = $mainMod, 2, workspace, 2
bind = $mainMod, 3, workspace, 3
bind = $mainMod, 4, workspace, 4
bind = $mainMod, 5, workspace, 5
`,
		theme.Name, theme.JapaneseName, wallFile,
		s.GapsIn, s.GapsOut, s.BorderSize, p, sec, surf,
		s.BorderRadius, opacity, blurBool, animStr,
	)

	var ansiLines strings.Builder
	for i, c := range theme.TermColors {
		ansiLines.WriteString(fmt.Sprintf("color%d  %s\ncolor%d %s\n", i, c, i+8, c))
	}

	kittyConf := fmt.Sprintf(`# KitsuneDE Kitty Terminal - %s (%s)
font_family      JetBrainsMono Nerd Font
font_size        11.5
window_padding_width 12
background_opacity %s
confirm_os_window_close 0

foreground       %s
background       %s
cursor           %s
selection_foreground %s
selection_background %s

%s`,
		theme.Name, theme.JapaneseName, opacity,
		theme.TextMain, theme.BgBase, theme.Primary, theme.BgBase, theme.Primary,
		ansiLines.String(),
	)

	rofiRasi := fmt.Sprintf(`/* KitsuneDE Rofi Wayland Anime Launcher - %s */
* {
    bg:      %sE6;
    surface: %s;
    primary: %s;
    accent:  %s;
    fg:      %s;
    muted:   %s;
    font:    "JetBrainsMono Nerd Font 12";
}

window {
    width: 560px;
    background-color: @bg;
    border: 2px;
    border-color: @primary;
    border-radius: %dpx;
    padding: 18px;
}

inputbar {
    children: [prompt, entry];
    background-color: @surface;
    border-radius: 10px;
    padding: 10px 14px;
    text-color: @fg;
}

prompt {
    text-color: @primary;
    padding: 0 10px 0 0;
}

listview {
    lines: 7;
    spacing: 6px;
    margin: 12px 0 0 0;
}

element selected {
    background-color: @primary;
    text-color: %s;
    border-radius: 8px;
}
`,
		theme.Name, theme.BgBase, theme.BgSurface, theme.Primary, theme.Secondary,
		theme.TextMain, theme.TextMuted, s.BorderRadius, theme.BgBase,
	)

	wmLabel := "KitsuneDE + SwayFX (Go)"
	if comp == "sway" {
		wmLabel = "KitsuneDE + Sway (Go)"
	} else if comp == "hyprland" {
		wmLabel = "KitsuneDE + Hyprland (Go)"
	}

	fastfetchConf := fmt.Sprintf(`{
  "$schema": "https://github.com/fastfetch-cli/fastfetch/raw/dev/doc/json_schema.json",
  "logo": {
    "type": "kitty",
    "source": "~/.config/kitsune-de/wallpapers/kitsune-avatar.jpg",
    "width": 28
  },
  "display": {
    "separator": "  ➜  ",
    "color": "%s"
  },
  "modules": [
    "title",
    "separator",
    { "type": "os", "key": "󰣇 OS" },
    { "type": "kernel", "key": "󰌽 Kernel" },
    { "type": "wm", "key": "󰖲 WM", "format": "%s" },
    { "type": "theme", "key": "🎨 Rice", "format": "%s" },
    { "type": "uptime", "key": "󰥔 Uptime" },
    { "type": "packages", "key": "󰏖 Packages" },
    { "type": "cpu", "key": "󰻠 CPU" },
    { "type": "memory", "key": "󰍛 Memory" },
    "break",
    "colors"
  ]
}
`, theme.Primary, wmLabel, theme.Name)

	return Bundle{
		ThemeID:    theme.ID,
		ThemeName:  theme.Name,
		Compositor: comp,
		Files: map[string]string{
			"sway/config":              swayBaseConf,
			"sway/swayfx-effects.conf": swayFXBlock,
			"swayfx/config":            swayFXFullConf,
			"waybar/config-sway.jsonc": waybarSwayConfig,
			"waybar/config.jsonc":      defaultWaybar,
			"waybar/style.css":         waybarCSS,
			"swaylock/config":          swaylockConf,
			"swaync/style.css":         swayncCSS,
			"kitty/kitty.conf":         kittyConf,
			"rofi/config.rasi":         rofiRasi,
			"fastfetch/config.jsonc":   fastfetchConf,
			"hypr/hyprland.conf":       hyprlandConf,
		},
	}
}

// ExportToDir writes all generated dotfiles to outDir (e.g., ~/.config or ./dotfiles).
func ExportToDir(outDir string, theme config.Theme, s config.DesktopSettings) ([]string, error) {
	bundle := Generate(theme, s)
	var written []string
	for rel, content := range bundle.Files {
		full := filepath.Join(outDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return written, err
		}
		written = append(written, full)
	}
	return written, nil
}
