package ricer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ShadowNigth/Jonatan-1999/internal/config"
)

// Bundle contains pure Kitsune Go Wayland & companion Arch Linux app configs.
type Bundle struct {
	ThemeID   string            `json:"themeId"`
	ThemeName string            `json:"themeName"`
	Engine    string            `json:"engine"`
	Files     map[string]string `json:"files"`
}

func hexNoHash(h string) string {
	return strings.TrimPrefix(strings.TrimSpace(h), "#")
}

// Generate builds all pure Kitsune Go Wayland + companion app dotfiles (zero Sway/SwayFX/Hyprland/Dock).
func Generate(t config.Theme, s config.DesktopSettings) Bundle {
	files := map[string]string{
		"kitsune-wayland/config.toml":  genKitsuneWaylandConfig(t, s),
		"kitsune-wayland/waybar.jsonc": genWaybarConfig(t, s),
		"kitsune-wayland/waybar.css":   genWaybarCSS(t, s),
		"rofi/config.rasi":             genRofiRasi(t, s),
		"btop/themes/kitsune.theme":    genBtopTheme(t),
		"yazi/theme.toml":              genYaziTheme(t),
		"nvim/init.lua":                genNvimInitLua(t),
		"kitty/kitty.conf":             genKittyConf(t, s),
		"fastfetch/config.jsonc":       genFastfetchJSON(t),
		"gtk-3.0/settings.ini":         genGTKSettingsINI(),
		"gtk-4.0/gtk.css":              genGTKCSS(t),
	}

	return Bundle{
		ThemeID:   t.ID,
		ThemeName: t.Name,
		Engine:    "kitsune-go-wayland",
		Files:     files,
	}
}

