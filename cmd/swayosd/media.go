package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
	"github.com/ShadowNigth/swaydesk/internal/mpris"
	"github.com/ShadowNigth/swaydesk/internal/osd"
)

func runMedia(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: swayosd media <show|watch>")
		return
	}
	switch args[0] {
	case "show", "mostrar":
		runMediaShow(args[1:])
	case "watch", "vigilar":
		cfg, err := config.Load()
		if err != nil {
			fail(err)
		}
		if cfg.OSDMediaEnabled {
			vals, _ := parseFlags(args[1:], nil)
			runMediaWatch(cfg, scaleFor(cfg, last(vals, "scale")), last(vals, "theme"))
		}
	default:
		fail(fmt.Errorf("orden media desconocida %q (usa show o watch)", args[0]))
	}
}

// runMediaWatch escucha señales MPRIS y delega cada tarjeta a un proceso
// independiente para que el aviso pueda desaparecer sin bloquear al vigilante.
func runMediaWatch(cfg *config.Config, scale int, themeName string) {
	exe, err := os.Executable()
	if err != nil {
		exe = "swayosd"
	}
	lastError := ""
	for {
		err := mpris.Watch(context.Background(), func(track mpris.Track) {
			if !cfg.OSDMediaEnabled || !track.Valid() {
				return
			}
			args := []string{"media", "show", "--title", track.Title}
			if track.Artist != "" {
				args = append(args, "--artist", track.Artist)
			}
			if track.Album != "" {
				args = append(args, "--album", track.Album)
			}
			if track.ArtworkURL != "" {
				args = append(args, "--art-url", track.ArtworkURL)
			}
			if track.PlaybackStatus != "" {
				args = append(args, "--status", track.PlaybackStatus)
			}
			if scale > 0 {
				args = append(args, "--scale", fmt.Sprint(scale))
			}
			if themeName != "" {
				args = append(args, "--theme", themeName)
			}
			if err := launcher.StartDetached(exe, args...); err != nil {
				fmt.Fprintf(os.Stderr, "swayosd: no se pudo mostrar el OSD de reproducción: %v\n", err)
			}
		})
		if err == nil {
			return
		}
		if err.Error() != lastError {
			fmt.Fprintf(os.Stderr, "swayosd: MPRIS no disponible: %v (se reintentará)\n", err)
			lastError = err.Error()
		}
		time.Sleep(3 * time.Second)
	}
}

func runMediaShow(args []string) {
	vals, _ := parseFlags(args, nil)
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	if !cfg.OSDMediaEnabled {
		return
	}

	state := osd.MediaState{
		Title:          last(vals, "title"),
		Artist:         last(vals, "artist"),
		Album:          last(vals, "album"),
		PlaybackStatus: last(vals, "status"),
	}
	if artURL := last(vals, "art-url"); artURL != "" {
		// La tarjeta sigue apareciendo aunque el reproductor retire la carátula
		// antes de que podamos leerla o el archivo no esté disponible.
		state.Artwork, _ = osd.LoadArtwork(artURL)
	}

	scale := scaleFor(cfg, last(vals, "scale"))
	r := mediaRendererFor(cfg, scale, 0, last(vals, "theme"))
	hold := time.Duration(cfg.OSDMediaTimeout) * time.Millisecond
	if hold < 100*time.Millisecond {
		hold = 3200 * time.Millisecond
	}
	if value := last(vals, "timeout"); value != "" {
		if ms, err := atoi(value); err == nil && ms >= 100 && ms <= 30000 {
			hold = time.Duration(ms) * time.Millisecond
		}
	}
	if err := showMediaOverlay(cfg, r, state, hold); err != nil {
		fail(err)
	}
}
