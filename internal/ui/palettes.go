package ui

import (
	"sort"
	"strings"
	"sync"
)

// Palette es un conjunto de colores con roles semánticos.
//
// Cada paleta puede además sugerir un degradado (para barras y acentos), una
// escena de fondo animada y una categoría con la que se agrupa en la galería.
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

	// Gradient son los dos extremos del degradado de acento.
	Gradient [2]Color `json:"gradient"`
	// Category agrupa la paleta: anime, oscuro, claro, retro, neón.
	Category string `json:"category"`
	// Light indica que la paleta es de fondo claro.
	Light bool `json:"light"`
	// Scene sugiere una escena de fondo animada (ver internal/wallpaper).
	Scene string `json:"scene"`
	// Adaptive marca las paletas generadas a partir de un fondo de pantalla.
	Adaptive bool `json:"adaptive"`
}

// GradientOr devuelve el degradado, usando acento y acento2 si no se definió.
func (p Palette) GradientOr() [2]Color {
	if p.Gradient[0] != "" && p.Gradient[1] != "" {
		return p.Gradient
	}
	return [2]Color{p.Accent, p.Accent2}
}

// PaletteMeta describe una paleta en la galería.
type PaletteMeta struct {
	Key      string
	Name     string
	Category string
	Light    bool
	Scene    string
	Adaptive bool
}

var (
	paletteMu    sync.RWMutex
	extraPalette = map[string]Palette{}
)

// RegisterPalette añade o reemplaza una paleta en tiempo de ejecución.
// Se usa para las paletas adaptativas generadas desde un fondo de pantalla.
func RegisterPalette(key string, p Palette) {
	paletteMu.Lock()
	defer paletteMu.Unlock()
	if BuiltinPalettes[key].Name != "" && p.Name == "" {
		return
	}
	extraPalette[strings.ToLower(key)] = p
}

// UnregisterPalette elimina una paleta dinámica.
func UnregisterPalette(key string) {
	paletteMu.Lock()
	defer paletteMu.Unlock()
	delete(extraPalette, strings.ToLower(key))
}

