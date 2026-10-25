package dbus

import (
	"bufio"
	"encoding/binary"
	"io"
	"testing"
)

// readMessage lee un mensaje completo de un socket (para el bus de pruebas).
func readMessage(t *testing.T, br *bufio.Reader) []byte {
	t.Helper()
	header := make([]byte, 16)
	if _, err := io.ReadFull(br, header); err != nil {
		t.Fatalf("leyendo cabecera: %v", err)
	}
	bodyLen := int(binary.LittleEndian.Uint32(header[4:]))
	fieldsLen := int(binary.LittleEndian.Uint32(header[12:]))
	total := align8(16+fieldsLen) + bodyLen
	msg := make([]byte, total)
	copy(msg, header)
	if _, err := io.ReadFull(br, msg[16:]); err != nil {
		t.Fatalf("leyendo cuerpo: %v", err)
	}
	return msg
}

func TestSignatureSplitting(t *testing.T) {
	cases := []struct{ in, one, rest string }{
		{"a{sv}", "a{sv}", ""},
		{"sa{sv}as", "s", "a{sv}as"},
		{"(uus)", "(uus)", ""},
		{"a(us)u", "a(us)", "u"},
		{"v", "v", ""},
	}
	for _, c := range cases {
		one, rest := splitSig(c.in)
		if one != c.one || rest != c.rest {
			t.Errorf("splitSig(%q) = %q, %q; esperado %q, %q", c.in, one, rest, c.one, c.rest)
		}
	}
}

func TestMessageRoundTrip(t *testing.T) {
	m := &Message{
		Type: Signal, Path: "/org/freedesktop/Notifications",
		Interface: "org.freedesktop.Notifications", Member: "ActionInvoked",
		Destination: ":1.5", Signature: "us", Body: []any{uint32(7), "mini"},
	}
	data, err := m.Encode(1)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	bodyLen := int(binary.LittleEndian.Uint32(data[4:8]))
	fieldsLen := int(binary.LittleEndian.Uint32(data[12:16]))
	wantLen := align8(16+fieldsLen) + bodyLen
	if len(data) != wantLen {
		t.Fatalf("longitud D-Bus = %d, esperada %d (sin relleno tras el cuerpo)", len(data), wantLen)
	}
	if len(data)%8 == 0 {
		t.Fatal("este caso debe terminar sin alineación a 8 bytes tras el cuerpo")
	}
	if data[0] != 'l' || data[1] != byte(Signal) || data[3] != 1 {
		t.Fatalf("cabecera inesperada: % x", data[:4])
	}
	got, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Path != m.Path || got.Member != m.Member || got.Interface != m.Interface {
		t.Fatalf("cabeceras mal: %+v", got)
	}
	if got.Destination != ":1.5" || got.Signature != "us" {
		t.Fatalf("destino o firma mal: %+v", got)
	}
	if len(got.Body) != 2 {
		t.Fatalf("cuerpo: %#v", got.Body)
	}
	if n, ok := toUint(got.Body[0]); !ok || n != 7 {
		t.Fatalf("primer argumento: %#v", got.Body[0])
	}
	if s, _ := got.Body[1].(string); s != "mini" {
		t.Fatalf("segundo argumento: %#v", got.Body[1])
	}
}

func TestDecodeBigEndianMessage(t *testing.T) {
	// Una señal mínima: cabecera con firma "u" y argumento uint32(42), toda
	// codificada en big endian según el primer byte del mensaje.
	data := []byte{
		'B', byte(Signal), 0, 1,
		0, 0, 0, 4, // longitud del cuerpo
		0, 0, 0, 1, // serial
		0, 0, 0, 7, // longitud de campos de cabecera
		fieldSignature, 1, 'g', 0, 1, 'u', 0, 0, // campos + relleno
		0, 0, 0, 42, // cuerpo
	}
	got, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("decode big endian: %v", err)
	}
	if got.Signature != "u" || len(got.Body) != 1 || got.Body[0] != uint32(42) {
		t.Fatalf("mensaje decodificado: %+v", got)
	}
}

func TestNotifyCallWithHints(t *testing.T) {
	hints := Dict{
		{Key: "urgency", Value: Variant{Sig: "y", Value: uint8(2)}},
		{Key: "transient", Value: Variant{Sig: "b", Value: true}},
		{Key: "category", Value: Variant{Sig: "s", Value: "device"}},
		{Key: "progress", Value: Variant{Sig: "i", Value: int32(42)}},
	}
	m := NewCall("org.freedesktop.Notifications", "/org/freedesktop/Notifications",
		"org.freedesktop.Notifications", "Notify", "susssasa{sv}i",
		"foot", uint32(0), "", "Terminado", "La compilación fue bien",
		[]string{"default", "Abrir"}, hints, int32(5000))
	data, err := m.Encode(3)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Body) != 8 {
		t.Fatalf("argumentos: %d (%#v)", len(got.Body), got.Body)
	}
	if got.Body[0] != "foot" {
		t.Errorf("app_name: %#v", got.Body[0])
	}
	if got.Body[3] != "Terminado" || got.Body[4] != "La compilación fue bien" {
		t.Errorf("textos: %#v", got.Body[3:5])
	}
	actions, _ := got.Body[5].([]any)
	if len(actions) != 2 || actions[0] != "default" {
		t.Errorf("actions: %#v", got.Body[5])
	}
	h, ok := got.Body[6].(Dict)
	if !ok {
		t.Fatalf("hints no es Dict: %T", got.Body[6])
	}
	if u := h.Get("urgency"); u == nil {
		t.Fatalf("falta urgency en %#v", h)
	} else if va, ok := u.(Variant); !ok || va.Value != uint8(2) {
		t.Errorf("urgency: %#v", u)
	}
	if tr := h.Get("transient"); tr == nil {
		t.Errorf("falta transient: %#v", h)
	}
	// La firma a{sv} se decodifica en orden y conserva el tipo de cada valor.
	if s := h.String("category"); s != "device" {
		t.Errorf("category: %q", s)
	}
	if n, ok := toUint(h.Value("progress")); !ok || int32(n) != 42 {
		t.Errorf("progress: %#v", h.Get("progress"))
	}
}

func TestArrayOfBytes(t *testing.T) {
	m := &Message{Type: MethodReturn, Signature: "ay", Body: []any{[]byte{1, 2, 3, 250}}}
	data, err := m.Encode(9)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	b, ok := got.Body[0].([]byte)
	if !ok || len(b) != 4 || b[3] != 250 {
		t.Fatalf("bytes: %#v", got.Body[0])
	}
}

func TestStructArray(t *testing.T) {
	m := &Message{
		Type: MethodReturn, Signature: "a(ss)",
		Body: []any{[]any{[]any{"a", "b"}, []any{"c", "d"}}},
	}
	data, err := m.Encode(1)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	arr, _ := got.Body[0].([]any)
	if len(arr) != 2 {
		t.Fatalf("array: %#v", got.Body[0])
	}
	first, _ := arr[0].([]any)
	if len(first) != 2 || first[0] != "a" || first[1] != "b" {
		t.Fatalf("estructura: %#v", arr[0])
	}
}
