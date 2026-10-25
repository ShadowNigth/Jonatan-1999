package gallery

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/ui"
	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// WallpaperResult es el resultado del selector de fondos.
type WallpaperResult struct {
	Spec    wallpaper.Spec
	Applied bool
}

type previewLoadRequest struct {
	id   uint64
	spec wallpaper.Spec
}

type previewLoadResult struct {
	id     uint64
	engine *wallpaper.Engine
	err    error
}

// WallpaperPicker abre el selector de fondos animados: escenas procedurales y
// archivos (imágenes, GIF y vídeos) de las carpetas configuradas.
//
// Devuelve la especificación elegida; el llamador decide si la aplica.
func WallpaperPicker(pal ui.Palette, nerdFont bool, dirs []string, current string) (WallpaperResult, error) {
	theme := ui.NewTheme("", nerdFont)
	theme.Palette = pal
	sc, err := ui.OpenScreen()
	if err != nil {
		return WallpaperResult{}, err
	}
	defer sc.Close()

	pane := &listPane{}
	var (
		filter           string
		live             = true
		engine           *wallpaper.Engine
		engineErr        error
		result           WallpaperResult
		previewKey       string
		previewRequestID uint64
		previewLoading   bool
		previewLines     []string
		previewWidth     int
		previewHeight    int
		previewAt        time.Time
		tabs             = []string{"todas", "escenas", "archivos"}
		tabIdx           = 0
	)

	files := scanWallpapers(dirs)
	outputW, outputH := wallpaper.OutputSize()
	if outputW <= 0 || outputH <= 0 {
		outputW, outputH = 1920, 1080
	}
	// Una fila de terminal mide aproximadamente dos píxeles de alto porque la
	// vista usa medios bloques; el preview sigue la proporción del monitor.
	cellAspect := 2 * float64(outputW) / float64(outputH)
	if cellAspect <= 0 {
		cellAspect = 32.0 / 9.0
	}

	// Cargar imágenes, GIF y metadatos de vídeo fuera del bucle de la UI.
	// Solo se encola la selección más reciente para que navegar rápido no inicie
	// decodificaciones costosas que ya quedaron obsoletas.
	previewRequests := make(chan previewLoadRequest, 1)
	previewResults := make(chan previewLoadResult)
	stopPreview := make(chan struct{})
	defer close(stopPreview)
	go func() {
		for {
			select {
			case <-stopPreview:
				return
			case req := <-previewRequests:
				eng, loadErr := wallpaper.NewEngine(req.spec)
				loaded := previewLoadResult{id: req.id, engine: eng, err: loadErr}
				select {
				case previewResults <- loaded:
				case <-stopPreview:
					if eng != nil {
						eng.Close()
					}
					return
				}
			}
		}
	}()

	requestPreview := func() {
		entry, ok := pane.current()
		key := ""
		if ok {
			key = entry.Key
		}
		if key == previewKey {
			return
		}
		previewKey = key
		previewRequestID++
		previewLoading = ok
		engineErr = nil
		previewLines = nil
		previewWidth, previewHeight = 0, 0
		previewAt = time.Time{}
		if engine != nil {
			engine.Close()
			engine = nil
		}
		if !ok {
			select {
			case <-previewRequests:
			default:
			}
			return
		}

		spec := entry.Spec
		spec.Pal = pal
		switch spec.Kind {
		case wallpaper.KindScene:
			spec.FPS = 24
		case wallpaper.KindGIF, wallpaper.KindVideo:
			spec.FPS = 15 // el motor usa la temporización real del archivo
		default:
			spec.FPS = 4
		}
		req := previewLoadRequest{id: previewRequestID, spec: spec}
		select {
		case previewRequests <- req:
		default:
			select {
			case <-previewRequests:
			default:
			}
			select {
			case previewRequests <- req:
			default:
			}
		}
	}

	acceptPreviewResults := func() {
		for {
			select {
			case loaded := <-previewResults:
				if loaded.id != previewRequestID {
					if loaded.engine != nil {
						loaded.engine.Close()
					}
					continue
				}
				engine, engineErr = loaded.engine, loaded.err
				previewLoading = false
				previewLines = nil
				previewWidth, previewHeight = 0, 0
				previewAt = time.Time{}
				if engine != nil && !live {
					engine.Pause()
				}
			default:
				return
			}
		}
	}

	rebuild := func() {
		pane.entries = pane.entries[:0]
		addScene := func(s wallpaper.SceneInfo) {
			pane.entries = append(pane.entries, Entry{
				Key: "scene:" + s.Key, Title: s.Name, Sub: s.Key, Tag: "escenas",
				Icon: theme.Icons.Star, Pal: pal,
				Spec: wallpaper.Spec{Kind: wallpaper.KindScene, Scene: s.Key, FPS: 24, Pal: pal},
			})
		}
		addFile := func(f fileEntry) {
			kind := wallpaper.KindGIF
			icon := theme.Icons.Camera
			switch f.kind {
			case "video":
				kind, icon = wallpaper.KindVideo, theme.Icons.Play
			case "image":
				kind, icon = wallpaper.KindImage, theme.Icons.Files
			}
			pane.entries = append(pane.entries, Entry{
				Key: string(kind) + ":" + f.path, Title: filepath.Base(f.path),
				Sub: f.kind, Tag: "archivos", Icon: icon, Pal: pal,
				Spec: wallpaper.Spec{Kind: kind, Path: f.path, FPS: 24, Pal: pal},
			})
		}

		if tabIdx == 0 || tabIdx == 1 {
			for _, s := range wallpaper.AvailableScenes() {
				addScene(s)
			}
		}
		if tabIdx == 0 || tabIdx == 2 {
			for _, f := range files {
				addFile(f)
			}
		}
		if filter != "" {
			kept := pane.entries[:0]
			for _, e := range pane.entries {
				if _, ok := ui.Fuzzy(filter, e.Title+" "+e.Key+" "+e.Tag); ok {
					kept = append(kept, e)
				}
			}
			pane.entries = kept
		}
		if pane.cursor >= len(pane.entries) {
			pane.cursor = max(0, len(pane.entries)-1)
		}
		requestPreview()
	}
	rebuild()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	view := func(w, h int) string {
		acceptPreviewResults()
		innerW := w - 4
		head := header(theme, innerW, "fondos animados", tabs, tabIdx)
		search := searchBar(theme, innerW, filter, "filtra fondos… (Enter aplica · p ventana completa)", theme.Icons.Search)

		listW := clampInt(w/3, 26, 40)
		bodyH := h - 8
		if bodyH < 8 {
			bodyH = 8
		}
		leftBlock := pane.render(theme, listW, bodyH, "")

		rightW := innerW - listW - 5
		if rightW < 20 {
			rightW = 20
		}
		var right []string
		wallH := clampInt(bodyH-9, 6, 20)
		wallW := int(math.Round(float64(wallH) * cellAspect))
		if wallW > rightW {
			wallW = rightW
			wallH = max(2, int(math.Round(float64(wallW)/cellAspect)))
		}
		if wallH < 2 {
			wallH = 2
			wallW = min(rightW, max(2, int(math.Round(float64(wallH)*cellAspect))))
		}
		if wallW < 2 {
			wallW = 2
		}
		padLeft := (rightW - wallW) / 2
		padRight := rightW - wallW - padLeft
		bgPad := ui.Style{Bg: pal.Bg}.Render(strings.Repeat(" ", padLeft))
		bgTail := ui.Style{Bg: pal.Bg}.Render(strings.Repeat(" ", padRight))
		var preview []string
		if engine != nil && live {
			// Limita la miniatura animada a 10 Hz para que el render del menú no
			// compita con la carga de archivos; las imágenes fijas se calculan una vez.
			refresh := wallW != previewWidth || wallH != previewHeight
			if engine.Kind() == wallpaper.KindImage {
				refresh = refresh || len(previewLines) == 0
			} else {
				refresh = refresh || time.Since(previewAt) >= 100*time.Millisecond
			}
			if refresh {
				previewLines = wallpaper.PreviewBlock(engine, wallW, wallH)
				previewWidth, previewHeight = wallW, wallH
				previewAt = time.Now()
			}
			preview = previewLines
		}
		if len(preview) > 0 {
			for _, line := range preview {
				right = append(right, bgPad+line+bgTail)
			}
		} else {
			message := "vista previa no disponible"
			switch {
			case engineErr != nil:
				message = "error: " + engineErr.Error()
			case previewLoading:
				message = "cargando vista previa…"
			case !live:
				message = "vista previa pausada"
			case engine != nil && engine.PreviewError() != nil:
				message = "error de vídeo: " + engine.PreviewError().Error()
			case engine != nil && engine.Spec().Kind == wallpaper.KindVideo:
				message = "cargando fotograma de vídeo…"
			}
			message = ui.Truncate(message, rightW-2)
			left := max(0, (rightW-ui.Width(message))/2)
			for i := 0; i < wallH; i++ {
				blank := strings.Repeat(" ", rightW)
				if i == wallH/2 {
					line := ui.Style{Bg: pal.BgAlt}.Render(strings.Repeat(" ", left)) +
						ui.Style{Fg: pal.Muted, Bg: pal.BgAlt}.Render(message)
					line += ui.Style{Bg: pal.BgAlt}.Render(strings.Repeat(" ", max(0, rightW-left-ui.Width(message))))
					right = append(right, line)
				} else {
					right = append(right, ui.Style{Bg: pal.BgAlt}.Render(blank))
				}
			}
		}
		right = append(right, "")
		if e, ok := pane.current(); ok {
			desc := ""
			anim := ""
			cost := ""
			switch e.Spec.Kind {
			case wallpaper.KindScene:
				if info, ok := wallpaper.SceneByKey(e.Spec.Scene); ok {
					desc = info.Description
					anim = info.Animation
					cost = strings.Join(info.Tags, " · ")
				}
			case wallpaper.KindGIF:
				desc = "GIF animado · se reproduce en bucle"
				anim = "bucle"
			case wallpaper.KindVideo:
				desc = "Vídeo · reproducción en bucle con FFmpeg"
				anim = "bucle"
			case wallpaper.KindImage:
				desc = "Imagen fija · paleta adaptativa disponible"
				anim = "estática"
			}
			right = append(right,
				" "+ui.Style{Fg: pal.Fg, Bg: pal.Bg}.B().Render(e.Title),
				metaLine(theme, "descripción", desc, rightW),
				metaLine(theme, "movimiento", anim, rightW),
				metaLine(theme, "detalles", cost, rightW),
			)
			if engine != nil {
				dur := engine.Duration()
				if dur > 0 {
					right = append(right, metaLine(theme, "bucle", dur.Round(time.Millisecond).String(), rightW))
				}
				if sz := engine.SizeBytes(); sz > 0 {
					right = append(right, metaLine(theme, "memoria", humanBytes(sz), rightW))
				}
			}
		} else {
			right = append(right, " "+ui.Style{Fg: pal.Muted, Bg: pal.Bg}.Render("sin fondos que mostrar"),
				" "+ui.Style{Fg: pal.Muted, Bg: pal.Bg}.Render("añade imágenes a: "+strings.Join(dirs, ", ")))
		}
		right = append(right, "")
		if proto := wallpaper.DetectProtocol(); proto != wallpaper.ProtoNone {
			name := "kitty"
			if proto == wallpaper.ProtoSixel {
				name = "sixel"
			}
			right = append(right, metaLine(theme, "terminal", "protocolo de imágenes "+name+" detectado", rightW))
		}

		cols := joinColumns(leftBlock, listW, right, 2, pal.Border, pal.Bg)
		body := make([]string, 0, bodyH)
		for i := 0; i < bodyH; i++ {
			if i < len(cols) {
				body = append(body, cols[i])
			} else {
				body = append(body, "")
			}
		}
		footer := footerBar(theme,
			[2]string{"↑↓", "elegir"}, [2]string{"Tab", "grupo"},
			[2]string{"↵", "aplicar"}, [2]string{"p", "pantalla completa"},
			[2]string{"espacio", "pausar"}, [2]string{"esc", "salir"})

		content := []string{head[0], head[1], search, ""}
		content = append(content, body...)
		return theme.Frame(w, h, "swaydesk · fondos", footer, content)
	}

	playFull := func() {
		if e, ok := pane.current(); ok {
			spec := e.Spec
			spec.Pal = pal
			if spec.FPS == 0 {
				spec.FPS = 24
			}
			eng, err := wallpaper.NewEngine(spec)
			if err == nil {
				sc.Close()
				_ = wallpaper.RunTerminal(eng, wallpaper.Options{FPS: spec.FPS, ShowInfo: true})
				eng.Close()
				os.Exit(0)
			}
		}
	}

	err = sc.Loop(view, func(k ui.Key) bool {
		switch {
		case k.Is("esc"):
			return true
		case k.Is("enter"):
			if e, ok := pane.current(); ok {
				result.Spec = e.Spec
				result.Spec.Pal = pal
				result.Applied = true
				return true
			}
		case k.IsRune('p'):
			playFull()
		case k.Is("tab"):
			tabIdx = (tabIdx + 1) % len(tabs)
			pane.cursor, pane.offset = 0, 0
			rebuild()
		case k.Is("backtab"):
			tabIdx = (tabIdx + len(tabs) - 1) % len(tabs)
			pane.cursor, pane.offset = 0, 0
			rebuild()
		case k.Is("up"):
			pane.move(-1)
			rebuild()
		case k.Is("down"):
			pane.move(1)
			rebuild()
		case k.Is("pgup"):
			pane.move(-8)
			rebuild()
		case k.Is("pgdn"):
			pane.move(8)
			rebuild()
		case k.Is("home"):
			pane.cursor = 0
			rebuild()
		case k.Is("end"):
			pane.cursor = len(pane.entries) - 1
			rebuild()
		case k.IsRune(' '):
			live = !live
			if engine != nil {
				if live {
					engine.Resume()
				} else {
					engine.Pause()
				}
			}
		case k.Is("backspace"):
			if len(filter) > 0 {
				filter = filter[:len(filter)-1]
				rebuild()
			}
		case k.Rune >= 32 && !k.Ctrl && !k.Alt:
			filter += string(k.Rune)
			rebuild()
		}
		return false
	}, ticker.C)
	if engine != nil {
		engine.Close()
	}
	return WallpaperResult{Spec: result.Spec, Applied: result.Applied}, err
}

