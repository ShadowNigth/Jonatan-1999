package ricer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

// Bundle holds generated Arch Linux configuration files for a given Anime theme.
type Bundle struct {
	ThemeID   string            `json:"themeId"`
	ThemeName string            `json:"themeName"`
	Files     map[string]string `json:"files"`
}

func stripHex(c string) string {
	return strings.TrimPrefix(strings.TrimSpace(c), "#")
}

// Generate creates a complete set of Arch Linux dotfiles (Hyprland, Waybar, Kitty, Rofi, Fastfetch).
func Generate(theme config.Theme, s config.DesktopSettings) Bundle {
	p := stripHex(theme.Primary)
	sec := stripHex(theme.Secondary)
	bg := stripHex(theme.BgBase)
	surf := stripHex(theme.BgSurface)
	txt := stripHex(theme.TextMain)
	muted := stripHex(theme.TextMuted)
	opacity := fmt.Sprintf("%.2f", float64(s.WindowOpacity)/100.0)
	blurStr := "true"
	if !s.BlurEnabled {
		blurStr = "false"
	}
	animStr := "yes"
	if !s.AnimationsEnabled {
		animStr = "no"
	}

	hyprlandConf := fmt.Sprintf(`# ============================================================================
# KitsuneDE // Generado automáticamente en Go
# Tema Anime: %s (%s)
# ============================================================================

monitor=,preferred,auto,1

exec-once = waybar
exec-once = swww-daemon && swww img ~/.config/kitsune-de/wallpapers/%s --transition-type grow --transition-fps 60
exec-once = kitsune-de --daemon

env = XCURSOR_SIZE,24
env = QT_QPA_PLATFORMTHEME,qt6ct

input {
    kb_layout = latam,us
    follow_mouse = 1
    sensitivity = 0
}

general {
    gaps_in = %d
    gaps_out = %d
    border_size = %d
    col.active_border = rgba(%see) rgba(%see) 45deg
    col.inactive_border = rgba(%s88)
    layout = dwindle
    allow_tearing = false
}

decoration {
    rounding = %d
    active_opacity = %s
    inactive_opacity = 0.86

    blur {
        enabled = %s
        size = 8
        passes = 3
        new_optimizations = true
        vibrancy = 0.22
    }

    shadow {
        enabled = true
        range = 22
        render_power = 3
        color = rgba(%s66)
    }
}

animations {
    enabled = %s
    bezier = kitsuneCurve, 0.05, 0.9, 0.1, 1.05
    animation = windows, 1, 6, kitsuneCurve
    animation = windowsOut, 1, 5, default, popin 80%%
    animation = border, 1, 10, default
    animation = borderangle, 1, 45, default, loop
    animation = fade, 1, 6, default
    animation = workspaces, 1, 5, kitsuneCurve, slide
}

dwindle {
    pseudotile = true
    preserve_split = true
}

$mainMod = SUPER

bind = $mainMod, Return, exec, kitty
bind = $mainMod, Q, killactive,
bind = $mainMod, M, exit,
bind = $mainMod, E, exec, kitty -e yazi
bind = $mainMod, V, togglefloating,
bind = $mainMod, Space, exec, rofi -show drun
bind = $mainMod, F, fullscreen, 0
bind = $mainMod, T, exec, kitsune-ctl theme next
bind = $mainMod, D, exec, kitsune-de --kiosk

bind = $mainMod, 1, workspace, 1
bind = $mainMod, 2, workspace, 2
bind = $mainMod, 3, workspace, 3
bind = $mainMod, 4, workspace, 4
bind = $mainMod, 5, workspace, 5

bind = $mainMod SHIFT, 1, movetoworkspace, 1
bind = $mainMod SHIFT, 2, movetoworkspace, 2
bind = $mainMod SHIFT, 3, movetoworkspace, 3
bind = $mainMod SHIFT, 4, movetoworkspace, 4
bind = $mainMod SHIFT, 5, movetoworkspace, 5

bindm = $mainMod, mouse:272, movewindow
bindm = $mainMod, mouse:273, resizewindow
`,
		theme.Name, theme.JapaneseName,
		filepath.Base(theme.Wallpaper),
		s.GapsIn, s.GapsOut, s.BorderSize,
		p, sec, surf,
		s.BorderRadius, opacity, blurStr, p, animStr,
	)

	waybarConfig := `{
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
    "format": "󰥔  {:%H:%M  •  %d/%m}",
    "tooltip-format": "<big>{:%Y %B}</big>\n<tt>{calendar}</tt>"
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

	waybarCSS := fmt.Sprintf(`/* KitsuneDE Waybar CSS - %s */
* {
  font-family: "JetBrainsMono Nerd Font", "Noto Sans CJK JP", monospace;
  font-size: 13px;
  font-weight: 600;
  border: none;
}

window#waybar {
  background: rgba(15, 13, 28, 0.72);
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

#workspaces button.active {
  color: #%s;
  background: #%s;
  box-shadow: 0 0 12px #%s;
}

#clock, #cpu, #memory, #pulseaudio, #network, #custom-kitsune {
  background: #%s;
  color: #%s;
  padding: 4px 12px;
  margin: 4px 4px;
  border-radius: 9px;
}
`,
		theme.Name, txt, s.BorderRadius, p, p, sec, bg, muted, surf, bg, p, p, surf, txt,
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

	rofiRasi := fmt.Sprintf(`/* KitsuneDE Rofi Anime Launcher - %s */
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
    { "type": "wm", "key": "󱂬 DE/WM", "format": "KitsuneDE (Go + Hyprland)" },
    { "type": "theme", "key": "🎨 Rice", "format": "%s" },
    { "type": "uptime", "key": "󰥔 Uptime" },
    { "type": "packages", "key": "󰏖 Packages" },
    { "type": "cpu", "key": "󰻠 CPU" },
    { "type": "memory", "key": "󰍛 Memory" },
    "break",
    "colors"
  ]
}
`, theme.Primary, theme.Name)

	return Bundle{
		ThemeID:   theme.ID,
		ThemeName: theme.Name,
		Files: map[string]string{
			"hypr/hyprland.conf":      hyprlandConf,
			"waybar/config.jsonc":     waybarConfig,
			"waybar/style.css":        waybarCSS,
			"kitty/kitty.conf":        kittyConf,
			"rofi/config.rasi":        rofiRasi,
			"fastfetch/config.jsonc":  fastfetchConf,
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
