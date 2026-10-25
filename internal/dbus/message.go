// Package dbus implementa el protocolo de mensajes de D-Bus y un cliente y
// servidor mínimos sobre el autobús de sesión, sin dependencias externas.
//
// Cubre lo necesario para hablar con org.freedesktop.Notifications: el
// handshake SASL EXTERNAL, el formato binario de los mensajes (con firmas),
// llamadas a método, respuestas, errores y señales.
//
// Los mensajes se codifican en little endian y se decodifican según el byte de
// orden indicado por el emisor, como exige el protocolo D-Bus.
//
// Formato de un mensaje:
//
//	byte  'l' | byte tipo | byte flags | byte versión (1)
//	uint32 longitud del cuerpo
//	uint32 serial
//	uint32 longitud del array de cabeceras
//	cabeceras: a(yv) — código de campo, variante con el valor
//	cuerpo: los argumentos según la firma del mensaje
package dbus

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Tipo de mensaje D-Bus.
type MsgType byte

const (
	MethodCall   MsgType = 1
	MethodReturn MsgType = 2
	ErrorReply   MsgType = 3
	Signal       MsgType = 4
)

func (t MsgType) String() string {
	switch t {
	case MethodCall:
		return "METHOD_CALL"
	case MethodReturn:
		return "METHOD_RETURN"
	case ErrorReply:
		return "ERROR"
	case Signal:
		return "SIGNAL"
	}
	return fmt.Sprintf("tipo %d", byte(t))
}

// Códigos de los campos de cabecera.
const (
	fieldPath        = 1
	fieldInterface   = 2
	fieldMember      = 3
	fieldErrorName   = 4
	fieldReplySerial = 5
	fieldDestination = 6
	fieldSender      = 7
	fieldSignature   = 8
	fieldUnixFDs     = 9

	maxMessageSize = 128 << 20
)

// Variant es un valor con su firma, como los `v` de D-Bus.
type Variant struct {
	Sig   string
	Value any
}

// DictEntry es una entrada de diccionario.
type DictEntry struct {
	Key   any
	Value any
}

// Dict es un diccionario conservando el orden de llegada.
type Dict []DictEntry

// Get devuelve el valor de una clave o nil.
func (d Dict) Get(key any) any {
	for _, e := range d {
		if fmt.Sprint(e.Key) == fmt.Sprint(key) {
			return e.Value
		}
	}
	return nil
}

// Value devuelve el valor de una clave desenvolviendo la variante.
func (d Dict) Value(key any) any {
	return unwrap(d.Get(key))
}

// Has indica si una clave está presente.
func (d Dict) Has(key any) bool { return d.Get(key) != nil }

// String devuelve el valor de una clave como cadena.
func (d Dict) String(key string) string {
	s, _ := unwrap(d.Get(key)).(string)
	return s
}

// unwrap quita la capa de variante de un valor.
func unwrap(v any) any {
	for {
		va, ok := v.(Variant)
		if !ok {
			return v
		}
		v = va.Value
	}
}

// Message es un mensaje de D-Bus ya decodificado.
type Message struct {
	Type        MsgType
	Flags       byte
	Serial      uint32
	Path        string
	Interface   string
	Member      string
	ErrorName   string
	ReplySerial uint32
	Destination string
	Sender      string
	Signature   string
	Body        []any
}

// NewCall construye una llamada a método.
func NewCall(dest, path, iface, member, sig string, args ...any) *Message {
	return &Message{
		Type: MethodCall, Destination: dest, Path: path, Interface: iface,
		Member: member, Signature: sig, Body: args,
	}
}

// NewSignal construye una señal.
func NewSignal(path, iface, member, sig string, args ...any) *Message {
	return &Message{Type: Signal, Path: path, Interface: iface, Member: member, Signature: sig, Body: args}
}

// ─── Firma ──────────────────────────────────────────────────────────────────

// alignOf devuelve la alineación de un tipo.
func alignOf(c byte) int {
	switch c {
	case 'y', 'g', 'v':
		return 1
	case 'n', 'q':
		return 2
	case 'b', 'i', 'u', 'h', 's', 'o', 'a':
		return 4
	case 'x', 't', 'd', '(', '{':
		return 8
	}
	return 1
}

func isBasic(c byte) bool {
	return c == 'y' || c == 'b' || c == 'n' || c == 'q' || c == 'i' ||
		c == 'u' || c == 'x' || c == 't' || c == 'd' || c == 'h' ||
		c == 's' || c == 'o' || c == 'g'
}

