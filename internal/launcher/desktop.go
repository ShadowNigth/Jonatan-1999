// Package launcher descubre aplicaciones .desktop, ejecutables del PATH y
// guarda un historial de uso con puntuación "frecencia" (frecuencia + recencia).
package launcher

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Entry es una aplicación o comando lanzable.
type Entry struct {
	Key         string
	Name        string
	Comment     string
	Exec        string
	Icon        string
	Categories  string
	Terminal    bool
	Source      string // "desktop" o "path"
	DesktopFile string
}

// ParseDesktopFile lee un archivo .desktop y devuelve su entrada principal.
func ParseDesktopFile(path string) (Entry, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Entry{}, false
	}
	defer f.Close()

	entry := Entry{Source: "desktop", DesktopFile: path}
	inMain := false
	noDisplay := false
	hidden := false
	term := false

	// Se prefieren los nombres localizados en español si existen.
	lang := strings.ToLower(os.Getenv("LANG"))
	spanish := strings.HasPrefix(lang, "es")
	nameES, commentES := "", ""

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			sec := strings.Trim(line, "[]")
			inMain = sec == "Desktop Entry"
			if !first && !inMain && entry.Name != "" {
				// Solo interesa la primera sección [Desktop Entry].
				break
			}
			first = false
			continue
		}
		if !inMain {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		switch key {
		case "Name":
			entry.Name = unescapeDesktop(val)
		case "Name[es]":
			nameES = unescapeDesktop(val)
		case "Comment":
			entry.Comment = unescapeDesktop(val)
		case "Comment[es]":
			commentES = unescapeDesktop(val)
		case "Exec":
			entry.Exec = unescapeDesktop(val)
		case "Icon":
			entry.Icon = unescapeDesktop(val)
		case "Categories":
			entry.Categories = val
		case "Terminal":
			term = strings.EqualFold(val, "true")
		case "NoDisplay":
			noDisplay = strings.EqualFold(val, "true")
		case "Hidden":
			hidden = strings.EqualFold(val, "true")
		}
	}
	entry.Terminal = term
	if spanish {
		if nameES != "" {
			entry.Name = nameES
		}
		if commentES != "" {
			entry.Comment = commentES
		}
	}
	if entry.Name == "" || entry.Exec == "" || noDisplay || hidden {
		return Entry{}, false
	}
	entry.Key = "app:" + strings.ToLower(entry.Name)
	return entry, true
}

func unescapeDesktop(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	r := strings.NewReplacer(`\s`, " ", `\n`, "\n", `\t`, "\t", `\r`, "\r", `\\`, `\`)
	return r.Replace(s)
}

// dataDirs devuelve los directorios de datos XDG.
func dataDirs() []string {
	var dirs []string
	if h, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(h, ".local", "share"))
	}
	if v := os.Getenv("XDG_DATA_DIRS"); v != "" {
		for _, d := range strings.Split(v, ":") {
			if d != "" {
				dirs = append(dirs, d)
			}
		}
	} else {
		dirs = append(dirs, "/usr/local/share", "/usr/share")
	}
	return dirs
}

// DesktopEntries recorre los directorios XDG y devuelve las aplicaciones.
// Las entradas con el mismo nombre en directorios de mayor prioridad ganan.
func DesktopEntries() []Entry {
	seen := map[string]bool{}
	var out []Entry
	for _, base := range dataDirs() {
		dir := filepath.Join(base, "applications")
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			entry, ok := ParseDesktopFile(filepath.Join(dir, e.Name()))
			if !ok || seen[entry.Key] {
				continue
			}
			seen[entry.Key] = true
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// ExecArgs convierte el campo Exec de un .desktop en argumentos de línea de
// comandos, eliminando los códigos de campo (%f, %U, %i...).
func ExecArgs(exec string) []string {
	fields := splitQuoted(exec)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f, "%") && len(f) <= 2 {
			continue
		}
		f = strings.ReplaceAll(f, "%%", "%")
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// splitQuoted separa respetando comillas simples y dobles y barras invertidas.
func splitQuoted(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// PathCommands enumera los ejecutables del PATH sin duplicados.
func PathCommands() []Entry {
	seen := map[string]bool{}
	var out []Entry
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || strings.HasPrefix(name, ".") || seen[name] {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Mode()&0o111 == 0 {
				continue
			}
			seen[name] = true
			out = append(out, Entry{
				Key: "cmd:" + name, Name: name, Exec: name, Source: "path",
				Comment: "comando",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// History guarda la puntuación de uso de cada entrada.
type History struct {
	path    string
	entries map[string]histEntry
	max     int
}

type histEntry struct {
	Count int   `json:"count"`
	Last  int64 `json:"last"`
}

// LoadHistory lee el historial desde el archivo indicado.
func LoadHistory(path string, maxEntries int) *History {
	h := &History{path: path, entries: map[string]histEntry{}, max: maxEntries}
	if maxEntries <= 0 {
		h.max = 200
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return h
	}
	// Formato sencillo: clave<TAB>count<TAB>timestamp por línea. Se evita
	// encoding/json para que el archivo sea fácil de inspeccionar a mano.
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}
		count, err1 := strconv.Atoi(parts[1])
		last, err2 := strconv.ParseInt(parts[2], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		h.entries[parts[0]] = histEntry{Count: count, Last: last}
	}
	return h
}

// Score calcula la frecuencia ponderada por recencia (0 si nunca se usó).
func (h *History) Score(key string, now int64) int {
	e, ok := h.entries[key]
	if !ok {
		return 0
	}
	age := now - e.Last
	if age < 0 {
		age = 0
	}
	days := age / 86400
	decay := 100
	if days > 0 {
		decay = 100 / (1 + int(days)/3)
		if decay < 5 {
			decay = 5
		}
	}
	return e.Count * decay
}

// Record registra el uso de una entrada y persiste el historial.
func (h *History) Record(key string, now int64) {
	e := h.entries[key]
	e.Count++
	e.Last = now
	h.entries[key] = e
	h.persist()
}

// persist reescribe el archivo recortando a las entradas más usadas.
func (h *History) persist() {
	type kv struct {
		key string
		e   histEntry
	}
	all := make([]kv, 0, len(h.entries))
	for k, v := range h.entries {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].e.Count > all[j].e.Count })
	if len(all) > h.max {
		all = all[:h.max]
	}
	h.entries = make(map[string]histEntry, len(all))
	var b strings.Builder
	for _, item := range all {
		h.entries[item.key] = item.e
		b.WriteString(item.key)
		b.WriteByte('\t')
		b.WriteString(strconv.Itoa(item.e.Count))
		b.WriteByte('\t')
		b.WriteString(strconv.FormatInt(item.e.Last, 10))
		b.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(h.path, []byte(b.String()), 0o644)
}