// fileEntry es un archivo de fondo detectado en disco.
type fileEntry struct {
	path string
	kind string // image, gif, video
}

// scanWallpapers recorre los directorios indicados en busca de fondos.
func scanWallpapers(dirs []string) []fileEntry {
	seen := map[string]bool{}
	var out []fileEntry
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// Se limita la profundidad para no escanear todo el sistema.
				rel, _ := filepath.Rel(dir, path)
				if rel != "." && strings.Count(rel, string(os.PathSeparator)) >= 2 {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[path] {
				return nil
			}
			kind := ""
			switch strings.ToLower(filepath.Ext(path)) {
			case ".png", ".jpg", ".jpeg", ".webp", ".bmp":
				kind = "image"
			case ".gif":
				kind = "gif"
			case ".mp4", ".webm", ".mkv", ".mov":
				kind = "video"
			}
			if kind == "" {
				return nil
			}
			seen[path] = true
			out = append(out, fileEntry{path: path, kind: kind})
			return nil
		})
		_ = err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func humanBytes(n int64) string {
	switch {
	case n < 1024:
		return formatInt(int(n)) + " B"
	case n < 1024*1024:
		return formatInt(int(n/1024)) + " KB"
	case n < 1024*1024*1024:
		return formatInt(int(n/(1024*1024))) + " MB"
	}
	return formatInt(int(n/(1024*1024*1024))) + " GB"
}
