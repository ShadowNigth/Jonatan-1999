package ricer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

// Bundle holds generated Arch Linux configuration files for Sway, SwayFX, Waybar, nwg-dock, GTK & Hyprland.
type Bundle struct {
	ThemeID    string            `json:"themeId"`
	ThemeName  string            `json:"themeName"`
	Compositor string            `json:"compositor"`
	Files      map[string]string `json:"files"`
}

func stripHex(c string) string {
	return strings.TrimPrefix(strings.TrimSpace(c), "#")
}

// Generate creates a complete visual twin set of Arch Linux dotfiles matching the KitsuneDE Web Preview.
func Generate(theme config.Theme, s config.DesktopSettings) Bundle {
	comp := s.Compositor
	if comp == "" {
		comp = "swayfx"
	}

	p := stripHex(theme.Primary)
	sec := stripHex(theme.Secondary)
	acc := stripHex(theme.Accent)
	bg := stripHex(theme.BgBase)
	surf := stripHex(theme.BgSurface)
	over := stripHex(theme.BgOverlay)
	txt := stripHex(theme.TextMain)
	muted := stripHex(theme.TextMuted)
	succ := stripHex(theme.Success)
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

	// SwayFX eye-candy block (Kawase blur, rounded corners, neon shadows, and layer_effects for Waybar + Dock + Rofi + SwayNC)
	swayFXBlock := fmt.Sprintf(`# ── Efectos Visuales Exclusivos de SwayFX (Idénticos al Preview Web) ─────────
corner_radius %d
smart_corner_radius off

blur %s
blur_xray disable
blur_passes 3
blur_radius 7
blur_noise 0.015
blur_brightness 0.96
blur_contrast 1.08
blur_saturation 1.18

shadows %s
shadows_on_csd enable
shadow_blur_radius 24
shadow_color #%s77
shadow_inactive_color #%sAA

default_dim_inactive 0.08
dim_inactive_colors.unfocused #%sAA
dim_inactive_colors.urgent #%sAA

# Cristal Blur en Barra Superior (Waybar), Dock Inferior (nwg-dock), Lanzador (Rofi) y SwayNC
layer_effects "waybar" "blur %s; shadows %s; corner_radius %d"
layer_effects "gtk-layer-shell" "blur %s; shadows %s; corner_radius 16"
layer_effects "rofi" "blur %s; shadows enable; corner_radius %d"
layer_effects "notifications" "blur %s; corner_radius %d"
`,
		s.BorderRadius,
		blurEnable,
		shadowEnable, p, bg,
		bg, danger,
		blurEnable, shadowEnable, s.BorderRadius,
		blurEnable, shadowEnable,
		blurEnable, s.BorderRadius,
		blurEnable, s.BorderRadius,
	)

	fxIncludeLine := "# Modo Sway estándar (sin directivas exclusivas de SwayFX para evitar avisos)"
	if comp == "swayfx" {
		fxIncludeLine = "include ~/.config/sway/swayfx-effects.conf"
	}

	swayStartDockSh := `#!/usr/bin/env bash
# Inicia el Dock flotante inferior (nwg-dock) idéntico al Dock Anime de KitsuneDE
pkill -f nwg-dock 2>/dev/null || true
sleep 0.4
if command -v nwg-dock >/dev/null 2>&1; then
  exec nwg-dock -d -p bottom -i 34 -mb 10 -ml 12 -mr 12 -s style.css -c "rofi -show drun"
fi
`

	swayBaseConf := fmt.Sprintf(`# ============================================================================
# 🦊 KitsuneDE // Configuración Completa para Sway & SwayFX generada en Go
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

# Fuente global con soporte Nerd Font + Kanji Japonés
font pango:JetBrainsMono Nerd Font, Noto Sans CJK JP SemiBold 10

# Tema de cursor moderno estilo Anime
seat seat0 xcursor_theme Bibata-Modern-Ice 24

# Variables de entorno Wayland & portales
exec dbus-update-activation-environment --systemd WAYLAND_DISPLAY XDG_CURRENT_DESKTOP=sway
exec /usr/lib/polkit-gnome/polkit-gnome-authentication-agent-1

# Demonio de Wallpapers Animados a 60 FPS (swww) + respaldo swaybg
output * bg ~/.config/kitsune-de/wallpapers/%s fill
exec_once swww-daemon
exec swww img ~/.config/kitsune-de/wallpapers/%s --transition-type grow --transition-pos 0.85,0.90 --transition-duration 1.2 --transition-fps 60

# Barra Superior Waybar + Dock Inferior nwg-dock + SwayNC + Autotiling + Motor Go KitsuneDE
exec waybar -c ~/.config/waybar/config-sway.jsonc -s ~/.config/waybar/style.css
exec_always ~/.config/sway/start-dock.sh
exec swaync
exec autotiling
exec kitsune-de --addr 127.0.0.1:8080 --compositor %s

# Teclado y Touchpad
input "type:keyboard" {
    xkb_layout "latam,us"
    xkb_options "grp:alt_shift_toggle"
    repeat_delay 250
    repeat_rate 40
}

input "type:touchpad" {
    tap enabled
    natural_scroll enabled
    dwt enabled
}

# Gaps y Bordes Luminosos estilo Anime
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

# Transparencia de ventanas y reglas flotantes para el Panel Kitsune Studio
for_window [app_id="kitty"] opacity %s
for_window [app_id="thunar"] opacity 0.94
for_window [app_id="pavucontrol"] floating enable, resize set 760 500, move position center
for_window [app_id="blueman-manager"] floating enable
for_window [title="KitsuneDE.*"] floating enable, resize set 1120 720, move position center
for_window [window_role="pop-up"] floating enable
for_window [window_role="dialog"] floating enable

# ── Atajos Principales ($mod = Super, también compatibles con Alt) ──────────
bindsym $mod+Return exec $term
bindsym $mod+q kill
bindsym $mod+space exec $menu
bindsym $mod+d exec $menu
bindsym $mod+e exec kitty -e yazi
bindsym $mod+b exec kitty -e btop
bindsym $mod+n exec swaync-client -t -sw
bindsym $mod+l exec swaylock -f -C ~/.config/swaylock/config
bindsym $mod+x exec swaylock -f -C ~/.config/swaylock/config
bindsym $mod+t exec kitsune-ctl theme next
bindsym $mod+g exec chromium --ozone-platform-hint=auto --app=http://127.0.0.1:8080
bindsym $mod+Shift+d exec kitsune-de --kiosk

# Capturas de pantalla en Wayland (grim + slurp -> portapapeles)
bindsym Print exec grim -g "$(slurp)" - | wl-copy && notify-send "🦊 KitsuneDE" "Captura copiada al portapapeles"

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

# ── Espacios de Trabajo Kanji (1..5: 一 二 三 四 五) ─────────────────────────
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
bindsym $mod+Shift+b splith
bindsym $mod+v splitv
bindsym $mod+s layout stacking
bindsym $mod+w layout tabbed
bindsym $mod+a layout toggle split
bindsym $mod+f fullscreen
bindsym $mod+Shift+space floating toggle
bindsym $mod+Ctrl+space focus mode_toggle

# Scratchpad rápido
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

# Inclusión de efectos SwayFX
%s
`,
		theme.Name, theme.JapaneseName, strings.ToUpper(comp),
		wallFile, wallFile, comp,
		s.GapsIn, s.GapsOut, s.BorderSize, s.BorderSize,
		p, surf, txt, sec, p,
		over, bg, muted, over, over,
		over, bg, muted, over, over,
		danger, surf, txt, danger, danger,
		opacity,
		fxIncludeLine,
	)

	swayFXFullConf := strings.Replace(swayBaseConf, fxIncludeLine, swayFXBlock, 1)

	// Waybar config matching the exact visual structure of the Web Preview!
	waybarSwayConfig := fmt.Sprintf(`{
  "layer": "top",
  "position": "top",
  "height": 42,
  "margin-top": 10,
  "margin-left": 14,
  "margin-right": 14,
  "modules-left": ["custom/arch", "sway/workspaces", "custom/compositor", "sway/mode", "sway/window"],
  "modules-center": ["custom/synth", "clock"],
  "modules-right": ["custom/kitsune-theme", "cpu", "memory", "pulseaudio", "custom/swaync", "custom/lock"],
  "custom/arch": {
    "format": "󰣇  KitsuneDE",
    "on-click": "rofi -show drun",
    "tooltip-format": "Abrir Lanzador Rofi (Super+Space)"
  },
  "sway/workspaces": {
    "disable-scroll": false,
    "all-outputs": true,
    "format": "{icon}",
    "persistent-workspaces": {
      "1": [],
      "2": [],
      "3": [],
      "4": [],
      "5": []
    },
    "format-icons": {
      "1": "一",
      "2": "二",
      "3": "三",
      "4": "四",
      "5": "五",
      "urgent": "",
      "default": "狐"
    }
  },
  "custom/compositor": {
    "format": "✨ %s",
    "on-click": "swaymsg layout toggle split",
    "on-click-right": "swaymsg layout tabbed",
    "tooltip-format": "Clic izq: Split Tiling • Clic der: Sway Tabbed"
  },
  "sway/mode": {
    "format": "󰩨  {}",
    "tooltip": false
  },
  "sway/window": {
    "format": "🦊 {}",
    "max-length": 36,
    "tooltip": false
  },
  "custom/synth": {
    "format": "🎵 Kitsune Studio",
    "on-click": "chromium --ozone-platform-hint=auto --app=http://127.0.0.1:8080",
    "tooltip-format": "Abrir Panel KitsuneDE & Synth Lo-Fi"
  },
  "clock": {
    "format": "󰥔  {:%%H:%%M  •  %%d %%b}",
    "tooltip-format": "<big>{:%%Y %%B}</big>\n<tt>{calendar}</tt>"
  },
  "custom/kitsune-theme": {
    "format": "🎨 %s",
    "on-click": "kitsune-ctl theme next",
    "on-click-right": "chromium --ozone-platform-hint=auto --app=http://127.0.0.1:8080",
    "tooltip-format": "Clic izq: Siguiente Tema Anime en vivo • Clic der: Rice Studio"
  },
  "cpu": {
    "interval": 2,
    "format": "CPU {usage}%%",
    "on-click": "kitty -e btop"
  },
  "memory": {
    "interval": 3,
    "format": "RAM {percentage}%%",
    "on-click": "kitty -e btop"
  },
  "pulseaudio": {
    "format": "󰕾 {volume}%%",
    "format-muted": "󰝟 Mute",
    "on-click": "pavucontrol"
  },
  "custom/swaync": {
    "format": "🔔",
    "on-click": "swaync-client -t -sw",
    "tooltip-format": "Centro de Notificaciones SwayNC"
  },
  "custom/lock": {
    "format": "🔒",
    "on-click": "swaylock -f -C ~/.config/swaylock/config",
    "tooltip-format": "Bloquear pantalla estilo Anime (Super+L)"
  }
}
`, strings.ToUpper(comp), theme.Name)

	waybarHyprConfig := strings.ReplaceAll(waybarSwayConfig, `"sway/workspaces"`, `"hyprland/workspaces"`)
	waybarHyprConfig = strings.ReplaceAll(waybarHyprConfig, `"sway/window"`, `"hyprland/window"`)

	defaultWaybar := waybarSwayConfig
	if comp == "hyprland" {
		defaultWaybar = waybarHyprConfig
	}

	waybarCSS := fmt.Sprintf(`/* ============================================================================
 * KitsuneDE Waybar CSS — Gemelo Visual Exacto del Preview Web (%s)
 * ============================================================================ */
* {
  font-family: "JetBrainsMono Nerd Font", "Noto Sans CJK JP", "Noto Color Emoji", monospace;
  font-size: 12.5px;
  font-weight: 600;
  border: none;
  min-height: 0;
}

window#waybar {
  background: rgba(11, 9, 22, 0.78);
  color: #%s;
  border-radius: %dpx;
  border: 1px solid rgba(255, 255, 255, 0.12);
}

#custom-arch {
  background: linear-gradient(135deg, #%s, #%s);
  color: #090812;
  padding: 4px 14px;
  margin: 5px 4px 5px 8px;
  border-radius: 9px;
  font-weight: 800;
}

#workspaces {
  background: #%s;
  margin: 5px 4px;
  padding: 2px 4px;
  border-radius: 9px;
  border: 1px solid rgba(255, 255, 255, 0.06);
}

#workspaces button {
  color: #%s;
  padding: 2px 9px;
  margin: 2px;
  border-radius: 6px;
  background: transparent;
  font-weight: 700;
  transition: all 0.18s ease;
}

#workspaces button:hover {
  color: #%s;
  background: rgba(255, 255, 255, 0.08);
}

#workspaces button.focused,
#workspaces button.active {
  color: #090812;
  background: #%s;
  box-shadow: 0 0 10px #%s;
}

#workspaces button.urgent {
  background: #%s;
  color: #ffffff;
}

#custom-compositor {
  background: #%s;
  color: #%s;
  border: 1px solid #%s;
  padding: 4px 11px;
  margin: 5px 3px;
  border-radius: 9px;
}

#custom-kitsune-theme {
  background: #%s;
  color: #%s;
  border: 1px solid #%s;
  padding: 4px 11px;
  margin: 5px 3px;
  border-radius: 9px;
}

#mode {
  background: #%s;
  color: #090812;
  padding: 4px 10px;
  margin: 5px 3px;
  border-radius: 9px;
  font-weight: 800;
}

#window, #custom-synth, #clock, #cpu, #memory, #pulseaudio, #custom-swaync, #custom-lock {
  background: #%s;
  color: #%s;
  padding: 4px 11px;
  margin: 5px 3px;
  border-radius: 9px;
  border: 1px solid rgba(255, 255, 255, 0.07);
}

#cpu, #memory {
  color: #%s;
}

#custom-lock {
  margin-right: 8px;
}
`,
		theme.Name,
		txt, s.BorderRadius,
		p, sec,
		surf,
		muted,
		txt,
		p, p,
		danger,
		surf, p, p,
		surf, txt, sec,
		sec,
		surf, txt,
		p,
	)

	// nwg-dock CSS matching the floating bottom Anime Dock in the web preview!
	nwgDockCSS := fmt.Sprintf(`/* KitsuneDE Floating Anime Dock (nwg-dock) - %s */
window {
  background: rgba(11, 9, 22, 0.80);
  border-radius: 16px;
  border: 1px solid #%s;
}

#box {
  padding: 6px 10px;
}

button, image {
  background: none;
  border: none;
  box-shadow: none;
}

button {
  padding: 5px;
  margin: 2px 4px;
  border-radius: 11px;
  background: #%s;
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.18s ease;
}

button:hover {
  background: #%s;
  border: 1px solid #%s;
  box-shadow: 0 0 12px #%s;
}
`, theme.Name, p, surf, over, p, p)

	// Swaylock-Effects config with screenshot/wallpaper blur, clock, and glowing ring
	swaylockConf := fmt.Sprintf(`# KitsuneDE Swaylock-Effects Anime Lockscreen - %s
image=~/.config/kitsune-de/wallpapers/%s
scaling=fill
clock
timestr=%%H:%%M
datestr=%%A, %%d %%B
indicator
indicator-radius=115
indicator-thickness=9
effect-blur=9x6
effect-vignette=0.5:0.5
fade-in=0.25
font=JetBrainsMono Nerd Font

inside-color=%sCC
ring-color=%sFF
line-color=00000000
key-hl-color=%sFF
bs-hl-color=%sFF
separator-color=00000000
text-color=%sFF

inside-clear-color=%sCC
ring-clear-color=%sFF
text-clear-color=%sFF

inside-ver-color=%sCC
ring-ver-color=%sFF
text-ver-color=%sFF

inside-wrong-color=%sCC
ring-wrong-color=%sFF
text-wrong-color=%sFF
`,
		theme.Name, wallFile,
		surf, p, sec, danger, txt,
		surf, acc, txt,
		surf, succ, txt,
		surf, danger, txt,
	)

	swayncCSS := fmt.Sprintf(`/* KitsuneDE SwayNC Anime Notification Center - %s */
* {
  font-family: "JetBrainsMono Nerd Font", "Noto Sans CJK JP", sans-serif;
}

.notification-row .notification-background .notification {
  background: #%s;
  color: #%s;
  border-radius: %dpx;
  border: 1.5px solid #%s;
  padding: 10px 12px;
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.55);
}

.control-center {
  background: #%sEB;
  color: #%s;
  border-radius: 16px;
  border: 1.5px solid #%s;
  padding: 14px;
}

.widget-title > button {
  background: linear-gradient(135deg, #%s, #%s);
  color: #090812;
  font-weight: 800;
  border-radius: 8px;
  padding: 4px 12px;
}
`,
		theme.Name,
		surf, txt, s.BorderRadius, p,
		bg, txt, p,
		p, sec,
	)

	// GTK-3.0 & GTK-4.0 Anime Dark Theme so Thunar, Pavucontrol, Dialogs match the exact palette!
	gtkSettingsIni := `[Settings]
gtk-application-prefer-dark-theme=1
gtk-theme-name=Adwaita-dark
gtk-icon-theme-name=Papirus-Dark
gtk-font-name=Inter 10
gtk-cursor-theme-name=Bibata-Modern-Ice
gtk-cursor-theme-size=24
`

	gtkAnimeCSS := fmt.Sprintf(`/* KitsuneDE GTK3/GTK4 Anime Theme Override - %s */
@define-color accent_color #%s;
@define-color accent_bg_color #%s;
@define-color accent_fg_color #090812;
@define-color window_bg_color #%s;
@define-color window_fg_color #%s;
@define-color view_bg_color #%s;
@define-color view_fg_color #%s;
@define-color headerbar_bg_color #%s;
@define-color headerbar_fg_color #%s;
@define-color card_bg_color #%s;
@define-color card_fg_color #%s;
@define-color popover_bg_color #%s;
@define-color popover_fg_color #%s;

window, .background {
  background-color: #%s;
  color: #%s;
}

headerbar {
  background: #%s;
  color: #%s;
  border-bottom: 1px solid #%s;
}

*:selected, selection {
  background-color: #%s;
  color: #090812;
}
`,
		theme.Name,
		p, p,
		bg, txt,
		surf, txt,
		surf, txt,
		over, txt,
		surf, txt,
		bg, txt,
		surf, txt, p,
		p,
	)

	hyprlandConf := fmt.Sprintf(`# ============================================================================
# KitsuneDE // Hyprland Config Con Animaciones Fluidas — %s (%s)
# ============================================================================

monitor=,preferred,auto,1
exec-once = waybar
exec-once = ~/.config/sway/start-dock.sh
exec-once = swww-daemon && swww img ~/.config/kitsune-de/wallpapers/%s --transition-type grow --transition-fps 60
exec-once = swaync
exec-once = kitsune-de --addr 127.0.0.1:8080 --compositor hyprland

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
    inactive_opacity = 0.88
    blur {
        enabled = %s
        size = 8
        passes = 3
        vibrancy = 0.22
    }
}

animations {
    enabled = %s
    bezier = kitsuneCurve, 0.05, 0.9, 0.1, 1.05
    animation = windows, 1, 6, kitsuneCurve
    animation = borderangle, 1, 45, default, loop
    animation = workspaces, 1, 5, kitsuneCurve, slide
}

$mainMod = SUPER
bind = $mainMod, Return, exec, kitty
bind = $mainMod, Q, killactive,
bind = $mainMod, Space, exec, rofi -show drun
bind = $mainMod, V, togglefloating,
bind = $mainMod, T, exec, kitsune-ctl theme next
bind = $mainMod, L, exec, swaylock -f -C ~/.config/swaylock/config
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
bold_font        auto
italic_font      auto
font_size        11.5
window_padding_width 12
background_opacity %s
confirm_os_window_close 0
enable_audio_bell no
cursor_shape beam

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
configuration {
    modi: "drun,run,window";
    show-icons: true;
    icon-theme: "Papirus-Dark";
    display-drun: "󰣇  drun ❯";
    display-run: "❯_ run";
    display-window: "󰖲  win";
    drun-display-format: "{name}";
}

* {
    bg:      %sEB;
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
    padding: 16px;
}

inputbar {
    children: [prompt, entry];
    background-color: @surface;
    border-radius: 10px;
    padding: 10px 14px;
    text-color: @fg;
}

prompt {
    background-color: transparent;
    text-color: @primary;
    padding: 0 10px 0 0;
}

entry {
    background-color: transparent;
    text-color: @fg;
    placeholder: "Buscar aplicación, tema o comando...";
    placeholder-color: @muted;
}

listview {
    lines: 7;
    spacing: 6px;
    margin: 12px 0 0 0;
    background-color: transparent;
}

element {
    padding: 9px 12px;
    border-radius: 9px;
    background-color: transparent;
    text-color: @fg;
}

element selected {
    background-color: @primary;
    text-color: #090812;
    border-radius: 9px;
}

element-icon {
    size: 22px;
    margin: 0 10px 0 0;
    background-color: transparent;
}

element-text {
    background-color: transparent;
    text-color: inherit;
}
`,
		theme.Name, theme.BgBase, theme.BgSurface, theme.Primary, theme.Secondary,
		theme.TextMain, theme.TextMuted, s.BorderRadius,
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
			"sway/start-dock.sh":       swayStartDockSh,
			"swayfx/config":            swayFXFullConf,
			"waybar/config-sway.jsonc": waybarSwayConfig,
			"waybar/config.jsonc":      defaultWaybar,
			"waybar/style.css":         waybarCSS,
			"nwg-dock/style.css":       nwgDockCSS,
			"swaylock/config":          swaylockConf,
			"swaync/style.css":         swayncCSS,
			"gtk-3.0/settings.ini":     gtkSettingsIni,
			"gtk-3.0/gtk.css":          gtkAnimeCSS,
			"gtk-4.0/gtk.css":          gtkAnimeCSS,
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
		perm := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			perm = 0o755
		}
		if err := os.WriteFile(full, []byte(content), perm); err != nil {
			return written, err
		}
		written = append(written, full)
	}
	return written, nil
}
