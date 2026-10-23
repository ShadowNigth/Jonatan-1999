package ui

import (
	"bufio"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// Key representa una pulsación de tecla ya decodificada.
type Key struct {
	// Rune es el carácter imprimible (0 si no lo es).
	Rune rune
	// Name es el nombre de la tecla para las no imprimibles:
	// "up", "down", "left", "right", "home", "end", "pgup", "pgdn",
	// "delete", "insert", "enter", "esc", "tab", "backtab", "backspace",
	// "space", "f1".."f12".
	Name string
	// Ctrl y Alt indican modificadores.
	Ctrl bool
	Alt  bool
}

// String devuelve una representación legible de la tecla.
func (k Key) String() string {
	var parts []string
	if k.Ctrl {
		parts = append(parts, "ctrl")
	}
	if k.Alt {
		parts = append(parts, "alt")
	}
	if k.Name != "" {
		parts = append(parts, k.Name)
	} else {
		parts = append(parts, string(k.Rune))
	}
	return strings.Join(parts, "+")
}

// Is verifica si la tecla corresponde a un nombre concreto ("enter", "esc"...).
func (k Key) Is(name string) bool { return k.Name == name }

// IsRune verifica si la tecla es un carácter concreto.
func (k Key) IsRune(r rune) bool { return k.Rune == r && k.Name == "" }

// KeyReader decodifica pulsaciones desde un lector de bytes.
type KeyReader struct {
	ch   chan byte
	back []byte
}

// NewKeyReader crea un lector de teclas sobre r.
func NewKeyReader(r io.Reader) *KeyReader {
	k := &KeyReader{ch: make(chan byte, 512)}
	br := bufio.NewReaderSize(r, 8192)
	go func() {
		for {
			b, err := br.ReadByte()
			if err != nil {
				close(k.ch)
				return
			}
			k.ch <- b
		}
	}()
	return k
}

// next obtiene el siguiente byte, esperando como máximo timeout (0 = infinito).
func (k *KeyReader) next(timeout time.Duration) (byte, bool) {
	if n := len(k.back); n > 0 {
		b := k.back[n-1]
		k.back = k.back[:n-1]
		return b, true
	}
	if timeout <= 0 {
		b, ok := <-k.ch
		return b, ok
	}
	select {
	case b, ok := <-k.ch:
		return b, ok
	case <-time.After(timeout):
		return 0, false
	}
}

// unget devuelve un byte al flujo.
func (k *KeyReader) unget(b byte) { k.back = append(k.back, b) }

const escTimeout = 35 * time.Millisecond

// Read bloquea hasta decodificar una tecla.
func (k *KeyReader) Read() (Key, error) {
	b, ok := k.next(0)
	if !ok {
		return Key{}, io.EOF
	}
	return k.decodeFirst(b)
}

func (k *KeyReader) decodeFirst(b byte) (Key, error) {
	switch {
	case b == 0x1b:
		return k.decodeEscape()
	case b == '\r' || b == '\n':
		return Key{Name: "enter"}, nil
	case b == '\t':
		return Key{Name: "tab"}, nil
	case b == 0x7f || b == 0x08:
		return Key{Name: "backspace"}, nil
	case b == ' ':
		return Key{Rune: ' '}, nil
	case b < 0x20:
		// Ctrl + letra: 0x01 -> ctrl+a
		return Key{Rune: rune('a' + b - 1), Ctrl: true, Name: "ctrl+" + string(rune('a'+b-1))}, nil
	}
	// UTF-8 multibyte.
	if b < 0x80 {
		return Key{Rune: rune(b)}, nil
	}
	width := 0
	switch {
	case b&0xe0 == 0xc0:
		width = 2
	case b&0xf0 == 0xe0:
		width = 3
	case b&0xf8 == 0xf0:
		width = 4
	default:
		return Key{Rune: utf8.RuneError}, nil
	}
	buf := []byte{b}
	for i := 1; i < width; i++ {
		nb, ok := k.next(escTimeout)
		if !ok || nb&0xc0 != 0x80 {
			return Key{Rune: utf8.RuneError}, nil
		}
		buf = append(buf, nb)
	}
	r, _ := utf8.DecodeRune(buf)
	return Key{Rune: r}, nil
}

func (k *KeyReader) decodeEscape() (Key, error) {
	b, ok := k.next(escTimeout)
	if !ok {
		return Key{Name: "esc"}, nil
	}
	switch b {
	case '[':
		return k.decodeCSI()
	case 'O':
		return k.decodeSS3()
	}
	// Alt + tecla (incluye Alt+letra y Alt+flechas en algunos terminales).
	inner, err := k.decodeFirst(b)
	if err != nil {
		return Key{Name: "esc"}, err
	}
	if inner.Name == "esc" {
		return inner, nil
	}
	inner.Alt = true
	return inner, nil
}

func (k *KeyReader) decodeCSI() (Key, error) {
	var params []byte
	for {
		b, ok := k.next(escTimeout)
		if !ok {
			return Key{Name: "esc"}, nil
		}
		if b >= 0x40 && b <= 0x7e {
			return csiKey(string(params), b), nil
		}
		params = append(params, b)
		if len(params) > 32 {
			return Key{Name: "esc"}, nil
		}
	}
}

func csiKey(params string, final byte) Key {
	mod := 0
	if i := strings.IndexByte(params, ';'); i >= 0 {
		// El segundo parámetro es el modificador (2 = shift, 3 = alt, 5 = ctrl).
		p := params[i+1:]
		if j := strings.IndexByte(p, ';'); j >= 0 {
			p = p[:j]
		}
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				n = 0
				break
			}
			n = n*10 + int(c-'0')
		}
		switch n {
		case 2:
			mod = 1
		case 3:
			mod = 2
		case 5:
			mod = 4
		}
	}
	key := Key{}
	switch final {
	case 'A':
		key.Name = "up"
	case 'B':
		key.Name = "down"
	case 'C':
		key.Name = "right"
	case 'D':
		key.Name = "left"
	case 'H':
		key.Name = "home"
	case 'F':
		key.Name = "end"
	case 'Z':
		key.Name = "backtab"
	case '~':
		n := 0
		for _, c := range params {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		switch n {
		case 1, 7:
			key.Name = "home"
		case 2:
			key.Name = "insert"
		case 3:
			key.Name = "delete"
		case 4, 8:
			key.Name = "end"
		case 5:
			key.Name = "pgup"
		case 6:
			key.Name = "pgdn"
		case 11:
			key.Name = "f1"
		case 12:
			key.Name = "f2"
		case 13:
			key.Name = "f3"
		case 14:
			key.Name = "f4"
		case 15:
			key.Name = "f5"
		case 17:
			key.Name = "f6"
		case 18:
			key.Name = "f7"
		case 19:
			key.Name = "f8"
		case 20:
			key.Name = "f9"
		case 21:
			key.Name = "f10"
		case 23:
			key.Name = "f11"
		case 24:
			key.Name = "f12"
		}
	default:
		key.Name = "esc"
	}
	if mod&2 != 0 {
		key.Alt = true
	}
	if mod&4 != 0 {
		key.Ctrl = true
	}
	return key
}

func (k *KeyReader) decodeSS3() (Key, error) {
	b, ok := k.next(escTimeout)
	if !ok {
		return Key{Name: "esc"}, nil
	}
	switch b {
	case 'A':
		return Key{Name: "up"}, nil
	case 'B':
		return Key{Name: "down"}, nil
	case 'C':
		return Key{Name: "right"}, nil
	case 'D':
		return Key{Name: "left"}, nil
	case 'H':
		return Key{Name: "home"}, nil
	case 'F':
		return Key{Name: "end"}, nil
	case 'P', 'Q', 'R', 'S':
		return Key{Name: "f" + string(rune('1'+b-'P'))}, nil
	}
	return Key{Name: "esc"}, nil
}

// Keys devuelve un canal por el que se reciben las teclas pulsadas.
func (k *KeyReader) Keys() <-chan Key {
	ch := make(chan Key, 16)
	go func() {
		defer close(ch)
		for {
			key, err := k.Read()
			if err != nil {
				return
			}
			ch <- key
		}
	}()
	return ch
}
