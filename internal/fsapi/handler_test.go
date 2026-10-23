package fsapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerSafeResolveAndRead(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "hello.txt"), []byte("hola arch"), 0o644)
	_ = os.MkdirAll(filepath.Join(tmp, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, ".git", "config"), []byte("secret"), 0o644)

	h := NewHandler(tmp)

	// Valid read
	req := httptest.NewRequest(http.MethodGet, "/api/fs/read?path=hello.txt", nil)
	rec := httptest.NewRecorder()
	h.HandleRead(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hola arch") {
		t.Fatalf("expected 200 and content, got %d: %s", rec.Code, rec.Body.String())
	}

	// Block .git access
	reqGit := httptest.NewRequest(http.MethodGet, "/api/fs/read?path=.git/config", nil)
	recGit := httptest.NewRecorder()
	h.HandleRead(recGit, reqGit)
	if recGit.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for .git access, got %d", recGit.Code)
	}
}
