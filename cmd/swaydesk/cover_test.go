package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDemoCover(t *testing.T) {
	dir := t.TempDir()
	path := demoCover(dir, "sakura")
	if path == "" {
		t.Fatal("no se generó la carátula")
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("ruta inesperada: %s", path)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("archivo vacío: %v", err)
	}
	// La imagen generada debe poder volver a decodificarse.
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		t.Fatalf("PNG inválido: %v", err)
	}
	if im.Bounds().Dx() != 120 || im.Bounds().Dy() != 120 {
		t.Fatalf("tamaño = %v", im.Bounds())
	}
}
