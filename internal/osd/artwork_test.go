package osd

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadArtworkFromFileURL(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	img.Set(3, 5, color.RGBA{R: 255, A: 255})
	path := filepath.Join(t.TempDir(), "cover image.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := LoadArtwork((&url.URL{Scheme: "file", Path: path}).String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 24 || got.Bounds().Dy() != 16 {
		t.Fatalf("dimensiones de carátula = %v", got.Bounds())
	}
}

func TestLoadArtworkFromHTTP(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data.Bytes())
	}))
	defer server.Close()

	got, err := LoadArtwork(server.URL + "/cover.png")
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 10 || got.Bounds().Dy() != 10 {
		t.Fatalf("dimensiones de carátula HTTP = %v", got.Bounds())
	}
}

func TestLoadArtworkRejectsUnsupportedAndOversized(t *testing.T) {
	if _, err := LoadArtwork("ftp://example.invalid/cover.png"); err == nil {
		t.Fatal("se aceptó un esquema remoto no permitido")
	}
	if _, err := LoadArtwork(""); err == nil {
		t.Fatal("se aceptó una ruta vacía")
	}
	path := filepath.Join(t.TempDir(), "oversized.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	tooWide := image.NewRGBA(image.Rect(0, 0, maxArtworkSide+1, 1))
	if err := png.Encode(file, tooWide); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadArtwork(path); err == nil {
		t.Fatal("se aceptó una carátula fuera de las dimensiones máximas")
	}
}
