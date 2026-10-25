package osd

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const maxArtworkBytes = 8 << 20
const maxArtworkSide = 2048
const maxArtworkPixels = maxArtworkSide * maxArtworkSide

// LoadArtwork carga una carátula MPRIS desde file://, una ruta local o HTTP(S).
// El tamaño de la descarga y las dimensiones se limitan para evitar que una
// imagen remota o malformada consuma memoria sin control.
func LoadArtwork(source string) (image.Image, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("la carátula no tiene ruta")
	}
	data, err := readArtwork(source)
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("carátula ilegible: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxArtworkSide || cfg.Height > maxArtworkSide || cfg.Width > maxArtworkPixels/cfg.Height {
		return nil, fmt.Errorf("dimensiones de carátula no admitidas: %dx%d", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("carátula ilegible: %w", err)
	}
	return img, nil
}

func readArtwork(source string) ([]byte, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, err
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "", "file":
		path := source
		if scheme == "file" {
			if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
				return nil, fmt.Errorf("host file:// no permitido: %s", u.Host)
			}
			path = u.Path
			if path == "" {
				path = u.Opaque
			}
		}
		if path == "" {
			return nil, fmt.Errorf("ruta de carátula vacía")
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		st, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("la ruta de carátula no es un archivo regular")
		}
		if st.Size() > maxArtworkBytes {
			return nil, fmt.Errorf("carátula demasiado grande (%d bytes)", st.Size())
		}
		return readLimited(file)
	case "http", "https":
		client := &http.Client{Timeout: 4 * time.Second}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "swaydesk-media-osd/1.0")
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("descarga de carátula: HTTP %s", res.Status)
		}
		if res.ContentLength > maxArtworkBytes {
			return nil, fmt.Errorf("carátula demasiado grande (%d bytes)", res.ContentLength)
		}
		return readLimited(res.Body)
	default:
		return nil, fmt.Errorf("esquema de carátula no admitido: %s", u.Scheme)
	}
}

func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxArtworkBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxArtworkBytes {
		return nil, fmt.Errorf("carátula demasiado grande")
	}
	return data, nil
}