// sigLen devuelve la longitud en bytes de un tipo completo dentro de una firma.
func sigLen(sig string) int {
	if sig == "" {
		return 0
	}
	switch sig[0] {
	case 'a':
		return 1 + sigLen(sig[1:])
	case '(', '{':
		open := sig[0]
		close := byte(')')
		if open == '{' {
			close = '}'
		}
		depth := 0
		for i := 0; i < len(sig); i++ {
			switch sig[i] {
			case open:
				depth++
			case close:
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
		return len(sig)
	}
	return 1
}

// splitSig separa el primer tipo completo del resto de la firma.
func splitSig(sig string) (string, string) {
	n := sigLen(sig)
	if n > len(sig) {
		n = len(sig)
	}
	return sig[:n], sig[n:]
}

// validSignature comprueba que la firma contiene solo tipos completos.
func validSignature(sig string) bool {
	for sig != "" {
		n := sigLen(sig)
		if n <= 0 || n > len(sig) {
			return false
		}
		if !isBasic(sig[0]) && !strings.ContainsRune("av({", rune(sig[0])) {
			return false
		}
		sig = sig[n:]
	}
	return true
}

// ─── Codificación ───────────────────────────────────────────────────────────

type writer struct{ b []byte }

func (w *writer) pad(n int) {
	for len(w.b)%n != 0 {
		w.b = append(w.b, 0)
	}
}

func (w *writer) u32(v uint32) {
	w.pad(4)
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	w.b = append(w.b, tmp[:]...)
}

func (w *writer) u64(v uint64) {
	w.pad(8)
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], v)
	w.b = append(w.b, tmp[:]...)
}

// str escribe una cadena: longitud + bytes + NUL, sin el relleno final (el
// siguiente valor lo añade con su alineación).
func (w *writer) str(s string) {
	w.u32(uint32(len(s)))
	w.b = append(w.b, s...)
	w.b = append(w.b, 0)
}

func (w *writer) signature(s string) {
	w.b = append(w.b, byte(len(s)))
	w.b = append(w.b, s...)
	w.b = append(w.b, 0)
}

func toUint(v any) (uint64, bool) {
	switch n := v.(type) {
	case uint8:
		return uint64(n), true
	case int8:
		return uint64(n), true
	case uint16:
		return uint64(n), true
	case int16:
		return uint64(n), true
	case uint32:
		return uint64(n), true
	case int32:
		return uint64(n), true
	case int:
		return uint64(n), true
	case uint64:
		return n, true
	case int64:
		return uint64(n), true
	case uint:
		return uint64(n), true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	}
	if u, ok := toUint(v); ok {
		return float64(u), true
	}
	return 0, false
}

func (w *writer) value(sig string, v any) error {
	if sig == "" {
		return nil
	}
	switch sig[0] {
	case 'y', 'b', 'n', 'q', 'i', 'u', 'x', 't', 'h':
		u, ok := toUint(v)
		if !ok {
			return fmt.Errorf("D-Bus: se esperaba un número para %q, llegó %T", sig, v)
		}
		switch sig[0] {
		case 'y':
			w.pad(1)
			w.b = append(w.b, byte(u))
		case 'b':
			w.u32(uint32(u & 1))
		case 'n', 'q':
			w.pad(2)
			var tmp [2]byte
			binary.LittleEndian.PutUint16(tmp[:], uint16(u))
			w.b = append(w.b, tmp[:]...)
		case 'i', 'u', 'h':
			w.u32(uint32(u))
		default:
			w.u64(u)
		}
	case 'd':
		f, ok := toFloat(v)
		if !ok {
			return fmt.Errorf("D-Bus: se esperaba un número para %q, llegó %T", sig, v)
		}
		w.u64(math.Float64bits(f))
	case 's', 'o':
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("D-Bus: se esperaba una cadena para %q, llegó %T", sig, v)
		}
		w.str(s)
	case 'g':
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("D-Bus: se esperaba una firma, llegó %T", sig)
		}
		w.signature(s)
	case 'v':
		va, ok := v.(Variant)
		if !ok {
			return fmt.Errorf("D-Bus: se esperaba dbus.Variant, llegó %T", sig)
		}
		w.pad(1)
		w.signature(va.Sig)
		return w.value(va.Sig, va.Value)
	case 'a':
		return w.array(sig, v)
	case '(', '{':
		w.pad(8)
		return w.structure(sig, v)
	default:
		return fmt.Errorf("D-Bus: tipo no soportado %q", sig)
	}
	return nil
}

