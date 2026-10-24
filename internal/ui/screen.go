package ui

import (
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Screen gestiona una interfaz a pantalla completa: modo crudo, pantalla
// alternativa, cursor oculto y redimensionado.
type Screen struct {
	Term   *Terminal
	Keys   *KeyReader
	Resize <-chan os.Signal
	W, H   int
	closed bool
	sigCh  chan os.Signal
	once   sync.Once
}

// ErrNotTTY se devuelve cuando no hay terminal interactivo.
var ErrNotTTY = errors.New("se requiere un terminal interactivo (TTY)")

// OpenScreen prepara el terminal para dibujar una interfaz interactiva.
func OpenScreen() (*Screen, error) {
	t := NewTerminal()
	if !t.IsTTY() {
		return nil, ErrNotTTY
	}
	if err := t.EnterRaw(); err != nil {
		return nil, err
	}
	t.AltScreen(true)
	t.HideCursor(true)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	s := &Screen{Term: t, Keys: NewKeyReader(t.In), Resize: sig, sigCh: sig}
	s.W, s.H = t.Size()
	return s, nil
}

// Close restaura el terminal.
func (s *Screen) Close() {
	s.once.Do(func() {
		s.closed = true
		signal.Stop(s.sigCh)
		if s.Keys != nil {
			s.Keys.Close()
		}
		s.Term.HideCursor(false)
		s.Term.AltScreen(false)
		_ = s.Term.ExitRaw()
	})
}

// UpdateSize vuelve a leer el tamaño del terminal.
func (s *Screen) UpdateSize() {
	s.W, s.H = s.Term.Size()
}

// Draw muestra un fotograma completo.
func (s *Screen) Draw(frame string) { s.Term.Draw(frame) }

// NextKey espera la siguiente pulsación.
func (s *Screen) NextKey() (Key, error) { return s.Keys.Read() }

// Interrupted informa si la pantalla debe cerrarse por una señal externa.
func (s *Screen) Interrupted() bool { return s.closed }
