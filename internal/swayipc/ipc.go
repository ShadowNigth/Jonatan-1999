// Package swayipc implementa desde cero el protocolo IPC de Sway (compatible
// con i3) usando únicamente la biblioteca estándar de Go.
//
// El protocolo consiste en mensajes con una cabecera de 14 bytes:
//
//	"i3-ipc" + uint32(le) longitud del payload + uint32(le) tipo de mensaje
//
// seguidos del payload en JSON. Toda la comunicación ocurre sobre un socket
// UNIX cuya ruta se publica en la variable de entorno SWAYSOCK.
package swayipc

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Magic identifica los mensajes del protocolo.
	Magic = "i3-ipc"
	// HeaderLen es el tamaño de la cabecera: 6 bytes de magia + 2 enteros de 4 bytes.
	HeaderLen = len(Magic) + 8
)

// MessageType identifica el tipo de mensaje o evento.
type MessageType uint32

// Tipos de mensaje definidos por el protocolo.
const (
	MsgRunCommand MessageType = iota
	MsgGetWorkspaces
	MsgSubscribe
	MsgGetOutputs
	MsgGetTree
	MsgGetMarks
	MsgGetBarConfig
	MsgGetVersion
	MsgGetBindingModes
	MsgGetConfig
	MsgSendTick
	MsgSync
	MsgGetBindingState
)

// Tipos propios de Sway (no existen en i3).
const (
	MsgGetInputs MessageType = 100
	MsgGetSeats  MessageType = 101
)

// eventBit marca los tipos que en realidad son eventos emitidos por sway.
const eventBit MessageType = 1 << 31

// Tipos de evento.
const (
	EventWorkspace MessageType = eventBit + iota
	EventOutput
	EventMode
	EventWindow
	EventBarConfigUpdate
	EventBinding
	EventShutdown
	EventTick
	EventBarStateUpdate
	EventInput
)

// IsEvent informa si el tipo corresponde a un evento.
func (t MessageType) IsEvent() bool { return t&eventBit != 0 }

// String devuelve un nombre legible del tipo de mensaje.
func (t MessageType) String() string {
	switch t {
	case MsgRunCommand:
		return "run_command"
	case MsgGetWorkspaces:
		return "get_workspaces"
	case MsgSubscribe:
		return "subscribe"
	case MsgGetOutputs:
		return "get_outputs"
	case MsgGetTree:
		return "get_tree"
	case MsgGetVersion:
		return "get_version"
	case MsgGetInputs:
		return "get_inputs"
	case MsgGetSeats:
		return "get_seats"
	case EventWorkspace:
		return "workspace"
	case EventWindow:
		return "window"
	case EventMode:
		return "mode"
	case EventTick:
		return "tick"
	case EventShutdown:
		return "shutdown"
	case EventOutput:
		return "output"
	case EventBinding:
		return "binding"
	}
	return fmt.Sprintf("msg(%d)", uint32(t))
}

// Errores habituales del paquete.
var (
	// ErrNoSocket indica que no se encontró el socket de sway.
	ErrNoSocket = errors.New("no se encontró el socket de sway (SWAYSOCK/I3SOCK): ¿está sway en ejecución?")
	// ErrBadMagic indica que el otro extremo no habla el protocolo esperado.
	ErrBadMagic = errors.New("cabecera IPC inválida: no parece un socket de sway/i3")
	// ErrClosed indica que la conexión ya fue cerrada.
	ErrClosed = errors.New("conexión IPC cerrada")
)

