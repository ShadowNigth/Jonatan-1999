package ui

import (
	"time"
)

// Loop ejecuta el bucle principal de una aplicación TUI: dibuja, espera
// teclas o redimensionados y, opcionalmente, ticks periódicos.
//
// handle debe devolver true para salir del bucle.
func (s *Screen) Loop(render func(w, h int) string, handle func(k Key) (quit bool), tick <-chan time.Time) error {
	keys := s.Keys.Keys()
	for {
		s.Draw(render(s.W, s.H))
		select {
		case k, ok := <-keys:
			if !ok {
				return nil
			}
			// Ctrl+C siempre cierra de forma limpia.
			if k.Ctrl && (k.Rune == 'c' || k.Name == "ctrl+c") {
				return nil
			}
			if handle(k) {
				return nil
			}
		case <-s.Resize:
			s.UpdateSize()
		case <-tick:
			// solo fuerza un redibujado
		}
	}
}
