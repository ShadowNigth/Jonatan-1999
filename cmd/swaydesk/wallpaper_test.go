package main

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/wallpaper"
)

// Super+Shift+w ejecuta `swaydesk wallpaper --gallery`; si la bandera no se
// reconoce, el atajo parece roto (antes contestaba «no se reconoce el fondo»).
func TestIsGalleryRequest(t *testing.T) {
	yes := [][]string{
		{"--gallery"},
		{"--pick"},
		{"-g"},
		{"gallery"},
	}
	for _, args := range yes {
		if args[0] == "gallery" {
			continue // ese lo resuelve el switch de subcomandos
		}
		if !isGalleryRequest(args) {
			t.Fatalf("%v debería abrir la galería", args)
		}
	}
	no := [][]string{nil, {"set", "scene:petals"}, {"scene:petals"}, {"--scenes"}, {"--live"}}
	for _, args := range no {
		if isGalleryRequest(args) {
			t.Fatalf("%v no debería abrir la galería", args)
		}
	}
}

// Escenas, GIF y vídeo necesitan el demonio; una imagen fija no.
func TestAnimatedSpec(t *testing.T) {
	for _, k := range []wallpaper.Kind{wallpaper.KindScene, wallpaper.KindGIF, wallpaper.KindVideo} {
		if !animatedSpec(wallpaper.Spec{Kind: k}) {
			t.Fatalf("%s debería considerarse animado", k)
		}
	}
	if animatedSpec(wallpaper.Spec{Kind: wallpaper.KindImage}) {
		t.Fatal("una imagen fija no necesita demonio")
	}
}