func (w *writer) array(sig string, v any) error {
	elem, _ := splitSig(sig[1:])
	if elem == "" {
		return fmt.Errorf("D-Bus: array sin tipo de elemento en %q", sig)
	}
	w.pad(4)
	lenPos := len(w.b)
	w.b = append(w.b, 0, 0, 0, 0)
	w.pad(alignOf(elem[0]))
	start := len(w.b)

	items, err := arrayItems(v)
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := w.value(elem, it); err != nil {
			return err
		}
	}
	binary.LittleEndian.PutUint32(w.b[lenPos:], uint32(len(w.b)-start))
	return nil
}

// arrayItems normaliza los valores que se pueden codificar como array.
func arrayItems(v any) ([]any, error) {
	switch it := v.(type) {
	case nil:
		return nil, nil
	case []any:
		return it, nil
	case []string:
		out := make([]any, len(it))
		for i, s := range it {
			out[i] = s
		}
		return out, nil
	case []uint32:
		out := make([]any, len(it))
		for i, n := range it {
			out[i] = n
		}
		return out, nil
	case []int:
		out := make([]any, len(it))
		for i, n := range it {
			out[i] = int32(n)
		}
		return out, nil
	case []byte:
		out := make([]any, len(it))
		for i, b := range it {
			out[i] = b
		}
		return out, nil
	case Dict:
		out := make([]any, len(it))
		for i, e := range it {
			out[i] = e
		}
		return out, nil
	case map[string]any:
		keys := make([]string, 0, len(it))
		for k := range it {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]any, 0, len(it))
		for _, k := range keys {
			out = append(out, DictEntry{Key: k, Value: it[k]})
		}
		return out, nil
	}
	return nil, fmt.Errorf("D-Bus: no se puede codificar %T como array", v)
}

func (w *writer) structure(sig string, v any) error {
	inner := sig[1 : len(sig)-1]
	var fields []any
	switch it := v.(type) {
	case []any:
		fields = it
	case DictEntry:
		fields = []any{it.Key, it.Value}
	case []DictEntry:
		if len(it) > 0 {
			fields = []any{it[0].Key, it[0].Value}
		}
	case string:
		fields = []any{it}
	default:
		return fmt.Errorf("D-Bus: no se puede codificar %T como estructura", v)
	}
	i := 0
	for inner != "" {
		var one string
		one, inner = splitSig(inner)
		var val any
		if i < len(fields) {
			val = fields[i]
		}
		if err := w.value(one, val); err != nil {
			return err
		}
		i++
	}
	return nil
}

// Encode serializa el mensaje completo con su cabecera.
func (m *Message) Encode(serial uint32) ([]byte, error) {
	if m.Serial != 0 {
		serial = m.Serial
	}
	body := &writer{}
	if m.Signature != "" {
		if !validSignature(m.Signature) {
			return nil, fmt.Errorf("D-Bus: firma inválida %q", m.Signature)
		}
		rest := m.Signature
		i := 0
		for rest != "" {
			var one string
			one, rest = splitSig(rest)
			var val any
			if i < len(m.Body) {
				val = m.Body[i]
			}
			if err := body.value(one, val); err != nil {
				return nil, err
			}
			i++
		}
	}

	fields := &writer{}
	add := func(code byte, sig string, val any) error {
		fields.pad(8)
		fields.b = append(fields.b, code)
		fields.signature(sig)
		return fields.value(sig, val)
	}
	type field struct {
		code byte
		sig  string
		val  any
	}
	list := []field{}
	if m.Path != "" {
		list = append(list, field{fieldPath, "o", m.Path})
	}
	if m.Interface != "" {
		list = append(list, field{fieldInterface, "s", m.Interface})
	}
	if m.Member != "" {
		list = append(list, field{fieldMember, "s", m.Member})
	}
	if m.ErrorName != "" {
		list = append(list, field{fieldErrorName, "s", m.ErrorName})
	}
	if m.ReplySerial != 0 {
		list = append(list, field{fieldReplySerial, "u", m.ReplySerial})
	}
	if m.Destination != "" {
		list = append(list, field{fieldDestination, "s", m.Destination})
	}
	if m.Sender != "" {
		list = append(list, field{fieldSender, "s", m.Sender})
	}
	if m.Signature != "" {
		list = append(list, field{fieldSignature, "g", m.Signature})
	}
	for _, f := range list {
		if err := add(f.code, f.sig, f.val); err != nil {
			return nil, err
		}
	}

	out := make([]byte, 0, 16+len(fields.b)+len(body.b)+8)
	out = append(out, 'l', byte(m.Type), m.Flags, 1)
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], uint32(len(body.b)))
	out = append(out, tmp[:]...)
	binary.LittleEndian.PutUint32(tmp[:], serial)
	out = append(out, tmp[:]...)
	binary.LittleEndian.PutUint32(tmp[:], uint32(len(fields.b)))
	out = append(out, tmp[:]...)
	out = append(out, fields.b...)
	// El cuerpo empieza en el siguiente múltiplo de 8; ese relleno forma
	// parte de la cabecera. D-Bus no añade relleno después del cuerpo: el
	// siguiente mensaje del flujo empieza inmediatamente al terminarlo.
	for len(out)%8 != 0 {
		out = append(out, 0)
	}
	out = append(out, body.b...)
	return out, nil
}

