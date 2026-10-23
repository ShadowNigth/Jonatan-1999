// Package wayland implementa un cliente mínimo del protocolo Wayland en Go
// puro, suficiente para publicar una superficie de fondo animada en la capa
// "background" mediante wlr-layer-shell.
//
// Se implementa el transporte (socket UNIX con paso de descriptores por
// SCM_RIGHTS), el formato binario de los mensajes, el registro de objetos y los
// objetos necesarios: wl_compositor, wl_shm, wl_shm_pool, wl_buffer,
// wl_surface, zwlr_layer_shell_v1 y zwlr_layer_surface_v1.
package wayland

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Wire format de Wayland:
//
//	cabecera: uint32 id del objeto | uint16 opcode | uint16 tamaño total
//	argumentos según la firma del mensaje (enteros, strings, new_id, fds)
//
// Los strings van como longitud (incluyendo el NUL) + bytes + NUL, alineados a 4.
// Los descriptores de archivo no forman parte del cuerpo: viajan como datos
// auxiliares SCM_RIGHTS y se consumen en el orden en que aparecen.

type encoder struct {
	buf []byte
}

func newEncoder(id uint32, opcode uint16) *encoder {
	e := &encoder{buf: make([]byte, 8)}
	binary.LittleEndian.PutUint32(e.buf[0:], id)
	binary.LittleEndian.PutUint16(e.buf[4:], opcode)
	return e
}

func (e *encoder) uint32(v uint32) *encoder {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	e.buf = append(e.buf, b[:]...)
	return e
}

func (e *encoder) int32(v int32) *encoder { return e.uint32(uint32(v)) }

func (e *encoder) string(s string) *encoder {
	if s == "" {
		// Un string nulo se codifica con longitud 0.
		return e.uint32(0)
	}
	n := len(s) + 1
	e.uint32(uint32(n))
	e.buf = append(e.buf, s...)
	e.buf = append(e.buf, 0)
	for len(e.buf)%4 != 0 {
		e.buf = append(e.buf, 0)
	}
	return e
}

// newID añade un identificador de objeto nuevo.
func (e *encoder) newID(id uint32) *encoder { return e.uint32(id) }

// fixed codifica un número de coma fija 24.8.
func (e *encoder) fixed(v float64) *encoder {
	return e.int32(int32(math.Round(v * 256)))
}

func (e *encoder) bytes() []byte {
	binary.LittleEndian.PutUint16(e.buf[6:], uint16(len(e.buf)))
	// Los mensajes siempre se alinean a 4 bytes.
	for len(e.buf)%4 != 0 {
		e.buf = append(e.buf, 0)
	}
	binary.LittleEndian.PutUint16(e.buf[6:], uint16(len(e.buf)))
	return e.buf
}

// decoder recorre los argumentos de un evento.
type decoder struct {
	buf   []byte
	pos   int
	fds   []int
	fdPos int
}

func (d *decoder) uint32() uint32 {
	if d.pos+4 > len(d.buf) {
		return 0
	}
	v := binary.LittleEndian.Uint32(d.buf[d.pos:])
	d.pos += 4
	return v
}

func (d *decoder) int32() int32 { return int32(d.uint32()) }

func (d *decoder) fixed() float64 { return float64(d.int32()) / 256 }

func (d *decoder) string() string {
	n := d.uint32()
	if n == 0 {
		return ""
	}
	if d.pos+int(n) > len(d.buf) {
		return ""
	}
	s := string(d.buf[d.pos : d.pos+int(n)-1])
	d.pos += int(n)
	// Alinear a 4.
	for d.pos%4 != 0 && d.pos < len(d.buf) {
		d.pos++
	}
	return s
}

// fd devuelve el siguiente descriptor recibido en los datos auxiliares.
func (d *decoder) fd() int {
	if d.fdPos >= len(d.fds) {
		return -1
	}
	fd := d.fds[d.fdPos]
	d.fdPos++
	return fd
}

// message es un mensaje recibido.
type message struct {
	id     uint32
	opcode uint16
	args   decoder
}

func decodeMessage(data []byte, fds []int) (message, error) {
	if len(data) < 8 {
		return message{}, fmt.Errorf("mensaje Wayland truncado")
	}
	size := binary.LittleEndian.Uint16(data[6:])
	if int(size) > len(data) || size < 8 {
		return message{}, fmt.Errorf("tamaño de mensaje inválido: %d", size)
	}
	return message{
		id:     binary.LittleEndian.Uint32(data[0:]),
		opcode: binary.LittleEndian.Uint16(data[4:]),
		args:   decoder{buf: data[8:size], fds: fds},
	}, nil
}

// opcodes de los objetos que usamos (según los XML de wayland y wlr-layer-shell).

// wl_display
const (
	displaySync       = 0
	displayGetRegist  = 1
	displayErrorEvent = 0
	displayDeleteID   = 1
)

// wl_registry
const (
	registryBind     = 0
	registryGlobal   = 0
	registryGlobalRm = 1
)

// wl_compositor
const compositorCreateSurface = 0
const compositorCreateRegion = 1

// wl_surface
const (
	surfaceDestroy    = 0
	surfaceAttach     = 1
	surfaceDamage     = 2
	surfaceFrame      = 3
	surfaceSetOpaque  = 4
	surfaceCommit     = 6
	surfaceDamageBuf  = 9
	surfaceEnterEvent = 0
	surfaceLeaveEvent = 1
)

// wl_region
const (
	regionDestroy  = 0
	regionAdd      = 1
	regionSubtract = 2
)

// wl_shm
const (
	shmCreatePool  = 0
	shmFormatEvent = 0
)

// wl_shm_pool
const (
	poolCreateBuffer = 0
	poolDestroy      = 1
	poolResize       = 2
)

// wl_buffer
const (
	bufferDestroy    = 0
	bufferReleaseEvt = 0
)

// wl_callback
const callbackDoneEvent = 0

// zwlr_layer_shell_v1
const (
	layerShellGetSurface = 0
	layerShellDestroy    = 1
)

// zwlr_layer_surface_v1
const (
	layerSurfaceSetSize          = 0
	layerSurfaceSetAnchor        = 1
	layerSurfaceSetExclusiveZone = 2
	layerSurfaceSetMargin        = 3
	layerSurfaceSetKeyboard      = 4
	layerSurfaceGetPopup         = 5
	layerSurfaceAckConfigure     = 6
	layerSurfaceDestroy          = 7
	layerSurfaceSetLayer         = 8
	layerSurfaceSetExclusiveEdge = 9
	layerSurfaceConfigureEvent   = 0
	layerSurfaceClosedEvent      = 1
)

// Capas de wlr-layer-shell.
const (
	LayerBackground = 0
	LayerBottom     = 1
	LayerTop        = 2
	LayerOverlay    = 3
)

// Anclas.
const (
	AnchorTop    = 1
	AnchorBottom = 2
	AnchorLeft   = 4
	AnchorRight  = 8
)

// Formato de píxel ARGB8888 (little-endian: BGRA en memoria).
const formatARGB8888 = 0