// ExportToDir writes all generated dotfiles safely into targetDir.
func ExportToDir(targetDir string, t config.Theme, s config.DesktopSettings) ([]string, error) {
	bundle := Generate(t, s)
	keys := make([]string, 0, len(bundle.Files))
	for k := range bundle.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	written := make([]string, 0, len(keys))
	for _, rel := range keys {
		content := bundle.Files[rel]
		full := filepath.Join(targetDir, filepath.FromSlash(rel))
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

func genKitsuneWaylandConfig(t config.Theme, s config.DesktopSettings) string {
	return fmt.Sprintf(`# ============================================================================
# 🦊 Kitsune Go Wayland — Configuración Principal del Escritorio Anime
# Tema activo : %s (%s) — %s
# Modo        : %s (Sin Dock inferior • Barra superior Waybar + Rofi)
# ============================================================================

[engine]
name = "Kitsune Go Wayland"
listen_addr = "127.0.0.1:18080"
wayland_backend = "cage-ozone-wayland"
dock_enabled = false
top_bar = "waybar-kitsune"
launcher = "rofi-kitsune"

[theme]
id = "%s"
name = "%s"
japanese_name = "%s"
wallpaper = "~/.config/kitsune-de/wallpapers/%s"
particle_mode = "%s"
particles_enabled = %v
animations_enabled = %v

[colors]
primary = "%s"
secondary = "%s"
accent = "%s"
bg_base = "%s"
bg_surface = "%s"
bg_overlay = "%s"
text_main = "%s"
text_muted = "%s"

[windows]
default_layout = "%s"
workspaces = ["一", "二", "三", "四", "五"]
gaps_in = %d
gaps_out = %d
border_size = %d
border_radius = %d
window_opacity = %d

[effects]
blur_enabled = %v
blur_radius = %d
border_glow = %v
glow_intensity = %d

[apps]
rice_studio = "kitsune-ctl open settings"
btop_monitor = "kitsune-ctl open sysmon"
yazi_files = "kitsune-ctl open files"
neovim_go_studio = "kitsune-ctl open nvim"
terminal = "kitsune-ctl open terminal"

[keybindings]
"Alt+Space" = "launcher:rofi"
"Alt+Return" = "app:terminal"
"Alt+W" = "layout:tabbed"
"Alt+T" = "layout:toggle"
"Alt+V" = "window:toggle_floating"
"Alt+Q" = "window:close"
"Alt+Tab" = "tab:next"
"Alt+1..5" = "workspace:switch"
`,
		t.Name, t.JapaneseName, t.Subtitle,
		strings.ToUpper(s.LayoutMode),
		t.ID, t.Name, t.JapaneseName, filepath.Base(t.Wallpaper),
		t.ParticleMode, s.ParticlesEnabled, s.AnimationsEnabled,
		t.Primary, t.Secondary, t.Accent, t.BgBase, t.BgSurface, t.BgOverlay, t.TextMain, t.TextMuted,
		s.LayoutMode, s.GapsIn, s.GapsOut, s.BorderSize, s.BorderRadius, s.WindowOpacity,
		s.BlurEnabled, s.BlurRadius, s.BorderGlow, s.GlowIntensity,
	)
}

func genWaybarConfig(t config.Theme, s config.DesktopSettings) string {
	return fmt.Sprintf(`// 🦊 Kitsune Go Wayland — Barra Superior Estilo Waybar (Workspaces Kanji 一 二 三 四 五)
{
  "layer": "top",
  "position": "top",
  "height": 42,
  "margin-top": 10,
  "margin-left": %d,
  "margin-right": %d,
  "spacing": 7,

  "modules-left": [
    "custom/rofi",
    "custom/workspaces",
    "custom/layout",
    "custom/apps"
  ],
  "modules-center": [
    "custom/synth",
    "clock"
  ],
  "modules-right": [
    "custom/theme",
    "cpu",
    "memory",
    "custom/notifications",
    "custom/lock"
  ],

  "custom/rofi": {
    "format": "󰣇  Kitsune Go",
    "on-click": "kitsune-ctl open rofi",
    "tooltip-format": "Abrir Lanzador Rofi (Alt+Space)"
  },
  "custom/workspaces": {
    "format": "一  二  三  四  五",
    "tooltip-format": "Espacios de trabajo Kanji (Alt+1..5)"
  },
  "custom/layout": {
    "format": "🗂️ %s",
    "on-click": "kitsune-ctl layout toggle",
    "on-click-right": "kitsune-ctl layout tabbed",
    "tooltip-format": "Modo Tabbed / Tiling / Flotante"
  },
  "custom/apps": {
    "format": "🎨 Studio  📊 Btop++  📁 Yazi  📝 Nvim",
    "tooltip-format": "Suite integrada en pestañas Kitsune Go Wayland"
  },
  "clock": {
    "format": "󰥔  {:%%H:%%M  •  %%d %%b}"
  },
  "custom/theme": {
    "format": "🌸 %s",
    "on-click": "kitsune-ctl theme next",
    "tooltip-format": "Cambiar tema Anime en vivo"
  }
}
`, s.GapsOut, s.GapsOut, strings.ToUpper(s.LayoutMode), t.Name)
}

func genWaybarCSS(t config.Theme, s config.DesktopSettings) string {
	blurPx := 0
	if s.BlurEnabled {
		blurPx = s.BlurRadius
	}
	return fmt.Sprintf(`/* 🦊 Kitsune Go Wayland — Waybar Top Bar Stylesheet (%s) */
* {
  font-family: "JetBrainsMono Nerd Font", "Inter", "Noto Sans CJK JP", sans-serif;
  font-size: 12px;
  font-weight: 600;
}

window#waybar {
  background: rgba(11, 9, 22, 0.78);
  color: %s;
  border-radius: %dpx;
  border: %dpx solid %s;
  backdrop-filter: blur(%dpx);
}

#workspaces button.active {
  background: linear-gradient(135deg, %s, %s);
  color: #090812;
  font-weight: 800;
  border-radius: 7px;
}
`, t.Name, t.TextMain, s.BorderRadius, s.BorderSize, t.Primary, blurPx, t.Primary, t.Secondary)
}

func genRofiRasi(t config.Theme, s config.DesktopSettings) string {
	return fmt.Sprintf(`/* 🦊 Kitsune Go Wayland — Lanzador Rofi (%s • %s) */
configuration {
  modi: "drun,run";
  show-icons: true;
  icon-theme: "Papirus-Dark";
  display-drun: "󰣇  Kitsune Go ❯";
  drun-display-format: "{name}";
  font: "JetBrainsMono Nerd Font 11";
}

* {
  bg: %sEA;
  bg-alt: %sF2;
  fg: %s;
  fg-muted: %s;
  accent: %s;
  secondary: %s;
  background-color: transparent;
  text-color: @fg;
}

window {
  width: 620px;
  background-color: @bg;
  border: %dpx;
  border-color: @accent;
  border-radius: %dpx;
  padding: 18px;
}

inputbar {
  children: [prompt, entry];
  background-color: @bg-alt;
  border-radius: 10px;
  padding: 11px 14px;
  spacing: 10px;
  border: 1px;
  border-color: @secondary;
}

prompt {
  text-color: @accent;
}

listview {
  columns: 2;
  lines: 7;
  spacing: 8px;
  margin: 14px 0 0 0;
}

element {
  padding: 10px 12px;
  border-radius: 9px;
}

element selected.normal {
  background-color: @bg-alt;
  border: 1px;
  border-color: @accent;
  text-color: @accent;
}
`, t.Name, t.JapaneseName, t.BgBase, t.BgSurface, t.TextMain, t.TextMuted, t.Primary, t.Secondary, s.BorderSize, s.BorderRadius)
}

func genBtopTheme(t config.Theme) string {
	return fmt.Sprintf(`# 🦊 Kitsune Go Wayland — Tema Oficial Btop++ (%s • %s)
theme[main_bg]="%s"
theme[main_fg]="%s"
theme[title]="%s"
theme[hi_fg]="%s"
theme[selected_bg]="%s"
theme[selected_fg]="%s"
theme[inactive_fg]="%s"
theme[proc_misc]="%s"
theme[cpu_box]="%s"
theme[mem_box]="%s"
theme[net_box]="%s"
theme[proc_box]="%s"
theme[div_line]="%s"
theme[cpu_start]="%s"
theme[cpu_mid]="%s"
theme[cpu_end]="%s"
theme[mem_start]="%s"
theme[mem_mid]="%s"
theme[mem_end]="%s"
`,
		t.Name, t.JapaneseName,
		t.BgBase, t.TextMain, t.Primary, t.Secondary,
		t.BgOverlay, t.Primary, t.TextMuted, t.Accent,
		t.Primary, t.Secondary, t.Accent, t.Primary, t.BgOverlay,
		t.Success, t.Primary, t.Secondary,
		t.Primary, t.Accent, t.Secondary,
	)
}

func genYaziTheme(t config.Theme) string {
	return fmt.Sprintf(`# 🦊 Kitsune Go Wayland — Tema Oficial Yazi File Manager (%s • %s)
[manager]
cwd = { fg = "%s", bold = true }
hovered = { fg = "#090812", bg = "%s", bold = true }
preview_hovered = { underline = true }
find_keyword = { fg = "%s", bold = true }
marker_selected = { fg = "%s", bg = "%s" }
border_symbol = "│"
border_style = { fg = "%s" }

[status]
separator_open = ""
separator_close = ""
mode_normal = { fg = "#090812", bg = "%s", bold = true }
mode_select = { fg = "#090812", bg = "%s", bold = true }
`,
		t.Name, t.JapaneseName,
		t.Primary, t.Primary, t.Secondary, t.Success, t.Success, t.Primary,
		t.Primary, t.Secondary,
	)
}

func genNvimInitLua(t config.Theme) string {
	return fmt.Sprintf(`-- 🦊 Kitsune Go Wayland — Neovim Go Studio (%s • %s)
vim.opt.number = true
vim.opt.relativenumber = true
vim.opt.termguicolors = true
vim.opt.tabstop = 4
vim.opt.shiftwidth = 4
vim.opt.expandtab = false
vim.opt.cursorline = true
vim.opt.signcolumn = "yes"

-- Paleta Anime sincronizada por Kitsune Rice Studio
local colors = {
  bg      = "%s",
  surface = "%s",
  fg      = "%s",
  muted   = "%s",
  primary = "%s",
  second  = "%s",
  accent  = "%s",
}

vim.api.nvim_set_hl(0, "Normal",       { fg = colors.fg, bg = colors.bg })
vim.api.nvim_set_hl(0, "CursorLine",   { bg = colors.surface })
vim.api.nvim_set_hl(0, "LineNr",       { fg = colors.muted })
vim.api.nvim_set_hl(0, "CursorLineNr", { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "Keyword",      { fg = colors.second, bold = true })
vim.api.nvim_set_hl(0, "Function",     { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "String",       { fg = "%s" })
vim.api.nvim_set_hl(0, "Comment",      { fg = colors.muted, italic = true })
vim.api.nvim_set_hl(0, "StatusLine",   { fg = "#090812", bg = colors.primary, bold = true })
`,
		t.Name, t.JapaneseName,
		t.BgBase, t.BgSurface, t.TextMain, t.TextMuted, t.Primary, t.Secondary, t.Accent, t.Success,
	)
}

func genKittyConf(t config.Theme, s config.DesktopSettings) string {
	op := fmt.Sprintf("%.2f", float64(s.WindowOpacity)/100.0)
	tc := t.TermColors
	for len(tc) < 8 {
		tc = append(tc, "#ffffff")
	}
	return fmt.Sprintf(`# 🦊 Kitsune Go Wayland — Kitty Terminal (%s • %s)
font_family                 JetBrainsMono Nerd Font
font_size                   11.5
window_padding_width        12
background_opacity          %s
confirm_os_window_close     0

foreground                  %s
background                  %s
cursor                      %s
selection_foreground        %s
selection_background        %s

color0  %s
color8  #4c566a
color1  %s
color9  %s
color2  %s
color10 %s
color3  %s
color11 %s
color4  %s
color12 %s
color5  %s
color13 %s
color6  %s
color14 %s
color7  %s
color15 #ffffff
`,
		t.Name, t.JapaneseName, op,
		t.TextMain, t.BgBase, t.Primary, t.BgBase, t.Secondary,
		tc[0], tc[1], tc[1], tc[2], tc[2], tc[3], tc[3], tc[4], tc[4], tc[5], tc[5], tc[6], tc[6], tc[7],
	)
}

func genFastfetchJSON(t config.Theme) string {
	return fmt.Sprintf(`{
  "$schema": "https://github.com/fastfetch-cli/fastfetch/raw/dev/doc/json_schema.json",
  "logo": {
    "type": "kitty",
    "source": "~/.config/kitsune-de/wallpapers/kitsune-avatar.jpg",
    "width": 26,
    "padding": { "top": 1, "left": 2 }
  },
  "display": {
    "separator": " ❯ ",
    "color": { "keys": "#%s", "title": "#%s" }
  },
  "modules": [
    "title",
    "separator",
    { "type": "os", "key": "󰣇 Distro" },
    { "type": "kernel", "key": " Kernel" },
    { "type": "wm", "key": "🦊 Wayland DE", "format": "Kitsune Go Wayland (Tabbed • No Dock)" },
    { "type": "custom", "key": "🌸 Tema Anime", "format": "%s (%s)" },
    { "type": "shell", "key": " Shell" },
    { "type": "terminal", "key": " Terminal" },
    { "type": "cpu", "key": "󰻠 CPU" },
    { "type": "memory", "key": "󰍛 Memoria" },
    "break",
    "colors"
  ]
}
`, hexNoHash(t.Primary), hexNoHash(t.Secondary), t.Name, t.JapaneseName)
}

func genGTKSettingsINI() string {
	return `[Settings]
gtk-application-prefer-dark-theme=1
gtk-theme-name=Adwaita-dark
gtk-icon-theme-name=Papirus-Dark
gtk-font-name=Inter 10.5
gtk-cursor-theme-name=Bibata-Modern-Ice
gtk-cursor-theme-size=24
`
}

func genGTKCSS(t config.Theme) string {
	return fmt.Sprintf(`/* 🦊 Kitsune Go Wayland — GTK4 Dark Anime Palette (%s) */
@define-color accent_color %s;
@define-color accent_bg_color %s;
@define-color accent_fg_color #090812;
@define-color window_bg_color %s;
@define-color window_fg_color %s;
@define-color view_bg_color %s;
@define-color view_fg_color %s;
@define-color headerbar_bg_color %s;
@define-color headerbar_fg_color %s;
`, t.Name, t.Primary, t.Primary, t.BgBase, t.TextMain, t.BgSurface, t.TextMain, t.BgSurface, t.TextMain)
}