// BuiltinPalettes contiene las paletas incluidas.
//
// Las primeras se agrupan como "anime": están inspiradas en la estética de la
// animación japonesa (sakura, neón de Tokio, atardecer, noche estrellada...).
var BuiltinPalettes = map[string]Palette{
	// ─── Anime ────────────────────────────────────────────────────────────
	"sakura": {
		Name: "Sakura", Category: "anime", Scene: "petals",
		Bg: "#1a1418", BgAlt: "#140f13", BgSel: "#33222c",
		Fg: "#f8e7ee", FgDim: "#e2c3d2", Muted: "#a98c9b",
		Accent: "#ff9ec4", Accent2: "#c58bff",
		Ok: "#a8e6a1", Warn: "#ffd6a5", Err: "#ff7a90", Border: "#4a3341",
		Gradient: [2]Color{"#ff9ec4", "#c58bff"},
	},
	"miku": {
		Name: "Miku", Category: "anime", Scene: "wave",
		Bg: "#0b1618", BgAlt: "#081113", BgSel: "#163236",
		Fg: "#dff7f5", FgDim: "#a9dbd6", Muted: "#6a9c98",
		Accent: "#39c5bb", Accent2: "#ff6ba9",
		Ok: "#7ee787", Warn: "#f7d774", Err: "#ff5d7a", Border: "#1e4448",
		Gradient: [2]Color{"#39c5bb", "#ff6ba9"},
	},
	"asuka": {
		Name: "Asuka", Category: "anime", Scene: "embers",
		Bg: "#160e11", BgAlt: "#100a0c", BgSel: "#33161c",
		Fg: "#fbe6e6", FgDim: "#dfb9b9", Muted: "#a47171",
		Accent: "#ff4655", Accent2: "#ffb03a",
		Ok: "#8fdc8a", Warn: "#ffcf6b", Err: "#ff2e63", Border: "#46232a",
		Gradient: [2]Color{"#ff4655", "#ffb03a"},
	},
	"rei": {
		Name: "Rei", Category: "anime", Light: true, Scene: "snow",
		Bg: "#eef1f6", BgAlt: "#e2e8f1", BgSel: "#cfd9ea",
		Fg: "#26303f", FgDim: "#4a5768", Muted: "#7d8ca1",
		Accent: "#5f7fb5", Accent2: "#c96f8a",
		Ok: "#3f9c6a", Warn: "#b07d1a", Err: "#c4435c", Border: "#b9c4d4",
		Gradient: [2]Color{"#5f7fb5", "#c96f8a"},
	},
	"zerotwo": {
		Name: "Zero Two", Category: "anime", Scene: "petals",
		Bg: "#1b1020", BgAlt: "#150c19", BgSel: "#361e3f",
		Fg: "#fbe6f5", FgDim: "#e0bfd8", Muted: "#a4809c",
		Accent: "#ff5fa2", Accent2: "#7ee8e0",
		Ok: "#8ee6a1", Warn: "#ffd28a", Err: "#ff4d6d", Border: "#4a2a52",
		Gradient: [2]Color{"#ff5fa2", "#7ee8e0"},
	},
	"neontokyo": {
		Name: "Neon Tokyo", Category: "anime", Scene: "city",
		Bg: "#0b0f19", BgAlt: "#080b12", BgSel: "#1c2233",
		Fg: "#e6f0ff", FgDim: "#a9bad6", Muted: "#65728f",
		Accent: "#ff2e97", Accent2: "#00e5ff",
		Ok: "#37e6a0", Warn: "#ffd166", Err: "#ff4d6d", Border: "#26304a",
		Gradient: [2]Color{"#ff2e97", "#00e5ff"},
	},
	"cyberpunk": {
		Name: "Cyberpunk", Category: "anime", Scene: "city",
		Bg: "#121014", BgAlt: "#0c0b0e", BgSel: "#26222b",
		Fg: "#f0ede6", FgDim: "#c9c4b6", Muted: "#8b8578",
		Accent: "#fcee0a", Accent2: "#00f0ff",
		Ok: "#00ff9c", Warn: "#fcee0a", Err: "#ff003c", Border: "#3a3540",
		Gradient: [2]Color{"#fcee0a", "#00f0ff"},
	},
	"vaporwave": {
		Name: "Vaporwave", Category: "anime", Scene: "grid",
		Bg: "#1b1035", BgAlt: "#150c2b", BgSel: "#31215a",
		Fg: "#f4e9ff", FgDim: "#cbb8e8", Muted: "#8d7ab5",
		Accent: "#ff71ce", Accent2: "#01cdfe",
		Ok: "#05ffa1", Warn: "#fffb96", Err: "#ff5f7e", Border: "#3d2a68",
		Gradient: [2]Color{"#ff71ce", "#01cdfe"},
	},
	"yuki": {
		Name: "Yuki", Category: "anime", Light: true, Scene: "snow",
		Bg: "#f5f8fc", BgAlt: "#e9eff8", BgSel: "#d5e2f2",
		Fg: "#22303f", FgDim: "#455a70", Muted: "#7b90a8",
		Accent: "#5b8def", Accent2: "#8ad7f0",
		Ok: "#2f9e6b", Warn: "#b5851d", Err: "#c94b5e", Border: "#c2d2e4",
		Gradient: [2]Color{"#5b8def", "#8ad7f0"},
	},
	"ghibli": {
		Name: "Ghibli", Category: "anime", Scene: "meadow",
		Bg: "#18211a", BgAlt: "#121a13", BgSel: "#293a2b",
		Fg: "#eef3e2", FgDim: "#cfd9bd", Muted: "#8fa085",
		Accent: "#a3be8c", Accent2: "#e8c07d",
		Ok: "#9ccf7a", Warn: "#e8c07d", Err: "#d97b6c", Border: "#35473a",
		Gradient: [2]Color{"#a3be8c", "#e8c07d"},
	},
	"neko": {
		Name: "Neko", Category: "anime", Scene: "stars",
		Bg: "#16131f", BgAlt: "#110f18", BgSel: "#2a2438",
		Fg: "#f2ecff", FgDim: "#d3c7ee", Muted: "#9186ad",
		Accent: "#c9a0ff", Accent2: "#ffd4a8",
		Ok: "#9fe8b8", Warn: "#ffd79a", Err: "#ff8fa3", Border: "#3a3350",
		Gradient: [2]Color{"#c9a0ff", "#ffd4a8"},
	},
	"matcha": {
		Name: "Matcha", Category: "anime", Scene: "meadow",
		Bg: "#131a15", BgAlt: "#0e1410", BgSel: "#233026",
		Fg: "#eaf3e6", FgDim: "#c4d8bf", Muted: "#829981",
		Accent: "#9ccf7a", Accent2: "#e8d9a0",
		Ok: "#9ee07a", Warn: "#e8d9a0", Err: "#e07a7a", Border: "#2f4234",
		Gradient: [2]Color{"#9ccf7a", "#e8d9a0"},
	},
	"ramen": {
		Name: "Ramen", Category: "anime", Scene: "embers",
		Bg: "#1d1611", BgAlt: "#16100c", BgSel: "#33261c",
		Fg: "#fdf0e2", FgDim: "#e6cdb4", Muted: "#b39377",
		Accent: "#ff8a3d", Accent2: "#ffd166",
		Ok: "#a8d97a", Warn: "#ffd166", Err: "#ff6b5e", Border: "#4a3527",
		Gradient: [2]Color{"#ff8a3d", "#ffd166"},
	},
	"kawaii": {
		Name: "Kawaii", Category: "anime", Light: true, Scene: "petals",
		Bg: "#fff5f8", BgAlt: "#ffeaf2", BgSel: "#ffd6e6",
		Fg: "#4a2b3a", FgDim: "#6f4a5c", Muted: "#a98295",
		Accent: "#ff6b9d", Accent2: "#7ad7f0",
		Ok: "#4fb783", Warn: "#c98a1a", Err: "#e04a6e", Border: "#f3c3d6",
		Gradient: [2]Color{"#ff6b9d", "#7ad7f0"},
	},
	"hoshizora": {
		Name: "Hoshizora", Category: "anime", Scene: "stars",
		Bg: "#0b1024", BgAlt: "#080c1c", BgSel: "#1b2340",
		Fg: "#e8eeff", FgDim: "#bfc9ee", Muted: "#77809f",
		Accent: "#8ab4ff", Accent2: "#ffd166",
		Ok: "#7ee2b8", Warn: "#ffd166", Err: "#ff7b93", Border: "#232c4a",
		Gradient: [2]Color{"#8ab4ff", "#ffd166"},
	},
	"oni": {
		Name: "Oni", Category: "anime", Scene: "embers",
		Bg: "#170c0e", BgAlt: "#110809", BgSel: "#321318",
		Fg: "#fbe5e5", FgDim: "#dda9a9", Muted: "#a26464",
		Accent: "#ff3b3b", Accent2: "#ffb703",
		Ok: "#8fd98f", Warn: "#ffb703", Err: "#ff1744", Border: "#43191d",
		Gradient: [2]Color{"#ff3b3b", "#ffb703"},
	},
	"shonen": {
		Name: "Shonen", Category: "anime", Scene: "wave",
		Bg: "#0f1820", BgAlt: "#0b1218", BgSel: "#1c2c3a",
		Fg: "#eaf4ff", FgDim: "#b9cde0", Muted: "#7691a8",
		Accent: "#f2a541", Accent2: "#4fc3f7",
		Ok: "#66d69a", Warn: "#f2a541", Err: "#ff6b6b", Border: "#24384a",
		Gradient: [2]Color{"#f2a541", "#4fc3f7"},
	},
	"mecha": {
		Name: "Mecha", Category: "anime", Scene: "grid",
		Bg: "#11151a", BgAlt: "#0c0f13", BgSel: "#1f282f",
		Fg: "#e8f1f5", FgDim: "#b8c8d1", Muted: "#75868f",
		Accent: "#5ad1e6", Accent2: "#ff6b35",
		Ok: "#5ae6a5", Warn: "#ffd166", Err: "#ff5c5c", Border: "#28353d",
		Gradient: [2]Color{"#5ad1e6", "#ff6b35"},
	},
	"mahou": {
		Name: "Mahou", Category: "anime", Scene: "aurora",
		Bg: "#191226", BgAlt: "#130d1e", BgSel: "#2d2044",
		Fg: "#f5edff", FgDim: "#d6c6ef", Muted: "#9784b8",
		Accent: "#b68cff", Accent2: "#7ef2d0",
		Ok: "#7ef2b0", Warn: "#ffd98a", Err: "#ff86a8", Border: "#3a2b56",
		Gradient: [2]Color{"#b68cff", "#7ef2d0"},
	},
	"pastel": {
		Name: "Pastel Dream", Category: "anime", Light: true, Scene: "snow",
		Bg: "#f7f3ff", BgAlt: "#efe8fb", BgSel: "#e0d4f7",
		Fg: "#3b3350", FgDim: "#5f5478", Muted: "#9187a8",
		Accent: "#b39ddb", Accent2: "#ffb3c8",
		Ok: "#66b98c", Warn: "#c39a3a", Err: "#d96a86", Border: "#d5c9ee",
		Gradient: [2]Color{"#b39ddb", "#ffb3c8"},
	},
	"aura": {
		Name: "Aura", Category: "anime", Scene: "aurora",
		Bg: "#0d1320", BgAlt: "#090e18", BgSel: "#182636",
		Fg: "#e6fbf6", FgDim: "#b4ddd6", Muted: "#729c99",
		Accent: "#7cf2c9", Accent2: "#8a9cff",
		Ok: "#7cf2c9", Warn: "#ffd88a", Err: "#ff7e94", Border: "#1d3040",
		Gradient: [2]Color{"#7cf2c9", "#8a9cff"},
	},

	// ─── Oscuros ──────────────────────────────────────────────────────────
	"catppuccin": {
		Name: "Catppuccin Mocha", Category: "oscuro", Scene: "aurora",
		Bg: "#1e1e2e", BgAlt: "#181825", BgSel: "#313244",
		Fg: "#cdd6f4", FgDim: "#a6adc8", Muted: "#6c7086",
		Accent: "#89b4fa", Accent2: "#cba6f7",
		Ok: "#a6e3a1", Warn: "#f9e2af", Err: "#f38ba8", Border: "#45475a",
		Gradient: [2]Color{"#89b4fa", "#cba6f7"},
	},
	"tokyonight": {
		Name: "Tokyo Night", Category: "oscuro", Scene: "city",
		Bg: "#1a1b26", BgAlt: "#16161e", BgSel: "#292e42",
		Fg: "#c0caf5", FgDim: "#a9b1d6", Muted: "#565f89",
		Accent: "#7aa2f7", Accent2: "#bb9af7",
		Ok: "#9ece6a", Warn: "#e0af68", Err: "#f7768e", Border: "#3b4261",
		Gradient: [2]Color{"#7aa2f7", "#bb9af7"},
	},
	"nord": {
		Name: "Nord", Category: "oscuro", Scene: "snow",
		Bg: "#2e3440", BgAlt: "#292e39", BgSel: "#3b4252",
		Fg: "#eceff4", FgDim: "#d8dee9", Muted: "#7b88a1",
		Accent: "#88c0d0", Accent2: "#b48ead",
		Ok: "#a3be8c", Warn: "#ebcb8b", Err: "#bf616a", Border: "#434c5e",
		Gradient: [2]Color{"#88c0d0", "#b48ead"},
	},
	"gruvbox": {
		Name: "Gruvbox Dark", Category: "retro", Scene: "embers",
		Bg: "#282828", BgAlt: "#1d2021", BgSel: "#3c3836",
		Fg: "#ebdbb2", FgDim: "#d5c4a1", Muted: "#928374",
		Accent: "#83a598", Accent2: "#d3869b",
		Ok: "#b8bb26", Warn: "#fabd2f", Err: "#fb4934", Border: "#504945",
		Gradient: [2]Color{"#83a598", "#d3869b"},
	},
	"dracula": {
		Name: "Dracula", Category: "oscuro", Scene: "aurora",
		Bg: "#282a36", BgAlt: "#21222c", BgSel: "#44475a",
		Fg: "#f8f8f2", FgDim: "#c8c8c2", Muted: "#6272a4",
		Accent: "#bd93f9", Accent2: "#ff79c6",
		Ok: "#50fa7b", Warn: "#f1fa8c", Err: "#ff5555", Border: "#44475a",
		Gradient: [2]Color{"#bd93f9", "#ff79c6"},
	},
	"kanagawa": {
		Name: "Kanagawa", Category: "retro", Scene: "wave",
		Bg: "#1f1f28", BgAlt: "#16161d", BgSel: "#2a2a37",
		Fg: "#dcd7ba", FgDim: "#c8c093", Muted: "#727169",
		Accent: "#7e9cd8", Accent2: "#957fb8",
		Ok: "#98bb6c", Warn: "#e6c384", Err: "#e46876", Border: "#363646",
		Gradient: [2]Color{"#7e9cd8", "#957fb8"},
	},
	"rosepine": {
		Name: "Rosé Pine", Category: "oscuro", Scene: "petals",
		Bg: "#191724", BgAlt: "#1f1d2e", BgSel: "#26233a",
		Fg: "#e0def4", FgDim: "#908caa", Muted: "#6e6a86",
		Accent: "#c4a7e7", Accent2: "#ebbcba",
		Ok: "#9ccfd8", Warn: "#f6c177", Err: "#eb6f92", Border: "#403d52",
		Gradient: [2]Color{"#c4a7e7", "#ebbcba"},
	},
	"everforest": {
		Name: "Everforest", Category: "oscuro", Scene: "meadow",
		Bg: "#2d353b", BgAlt: "#232a2e", BgSel: "#3d484d",
		Fg: "#d3c6aa", FgDim: "#b8ae91", Muted: "#859289",
		Accent: "#a7c080", Accent2: "#d699b6",
		Ok: "#a7c080", Warn: "#dbbc7f", Err: "#e67e80", Border: "#475258",
		Gradient: [2]Color{"#a7c080", "#d699b6"},
	},
	// ─── Claros ───────────────────────────────────────────────────────────
	"paper": {
		Name: "Paper", Category: "claro", Light: true, Scene: "snow",
		Bg: "#f6f3ee", BgAlt: "#ece8e1", BgSel: "#ded8cf",
		Fg: "#2b2a27", FgDim: "#514f49", Muted: "#8a867d",
		Accent: "#b0643c", Accent2: "#5f8a72",
		Ok: "#3f8a5f", Warn: "#a8791f", Err: "#b34a4a", Border: "#d3ccc1",
		Gradient: [2]Color{"#b0643c", "#5f8a72"},
	},
	"daylight": {
		Name: "Daylight", Category: "claro", Light: true, Scene: "meadow",
		Bg: "#f2f6fb", BgAlt: "#e7eef7", BgSel: "#d3e0f0",
		Fg: "#1d2836", FgDim: "#415163", Muted: "#7c8b9c",
		Accent: "#2f7ec4", Accent2: "#d97706",
		Ok: "#2e8b57", Warn: "#b7791f", Err: "#c0392b", Border: "#c3d2e2",
		Gradient: [2]Color{"#2f7ec4", "#d97706"},
	},
}