// Conn representa una conexión al socket IPC de sway.
//
// Un Conn puede compartirse entre goroutines: las peticiones se serializan con
// un mutex, ya que el protocolo no permite entrelazar respuestas.
type Conn struct {
	nc        net.Conn
	br        *bufio.Reader
	mu        sync.Mutex
	timeout   time.Duration
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

// SwaySock devuelve la ruta del socket IPC. Consulta SWAYSOCK, luego I3SOCK y,
// como último recurso, pregunta al binario de sway.
func SwaySock() string {
	for _, env := range []string{"SWAYSOCK", "I3SOCK"} {
		if p := strings.TrimSpace(os.Getenv(env)); p != "" {
			return p
		}
	}
	out, err := exec.Command("sway", "--get-socketpath").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Dial se conecta al socket IPC de la sesión actual.
func Dial() (*Conn, error) {
	path := SwaySock()
	if path == "" {
		return nil, ErrNoSocket
	}
	c, err := DialPath(path)
	if err != nil {
		return nil, fmt.Errorf("conectando a %s: %w", path, err)
	}
	return c, nil
}

// DialPath se conecta a un socket IPC concreto (útil para pruebas).
func DialPath(path string) (*Conn, error) {
	nc, err := net.DialTimeout("unix", path, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return newConn(nc), nil
}

func newConn(nc net.Conn) *Conn {
	return &Conn{
		nc:      nc,
		br:      bufio.NewReaderSize(nc, 64*1024),
		timeout: 10 * time.Second,
	}
}

// SetTimeout ajusta el plazo máximo de espera por respuesta (0 = sin límite).
func (c *Conn) SetTimeout(d time.Duration) {
	c.mu.Lock()
	c.timeout = d
	c.mu.Unlock()
}

// Close cierra la conexión. Es seguro llamarlo más de una vez y desbloquea
// una lectura en curso de Next o RoundTrip.
func (c *Conn) Close() error {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.closeErr = c.nc.Close()
	})
	return c.closeErr
}

// RoundTrip envía un mensaje y devuelve el payload JSON de la respuesta.
func (c *Conn) RoundTrip(t MessageType, payload any) (json.RawMessage, error) {
	raw, err := encodePayload(payload)
	if err != nil {
		return nil, err
	}
	return c.RoundTripRaw(t, raw)
}

// RoundTripRaw envía un payload ya codificado.
func (c *Conn) RoundTripRaw(t MessageType, payload []byte) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed.Load() {
		return nil, ErrClosed
	}
	if c.timeout > 0 {
		_ = c.nc.SetDeadline(time.Now().Add(c.timeout))
	} else {
		_ = c.nc.SetDeadline(time.Time{})
	}
	if err := writeFrame(c.nc, t, payload); err != nil {
		return nil, fmt.Errorf("enviando mensaje %s: %w", t, err)
	}
	resp, _, err := readFrame(c.br)
	if err != nil {
		return nil, fmt.Errorf("leyendo respuesta de %s: %w", t, err)
	}
	return resp, nil
}

// writeFrame escribe cabecera + payload en el writer.
func writeFrame(w io.Writer, t MessageType, payload []byte) error {
	buf := make([]byte, HeaderLen+len(payload))
	copy(buf, Magic)
	binary.LittleEndian.PutUint32(buf[6:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(buf[10:], uint32(t))
	copy(buf[HeaderLen:], payload)
	_, err := w.Write(buf)
	return err
}

// readFrame lee un mensaje completo y devuelve su payload y tipo.
func readFrame(r io.Reader) (json.RawMessage, MessageType, error) {
	header := make([]byte, HeaderLen)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, 0, err
	}
	if string(header[:len(Magic)]) != Magic {
		return nil, 0, ErrBadMagic
	}
	n := binary.LittleEndian.Uint32(header[6:10])
	t := MessageType(binary.LittleEndian.Uint32(header[10:14]))
	// Límite defensivo: ningún mensaje legítimo de sway supera unos pocos MiB.
	if n > 64<<20 {
		return nil, 0, fmt.Errorf("payload IPC demasiado grande: %d bytes", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}
	return json.RawMessage(payload), t, nil
}

// encodePayload serializa el payload.
//
// Si es nil se envía una lista vacía, que es lo que espera sway para los
// mensajes que no llevan argumentos. Los []byte y json.RawMessage se envían
// tal cual, y las cadenas se envían como string JSON (no como array).
func encodePayload(payload any) ([]byte, error) {
	switch v := payload.(type) {
	case nil:
		return []byte("[]"), nil
	case []byte:
		return v, nil
	case json.RawMessage:
		return v, nil
	case string:
		// RUN_COMMAND admite tanto un string como un array de strings.
		return json.Marshal(v)
	default:
		return json.Marshal(v)
	}
}