// ─── Decodificación ─────────────────────────────────────────────────────────

type reader struct {
	b     []byte
	pos   int
	order binary.ByteOrder
}

func (r *reader) byteOrder() binary.ByteOrder {
	if r.order != nil {
		return r.order
	}
	return binary.LittleEndian
}

func (r *reader) pad(n int) {
	for r.pos%n != 0 {
		r.pos++
	}
}

func (r *reader) need(n int) error {
	if r.pos+n > len(r.b) {
		return fmt.Errorf("D-Bus: mensaje truncado (faltan %d bytes)", r.pos+n-len(r.b))
	}
	return nil
}

func (r *reader) u32() (uint32, error) {
	r.pad(4)
	if err := r.need(4); err != nil {
		return 0, err
	}
	v := r.byteOrder().Uint32(r.b[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *reader) u64() (uint64, error) {
	r.pad(8)
	if err := r.need(8); err != nil {
		return 0, err
	}
	v := r.byteOrder().Uint64(r.b[r.pos:])
	r.pos += 8
	return v, nil
}

func (r *reader) str() (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	if err := r.need(int(n) + 1); err != nil {
		return "", err
	}
	s := string(r.b[r.pos : r.pos+int(n)])
	r.pos += int(n) + 1
	return s, nil
}

func (r *reader) signature() (string, error) {
	if err := r.need(1); err != nil {
		return "", err
	}
	n := int(r.b[r.pos])
	r.pos++
	if err := r.need(n + 1); err != nil {
		return "", err
	}
	s := string(r.b[r.pos : r.pos+n])
	r.pos += n + 1
	return s, nil
}

func (r *reader) value(sig string) (any, error) {
	if sig == "" {
		return nil, nil
	}
	switch sig[0] {
	case 'y':
		if err := r.need(1); err != nil {
			return nil, err
		}
		v := r.b[r.pos]
		r.pos++
		return v, nil
	case 'b':
		v, err := r.u32()
		return v != 0, err
	case 'n':
		r.pad(2)
		if err := r.need(2); err != nil {
			return nil, err
		}
		v := int16(r.byteOrder().Uint16(r.b[r.pos:]))
		r.pos += 2
		return v, nil
	case 'q':
		r.pad(2)
		if err := r.need(2); err != nil {
			return nil, err
		}
		v := r.byteOrder().Uint16(r.b[r.pos:])
		r.pos += 2
		return v, nil
	case 'i', 'h':
		v, err := r.u32()
		return int32(v), err
	case 'u':
		return r.u32()
	case 'x', 't':
		v, err := r.u64()
		if sig[0] == 'x' {
			return int64(v), err
		}
		return v, err
	case 'd':
		v, err := r.u64()
		return math.Float64frombits(v), err
	case 's', 'o':
		return r.str()
	case 'g':
		return r.signature()
	case 'v':
		s, err := r.signature()
		if err != nil {
			return nil, err
		}
		val, err := r.value(s)
		if err != nil {
			return nil, err
		}
		return Variant{Sig: s, Value: val}, nil
	case 'a':
		return r.array(sig)
	case '(', '{':
		return r.structure(sig)
	}
	return nil, fmt.Errorf("D-Bus: tipo no soportado %q", sig)
}

func (r *reader) array(sig string) (any, error) {
	elem, _ := splitSig(sig[1:])
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	r.pad(alignOf(elem[0]))
	start := r.pos
	end := start + int(n)
	if end > len(r.b) {
		return nil, fmt.Errorf("D-Bus: array fuera de límites")
	}
	if elem[0] == 'y' {
		out := make([]byte, end-start)
		copy(out, r.b[start:end])
		r.pos = end
		return out, nil
	}
	var out []any
	for r.pos < end {
		before := r.pos
		v, err := r.value(elem)
		if err != nil {
			return nil, err
		}
		if r.pos == before {
			return nil, fmt.Errorf("D-Bus: array sin avance")
		}
		out = append(out, v)
	}
	if elem[0] == '{' {
		d := make(Dict, 0, len(out))
		for _, it := range out {
			if e, ok := it.(DictEntry); ok {
				d = append(d, e)
			}
		}
		return d, nil
	}
	return out, nil
}

func (r *reader) structure(sig string) (any, error) {
	r.pad(8)
	inner := sig[1 : len(sig)-1]
	if sig[0] == '{' {
		keySig, rest := splitSig(inner)
		valSig, _ := splitSig(rest)
		k, err := r.value(keySig)
		if err != nil {
			return nil, err
		}
		v, err := r.value(valSig)
		if err != nil {
			return nil, err
		}
		return DictEntry{Key: k, Value: v}, nil
	}
	var out []any
	for inner != "" {
		var one string
		one, inner = splitSig(inner)
		v, err := r.value(one)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// DecodeMessage decodifica un mensaje completo, sin suponer relleno tras el
// cuerpo: en un flujo D-Bus el siguiente mensaje empieza inmediatamente allí.
func DecodeMessage(data []byte) (*Message, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("D-Bus: mensaje demasiado corto (%d bytes)", len(data))
	}
	var order binary.ByteOrder
	switch data[0] {
	case 'l':
		order = binary.LittleEndian
	case 'B':
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("D-Bus: orden de bytes desconocido %q", data[0])
	}
	if data[3] != 1 {
		return nil, fmt.Errorf("D-Bus: versión de protocolo %d no soportada", data[3])
	}
	bodyLen := uint64(order.Uint32(data[4:8]))
	m := &Message{Type: MsgType(data[1]), Flags: data[2], Serial: order.Uint32(data[8:12])}
	fieldsLen := uint64(order.Uint32(data[12:16]))
	if bodyLen+fieldsLen+16 > maxMessageSize {
		return nil, fmt.Errorf("D-Bus: mensaje demasiado grande (cuerpo %d, cabeceras %d)", bodyLen, fieldsLen)
	}

	fieldsEnd := 16 + int(fieldsLen)
	bodyStart := align8(fieldsEnd)
	total := bodyStart + int(bodyLen)
	if total > maxMessageSize {
		return nil, fmt.Errorf("D-Bus: mensaje demasiado grande (%d bytes)", total)
	}
	if fieldsEnd > len(data) || total > len(data) {
		return nil, fmt.Errorf("D-Bus: mensaje truncado (se necesitan %d bytes, hay %d)", total, len(data))
	}

	fr := &reader{b: data[16:fieldsEnd], order: order}
	for fr.pos < len(fr.b) {
		fr.pad(8)
		if fr.pos >= len(fr.b) {
			break
		}
		code := fr.b[fr.pos]
		fr.pos++
		val, err := fr.value("v")
		if err != nil {
			return nil, err
		}
		va, _ := val.(Variant)
		switch code {
		case fieldPath:
			m.Path, _ = va.Value.(string)
		case fieldInterface:
			m.Interface, _ = va.Value.(string)
		case fieldMember:
			m.Member, _ = va.Value.(string)
		case fieldErrorName:
			m.ErrorName, _ = va.Value.(string)
		case fieldReplySerial:
			if n, ok := toUint(va.Value); ok {
				m.ReplySerial = uint32(n)
			}
		case fieldDestination:
			m.Destination, _ = va.Value.(string)
		case fieldSender:
			m.Sender, _ = va.Value.(string)
		case fieldSignature:
			m.Signature, _ = va.Value.(string)
		}
	}

	if bodyLen > 0 {
		br := &reader{b: data[bodyStart:total], order: order}
		rest := m.Signature
		for rest != "" {
			var one string
			one, rest = splitSig(rest)
			v, err := br.value(one)
			if err != nil {
				return nil, err
			}
			m.Body = append(m.Body, v)
		}
	}
	return m, nil
}
