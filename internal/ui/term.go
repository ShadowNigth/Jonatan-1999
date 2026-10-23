package ui

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// Terminal encapsula el estado del terminal para dibujar interfaces a pantalla
// completa: modo crudo, pantalla alternativa y tamaño de la ventana.
type Terminal struct {
	In   *os.File
	Out  *os.File
	orig syscall.Termios
	raw  bool
	buf  []byte
}

// NewTerminal crea un terminal sobre la entrada y salida estándar.
func NewTerminal() *Terminal {
	return &Terminal{In: os.Stdin, Out: os.Stdout}
}

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// IsTTY informa si la entrada es un terminal interactivo.
func (t *Terminal) IsTTY() bool {
	var term syscall.Termios
	return ioctl(t.In.Fd(), syscall.TCGETS, unsafe.Pointer(&term)) == nil
}

// EnterRaw activa el modo crudo: sin eco, sin línea canónica y sin señales.
func (t *Terminal) EnterRaw() error {
	var term syscall.Termios
	if err := ioctl(t.In.Fd(), syscall.TCGETS, unsafe.Pointer(&term)); err != nil {
		return err
	}
	t.orig = term
	term.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	term.Oflag &^= syscall.OPOST
	term.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	term.Cflag &^= syscall.CSIZE | syscall.PARENB
	term.Cflag |= syscall.CS8
	term.Cc[syscall.VMIN] = 1
	term.Cc[syscall.VTIME] = 0
	if err := ioctl(t.In.Fd(), syscall.TCSETS, unsafe.Pointer(&term)); err != nil {
		return err
	}
	t.raw = true
	return nil
}

// ExitRaw restaura el estado original del terminal.
func (t *Terminal) ExitRaw() error {
	if !t.raw {
		return nil
	}
	t.raw = false
	return ioctl(t.In.Fd(), syscall.TCSETS, unsafe.Pointer(&t.orig))
}

// Size devuelve el ancho y alto actuales de la ventana en celdas.
func (t *Terminal) Size() (int, int) {
	var ws winsize
	if err := ioctl(t.Out.Fd(), syscall.TIOCGWINSZ, unsafe.Pointer(&ws)); err == nil && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Col), int(ws.Row)
	}
	w, h := 80, 24
	if v, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && v > 0 {
		w = v
	}
	if v, err := strconv.Atoi(os.Getenv("LINES")); err == nil && v > 0 {
		h = v
	}
	return w, h
}

// AltScreen activa o desactiva la pantalla alternativa.
func (t *Terminal) AltScreen(on bool) {
	if on {
		t.Write("\x1b[?1049h\x1b[H\x1b[2J")
	} else {
		t.Write("\x1b[?1049l")
	}
}

// HideCursor oculta o muestra el cursor.
func (t *Terminal) HideCursor(hide bool) {
	if hide {
		t.Write("\x1b[?25l")
	} else {
		t.Write("\x1b[?25h")
	}
}

// Write escribe directamente en la salida.
func (t *Terminal) Write(s string) {
	_, _ = t.Out.WriteString(s)
}

// Flush vuelca el búfer interno al terminal.
func (t *Terminal) Flush() {
	if len(t.buf) > 0 {
		_, _ = t.Out.Write(t.buf)
		t.buf = t.buf[:0]
	}
}

// Print acumula texto en el búfer (se envía con Flush o al llenarse).
func (t *Terminal) Print(s string) {
	t.buf = append(t.buf, s...)
	if len(t.buf) > 32*1024 {
		t.Flush()
	}
}

// Draw dibuja un fotograma completo: mueve el cursor al inicio, escribe el
// contenido y limpia lo que sobre.
func (t *Terminal) Draw(frame string) {
	w, h := t.Size()
	lines := splitLines(frame, h)
	var b []byte
	b = append(b, "\x1b[H"...)
	for i := 0; i < h; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		line = TruncateANSI(line, w)
		b = append(b, line...)
		if i < h-1 {
			b = append(b, "\x1b[K\r\n"...)
		}
	}
	b = append(b, "\x1b[J"...)
	_, _ = t.Out.Write(b)
}

func splitLines(s string, max int) []string {
	var out []string
	start := 0
	for i := 0; i < len(s) && len(out) < max; i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if len(out) < max && start <= len(s) {
		out = append(out, s[start:])
	}
	return out
}

// MoveTo coloca el cursor en la fila y columna indicadas (base 1).
func (t *Terminal) MoveTo(row, col int) {
	t.Print("\x1b[" + strconv.Itoa(row) + ";" + strconv.Itoa(col) + "H")
}

// Clear limpia la pantalla y lleva el cursor a casa.
func (t *Terminal) Clear() { t.Write("\x1b[H\x1b[2J") }