// PaletteByName devuelve una paleta por nombre; si no existe, usa catppuccin.
func PaletteByName(name string) Palette {
	key := strings.ToLower(strings.TrimSpace(name))
	paletteMu.RLock()
	extra, ok := extraPalette[key]
	paletteMu.RUnlock()
	if ok {
		return extra
	}
	if p, ok := BuiltinPalettes[key]; ok {
		return p
	}
	return BuiltinPalettes["catppuccin"]
}

// IsPalette indica si existe una paleta con ese nombre.
func IsPalette(name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	if _, ok := BuiltinPalettes[key]; ok {
		return true
	}
	paletteMu.RLock()
	defer paletteMu.RUnlock()
	_, ok := extraPalette[key]
	return ok
}

// PaletteNames devuelve los nombres de todas las paletas disponibles,
// ordenados por categoría y luego por nombre.
func PaletteNames() []string {
	metas := PaletteList("")
	names := make([]string, 0, len(metas))
	for _, m := range metas {
		names = append(names, m.Key)
	}
	return names
}

// PaletteCategories devuelve las categorías presentes, en orden fijo.
func PaletteCategories() []string {
	order := []string{"anime", "oscuro", "retro", "claro", "adaptativo"}
	present := map[string]bool{}
	for _, p := range BuiltinPalettes {
		present[p.Category] = true
	}
	paletteMu.RLock()
	for _, p := range extraPalette {
		present[p.Category] = true
	}
	paletteMu.RUnlock()
	var out []string
	for _, c := range order {
		if present[c] {
			out = append(out, c)
		}
	}
	return out
}

// PaletteList devuelve las paletas, opcionalmente filtradas por categoría.
func PaletteList(category string) []PaletteMeta {
	var out []PaletteMeta
	add := func(key string, p Palette) {
		if category != "" && p.Category != category {
			return
		}
		cat := p.Category
		if cat == "" {
			cat = "oscuro"
		}
		out = append(out, PaletteMeta{Key: key, Name: p.Name, Category: cat, Light: p.Light, Scene: p.Scene, Adaptive: p.Adaptive})
	}
	for key, p := range BuiltinPalettes {
		add(key, p)
	}
	paletteMu.RLock()
	for key, p := range extraPalette {
		add(key, p)
	}
	paletteMu.RUnlock()

	catRank := map[string]int{"anime": 0, "oscuro": 1, "retro": 2, "claro": 3, "adaptativo": 4}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := catRank[out[i].Category], catRank[out[j].Category]
		if ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}
