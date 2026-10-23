package main

import (
	"fmt"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/osd"
	"github.com/ShadowNigth/swaydesk/internal/wayland"
)

// showOverlay abre una superficie flotante, dibuja el aviso con un fundido de
// entrada y salida, y la cierra al terminar.
//
// La superficie no recibe clics (región de entrada vacía): el aviso nunca
// estorba a lo que hay debajo.
func showOverlay(cfg *config.Config, r *osd.Renderer, st osd.State, hold time.Duration) error {
	wl, err := wayland.NewConn()
	if err != nil {
		return fmt.Errorf("Wayland: %w", err)
	}
	if _, err := wl.GetRegistry(); err != nil {
		return fmt.Errorf("Wayland: %w", err)
	}

	// La tarjeta se dibuja en píxeles reales: la escala ya está aplicada en la
	// fuente del renderizador, así que no se reescala nada aquí.
	pw, ph := r.Size(st)
	anchors, mt, mb, ml, mr := osdAnchors(cfg.OSDPosition, cfg.OSDMargin)

	panel, err := wayland.NewPanel(wl, wayland.PanelSpec{
		Namespace:     "swaydesk-osd",
		Layer:         wayland.LayerOverlay,
		Width:         pw,
		Height:        ph,
		Anchors:       anchors,
		ExclusiveZone: 0,
		MarginTop:     mt,
		MarginBottom:  mb,
		MarginLeft:    ml,
		MarginRight:   mr,
	})
	if err != nil {
		return err
	}
	defer panel.Close()
	if err := panel.InitShm(2); err != nil {
		return err
	}
	// Nada de la superficie captura el ratón.
	_ = panel.SetInputRegion(nil)

	draw := func(alpha float64) error {
		canvas := r.Render(st, alpha)
		return panel.Draw(canvas.Pix, canvas.W, canvas.H)
	}

	fade := cfg.OSDFade
	if fade < 0 {
		fade = 0
	}
	const pasos = 6
	// Entrada: de transparente a opaco.
	for i := 1; i <= pasos && fade > 0; i++ {
		if err := draw(float64(i) / pasos); err != nil {
			return nil // el compositor la cerró: no es un fallo del OSD
		}
		_ = panel.Dispatch(time.Duration(fade/pasos) * time.Millisecond)
	}
	if err := draw(1); err != nil {
		return nil
	}

	// Espera: se sigue atendiendo al compositor para que no se acumulen
	// eventos (y por si cierra la superficie).
	fin := time.Now().Add(hold)
	for time.Now().Before(fin) {
		resto := time.Until(fin)
		if resto > 100*time.Millisecond {
			resto = 100 * time.Millisecond
		}
		if err := panel.Dispatch(resto); err != nil {
			if panel.Closed() {
				return nil
			}
			return err
		}
	}

	// Salida: de opaco a transparente.
	for i := pasos - 1; i >= 0 && fade > 0; i-- {
		if err := draw(float64(i) / pasos); err != nil {
			return nil
		}
		_ = panel.Dispatch(time.Duration(fade/pasos) * time.Millisecond)
	}
	return nil
}

// osdAnchors traduce la posición configurada en anclas y márgenes.
func osdAnchors(pos string, margin int) (anchors uint32, mt, mb, ml, mr int) {
	if margin < 0 {
		margin = 0
	}
	switch pos {
	case "center", "centro":
		return 0, 0, 0, 0, 0
	case "bottom-right", "abajo-derecha":
		return wayland.AnchorBottom | wayland.AnchorRight, 0, margin, 0, margin
	case "bottom-left", "abajo-izquierda":
		return wayland.AnchorBottom | wayland.AnchorLeft, 0, margin, margin, 0
	case "top-center", "arriba-centro":
		return wayland.AnchorTop, margin, 0, 0, 0
	case "center-bottom", "abajo-centro":
		return wayland.AnchorBottom, 0, margin, 0, 0
	}
	// Por defecto: abajo y centrado, como los OSD de los escritorios grandes.
	return wayland.AnchorBottom, 0, margin, 0, 0
}
