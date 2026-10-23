package ui

import "strings"

// Palette es un conjunto de colores con roles semánticos.
type Palette struct {
	Name    string `json:"name"`
	Bg      Color  `json:"bg"`
	BgAlt   Color  `json:"bg_alt"`
	BgSel   Color  `json:"bg_sel"`
	Fg      Color  `json:"fg"`
	FgDim   Color  `json:"fg_dim"`
	Muted   Color  `json:"muted"`
	Accent  Color  `json:"accent"`
	Accent2 Color  `json:"accent2"`
	Ok      Color  `json:"ok"`
	Warn    Color  `json:"warn"`
	Err     Color  `json:"err"`
	Border  Color  `json:"border"`
}

// BuiltinPalettes contiene las paletas incluidas.
var BuiltinPalettes = map[string]Palette{
	"catppuccin": {
		Name: "Catppuccin Mocha",
		Bg:   "#1e1e2e", BgAlt: "#181825", BgSel: "#313244",
		Fg: "#cdd6f4", FgDim: "#a6adc8", Muted: "#6c7086",
		Accent: "#89b4fa", Accent2: "#cba6f7",
		Ok: "#a6e3a1", Warn: "#f9e2af", Err: "#f38ba8", Border: "#45475a",
	},
	"tokyonight": {
		Name: "Tokyo Night",
		Bg:   "#1a1b26", BgAlt: "#16161e", BgSel: "#292e42",
		Fg: "#c0caf5", FgDim: "#a9b1d6", Muted: "#565f89",
		Accent: "#7aa2f7", Accent2: "#bb9af7",
		Ok: "#9ece6a", Warn: "#e0af68", Err: "#f7768e", Border: "#3b4261",
	},
	"nord": {
		Name: "Nord",
		Bg:   "#2e3440", BgAlt: "#292e39", BgSel: "#3b4252",
		Fg: "#eceff4", FgDim: "#d8dee9", Muted: "#7b88a1",
		Accent: "#88c0d0", Accent2: "#b48ead",
		Ok: "#a3be8c", Warn: "#ebcb8b", Err: "#bf616a", Border: "#434c5e",
	},
	"gruvbox": {
		Name: "Gruvbox Dark",
		Bg:   "#282828", BgAlt: "#1d2021", BgSel: "#3c3836",
		Fg: "#ebdbb2", FgDim: "#d5c4a1", Muted: "#928374",
		Accent: "#83a598", Accent2: "#d3869b",
		Ok: "#b8bb26", Warn: "#fabd2f", Err: "#fb4934", Border: "#504945",
	},
	"dracula": {
		Name: "Dracula",
		Bg:   "#282a36", BgAlt: "#21222c", BgSel: "#44475a",
		Fg: "#f8f8f2", FgDim: "#c8c8c2", Muted: "#6272a4",
		Accent: "#bd93f9", Accent2: "#ff79c6",
		Ok: "#50fa7b", Warn: "#f1fa8c", Err: "#ff5555", Border: "#44475a",
	},
	"kanagawa": {
		Name: "Kanagawa",
		Bg:   "#1f1f28", BgAlt: "#16161d", BgSel: "#2a2a37",
		Fg: "#dcd7ba", FgDim: "#c8c093", Muted: "#727169",
		Accent: "#7e9cd8", Accent2: "#957fb8",
		Ok: "#98bb6c", Warn: "#e6c384", Err: "#e46876", Border: "#363646",
	},
	"rosepine": {
		Name: "Rosé Pine",
		Bg:   "#191724", BgAlt: "#1f1d2e", BgSel: "#26233a",
		Fg: "#e0def4", FgDim: "#908caa", Muted: "#6e6a86",
		Accent: "#c4a7e7", Accent2: "#ebbcba",
		Ok: "#9ccfd8", Warn: "#f6c177", Err: "#eb6f92", Border: "#403d52",
	},
	"everforest": {
		Name: "Everforest",
		Bg:   "#2d353b", BgAlt: "#232a2e", BgSel: "#3d484d",
		Fg: "#d3c6aa", FgDim: "#b8ae91", Muted: "#859289",
		Accent: "#a7c080", Accent2: "#d699b6",
		Ok: "#a7c080", Warn: "#dbbc7f", Err: "#e67e80", Border: "#475258",
	},
}

// PaletteByName devuelve una paleta por nombre; si no existe, usa catppuccin.
func PaletteByName(name string) Palette {
	if p, ok := BuiltinPalettes[strings.ToLower(strings.TrimSpace(name))]; ok {
		return p
	}
	return BuiltinPalettes["catppuccin"]
}

// PaletteNames devuelve los nombres de las paletas disponibles.
func PaletteNames() []string {
	return []string{"catppuccin", "dracula", "everforest", "gruvbox", "kanagawa", "nord", "rosepine", "tokyonight"}
}

