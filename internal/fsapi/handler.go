package fsapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Entry struct {
	Name    string `json:"name"`
	RelPath string `json:"relPath"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	Ext     string `json:"ext"`
}

type Handler struct {
	rootDir string
}

func NewHandler(rootDir string) *Handler {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		abs = rootDir
	}
	return &Handler{rootDir: abs}
}

func (h *Handler) resolveSafe(rel string) (string, string, error) {
	cleaned := filepath.Clean("/" + strings.TrimSpace(rel))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		cleaned = ""
	}
	target := filepath.Join(h.rootDir, cleaned)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	relCheck, err := filepath.Rel(h.rootDir, absTarget)
	if err != nil || strings.HasPrefix(relCheck, "..") {
		return "", "", errors.New("ruta fuera del directorio permitido")
	}
	if relCheck == "." {
		relCheck = ""
	}
	for _, seg := range strings.Split(filepath.ToSlash(relCheck), "/") {
		if seg == ".git" {
			return "", "", errors.New("acceso restringido a .git")
		}
	}
	return absTarget, filepath.ToSlash(relCheck), nil
}

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	relQuery := r.URL.Query().Get("path")
	absPath, safeRel, err := h.resolveSafe(relQuery)
	if err != nil {
		writeJSONError(w, http.StatusForbidden, err.Error())
		return
	}

	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	items := make([]Entry, 0, len(dirEntries))
	for _, de := range dirEntries {
		name := de.Name()
		if name == ".git" {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		childRel := name
		if safeRel != "" {
			childRel = safeRel + "/" + name
		}
		items = append(items, Entry{
			Name:    name,
			RelPath: childRel,
			IsDir:   de.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
			Ext:     strings.ToLower(strings.TrimPrefix(filepath.Ext(name), ".")),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"root":    filepath.Base(h.rootDir),
		"path":    safeRel,
		"entries": items,
	})
}

func (h *Handler) HandleRead(w http.ResponseWriter, r *http.Request) {
	relQuery := r.URL.Query().Get("path")
	absPath, safeRel, err := h.resolveSafe(relQuery)
	if err != nil {
		writeJSONError(w, http.StatusForbidden, err.Error())
		return
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "no es un archivo válido")
		return
	}
	if info.Size() > 512*1024 {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "archivo mayor a 512 KB")
		return
	}
	f, err := os.Open(absPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":    safeRel,
		"name":    filepath.Base(absPath),
		"size":    info.Size(),
		"content": string(data),
	})
}

func (h *Handler) HandleWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 512*1024)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		writeJSONError(w, http.StatusBadRequest, "ruta vacía")
		return
	}
	absPath, safeRel, err := h.resolveSafe(req.Path)
	if err != nil {
		writeJSONError(w, http.StatusForbidden, err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(absPath, []byte(req.Content), 0o644); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"path": safeRel,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}
