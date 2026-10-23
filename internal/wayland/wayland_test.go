package wayland

import (
	"encoding/binary"
	"testing"
)

func TestEncoderHeaderAndAlignment(t *testing.T) {
	msg := newEncoder(42, 3).uint32(7).string("hola").int32(-1).bytes()
	if len(msg)%4 != 0 {
		t.Fatalf("mensaje sin alinear: %d bytes", len(msg))
	}
	if got := binary.LittleEndian.Uint32(msg[0:]); got != 42 {
		t.Fatalf("objeto = %d", got)
	}
	if got := binary.LittleEndian.Uint16(msg[4:]); got != 3 {
		t.Fatalf("opcode = %d", got)
	}
	size := int(binary.LittleEndian.Uint16(msg[6:]))
	if size != len(msg) {
		t.Fatalf("tamaño declarado %d, real %d", size, len(msg))
	}
}

func TestDecoderRoundtrip(t *testing.T) {
	enc := newEncoder(9, 1).uint32(11).string("hola").fixed(1.5).int32(-7)
	msg, err := decodeMessage(enc.bytes(), []int{5, 6})
	if err != nil {
		t.Fatal(err)
	}
	if msg.id != 9 || msg.opcode != 1 {
		t.Fatalf("cabecera inesperada: %d/%d", msg.id, msg.opcode)
	}
	if v := msg.args.uint32(); v != 11 {
		t.Fatalf("uint32 = %d", v)
	}
	if s := msg.args.string(); s != "hola" {
		t.Fatalf("string = %q", s)
	}
	if f := msg.args.fixed(); f != 1.5 {
		t.Fatalf("fixed = %f", f)
	}
	if v := msg.args.int32(); v != -7 {
		t.Fatalf("int32 = %d", v)
	}
	if fd := msg.args.fd(); fd != 5 {
		t.Fatalf("fd = %d", fd)
	}
	if fd := msg.args.fd(); fd != 6 {
		t.Fatalf("fd = %d", fd)
	}
	// Sin más descriptores devuelve -1 en vez de entrar en pánico.
	if fd := msg.args.fd(); fd != -1 {
		t.Fatalf("fd extra = %d", fd)
	}
}

func TestDecoderStringPadding(t *testing.T) {
	// "abcd" mide 5 con el NUL y necesita relleno hasta 8.
	e := newEncoder(1, 1).string("abcd").uint32(0xAABBCCDD)
	msg, err := decodeMessage(e.bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := msg.args.string(); s != "abcd" {
		t.Fatalf("string = %q", s)
	}
	if v := msg.args.uint32(); v != 0xAABBCCDD {
		t.Fatalf("el relleno rompió la alineación: %x", v)
	}
	// String vacío (nulo en el protocolo) se lee como cadena vacía.
	e2 := newEncoder(1, 1).string("")
	msg2, err := decodeMessage(e2.bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := msg2.args.string(); s != "" {
		t.Fatalf("string nulo = %q", s)
	}
}

func TestDecodeShortMessage(t *testing.T) {
	if _, err := decodeMessage([]byte{1, 2, 3}, nil); err == nil {
		t.Fatal("se esperaba error con una cabecera incompleta")
	}
}

func TestDetectHelpers(t *testing.T) {
	// Solo comprueban que no entran en pánico sin servidor.
	_ = Detect()
	_ = Available()
	_ = EnvDisplay()
}
