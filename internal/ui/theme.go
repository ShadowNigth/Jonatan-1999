package ui

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