// Icons agrupa los glifos usados por la interfaz. Todos tienen una alternativa
// ASCII para terminales sin Nerd Font.
type Icons struct {
	NerdFont bool
	Workspace, Window, Volume, VolumeLow, Muted, Wifi, Battery, Charging, Plug,
	CPU, RAM, Disk, Clock, Calendar, Lock, Power, Reboot, Suspend, Logout,
	Search, Terminal, Files, Browser, Code, Music, Play, Pause, Next, Prev,
	Gear, Check, Cross, Warn, Info, Chevron, Dot, Star, Eye, Keyboard, Network,
	Theme, Layout, Update, Rocket, Doc, Game, Chat, Mail, Camera, Calc, Scroll string
}

// IconsFor devuelve el juego de iconos adecuado.
func IconsFor(nerd bool) Icons {
	if nerd {
		return Icons{
			NerdFont:  true,
			Workspace: "\uf269", Window: "\uf2d0",
			Volume: "\uf028", VolumeLow: "\uf027", Muted: "\uf026",
			Wifi: "\uf1eb", Battery: "\uf240", Charging: "\uf0e7", Plug: "\uf1e6",
			CPU: "\uf2db", RAM: "\uf85a", Disk: "\uf0a0", Clock: "\uf017",
			Calendar: "\uf073", Lock: "\uf023", Power: "\uf011", Reboot: "\uf021",
			Suspend: "\uf186", Logout: "\uf08b", Search: "\uf002",
			Terminal: "\uf120", Files: "\uf07b", Browser: "\uf0ac", Code: "\uf121",
			Music: "\uf001", Play: "\uf04b", Pause: "\uf04c", Next: "\uf051",
			Prev: "\uf048", Gear: "\uf013", Check: "\uf00c", Cross: "\uf00d",
			Warn: "\uf071", Info: "\uf05a", Chevron: "\uf054", Dot: "\uf111",
			Star: "\uf005", Eye: "\uf06e", Keyboard: "\uf11c", Network: "\uf6ff",
			Theme: "\uf53f", Layout: "\uf0db", Update: "\uf019", Rocket: "\uf135",
			Doc: "\uf15c", Game: "\uf11b", Chat: "\uf075", Mail: "\uf0e0",
			Camera: "\uf030", Calc: "\uf1ec", Scroll: "\uf02d",
		}
	}
	return Icons{
		NerdFont:  false,
		Workspace: "◆", Window: "▭", Volume: "♪", VolumeLow: "♪", Muted: "✕",
		Wifi: "≋", Battery: "▮", Charging: "+", Plug: "⌁",
		CPU: "C", RAM: "M", Disk: "D", Clock: "◷",
		Calendar: "▦", Lock: "⊘", Power: "◎", Reboot: "↻",
		Suspend: "☾", Logout: "⇥", Search: "⌕",
		Terminal: ">_", Files: "▤", Browser: "◍", Code: "</>",
		Music: "♪", Play: "▶", Pause: "❙❙", Next: "▶▶",
		Prev: "◀◀", Gear: "⚙", Check: "✔", Cross: "✘",
		Warn: "!", Info: "i", Chevron: "›", Dot: "•",
		Star: "★", Eye: "◉", Keyboard: "⌨", Network: "≋",
		Theme: "◐", Layout: "▥", Update: "↑", Rocket: "▲",
		Doc: "≡", Game: "✚", Chat: "✉", Mail: "@",
		Camera: "◉", Calc: "=", Scroll: "≡",
	}
}

// Theme combina paleta, iconos y estilo de borde.
type Theme struct {
	Palette Palette
	Icons   Icons
	Border  BorderStyle
}

// NewTheme construye un tema.
func NewTheme(paletteName string, nerdFont bool) *Theme {
	return &Theme{Palette: PaletteByName(paletteName), Icons: IconsFor(nerdFont), Border: BorderRounded}
}

// Text devuelve un estilo con el color principal.
func (t *Theme) Text() Style { return Style{Fg: t.Palette.Fg, Bg: t.Palette.Bg} }

// Alt devuelve un estilo sobre el fondo alternativo.
func (t *Theme) Alt() Style { return Style{Fg: t.Palette.Fg, Bg: t.Palette.BgAlt} }

// Muted devuelve un estilo atenuado.
func (t *Theme) Muted() Style { return Style{Fg: t.Palette.Muted, Bg: t.Palette.Bg} }

// Faint devuelve el estilo secundario.
func (t *Theme) Faint() Style { return Style{Fg: t.Palette.FgDim, Bg: t.Palette.Bg} }

// Accent devuelve el estilo de acento.
func (t *Theme) Accent() Style { return Style{Fg: t.Palette.Accent, Bg: t.Palette.Bg}.B() }

// Selected devuelve el estilo de la fila seleccionada.
func (t *Theme) Selected() Style {
	return Style{Fg: t.Palette.Bg, Bg: t.Palette.Accent}.B()
}

// StyleOn devuelve un estilo con el fondo del panel base.
func (t *Theme) StyleOn(fg, bg Color) Style { return Style{Fg: fg, Bg: bg} }
