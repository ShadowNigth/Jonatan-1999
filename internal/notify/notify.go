// Package notify implementa el modelo y el dibujado de las notificaciones de
// escritorio: estados, caducidad, modo «no molestar», historial y las tarjetas
// que se pintan en píxeles sobre una superficie de Wayland.
package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Urgency es la prioridad de una notificación (hint `urgency`).
type Urgency int

const (
	Low Urgency = iota
	Normal
	Critical
)

// String devuelve el nombre de la urgencia.
func (u Urgency) String() string {
	switch u {
	case Low:
		return "baja"
	case Critical:
		return "crítica"
	}
	return "normal"
}

// ParseUrgency interpreta el valor del hint `urgency` (0, 1 o 2).
func ParseUrgency(v any) Urgency {
	switch n := v.(type) {
	case uint8:
		return clampUrgency(int(n))
	case int32:
		return clampUrgency(int(n))
	case uint32:
		return clampUrgency(int(n))
	case int:
		return clampUrgency(n)
	case int64:
		return clampUrgency(int(n))
	}
	return Normal
}

func clampUrgency(n int) Urgency {
	switch {
	case n <= 0:
		return Low
	case n >= 2:
		return Critical
	}
	return Normal
}

// Action es un botón de una notificación: la clave que se devuelve en
// ActionInvoked y la etiqueta que se dibuja.
type Action struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Notification es una notificación viva o del historial.
type Notification struct {
	ID        uint32   `json:"id"`
	App       string   `json:"app"`
	Icon      string   `json:"icon,omitempty"`
	Summary   string   `json:"summary"`
	Body      string   `json:"body,omitempty"`
	Actions   []Action `json:"actions,omitempty"`
	Urgency   Urgency  `json:"urgency"`
	Category  string   `json:"category,omitempty"`
	Desktop   string   `json:"desktop_entry,omitempty"`
	Transient bool     `json:"transient,omitempty"`
	Resident  bool     `json:"resident,omitempty"`
	Progress  int      `json:"progress"` // -1 si no hay barra de progreso
	// ReplacesID pide sustituir una notificación existente (0 = ninguna).
	ReplacesID uint32 `json:"-"`

	// InlineReply indica que la notificación admite una respuesta rápida: la
	// aplicación envió una acción con la clave "inline-reply" y espera el
	// texto por la señal NotificationReplied.
	InlineReply bool `json:"inline_reply,omitempty"`
	// ReplyPlaceholder es el texto de ayuda del campo de respuesta (hint
	// x-kde-reply-placeholder-text).
	ReplyPlaceholder string `json:"reply_placeholder,omitempty"`
	// Image es la imagen adjunta (hint `image-data`) ya decodificada. No se
	// guarda en el historial: puede pesar cientos de kilobytes.
	Image *Image `json:"-"`
	// ImagePath es la ruta de la imagen (hint `image-path` o el icono cuando es
	// un archivo), que sí se conserva en el historial para poder recargarla.
	ImagePath string `json:"image_path,omitempty"`
	// GroupID es la etiqueta de pila que manda el cliente (hints
	// x-dunst-stack-tag, x-kde-group-id o group). Con ella se separan grupos
	// dentro de una misma aplicación; vacía, se agrupa por aplicación.
	GroupID string `json:"group,omitempty"`
	// Sender es el nombre único D-Bus del cliente que la envió, para poder
	// entregarle la respuesta solo a él.
	Sender    string    `json:"-"`
	Timeout   int       `json:"timeout_ms"`
	Created   time.Time `json:"created"`
	Expires   time.Time `json:"expires,omitempty"`
	ActionKey string    `json:"action,omitempty"`

	// internal
	closed   bool
	imageRaw any // valor crudo del hint image-data, ya decodificado por D-Bus
}

// Visible indica si la notificación debe dibujarse en este instante.
func (n *Notification) Visible(now time.Time) bool {
	if n.closed {
		return false
	}
	if !n.Expires.IsZero() && !now.Before(n.Expires) {
		return false
	}
	return true
}

// HasImage indica si la notificación trae una imagen para dibujar.
func (n *Notification) HasImage() bool { return !n.Image.Empty() }

// AttachImage decodifica y asigna una imagen desde bytes (PNG o JPEG).
func (n *Notification) AttachImage(data []byte) error {
	im, err := DecodeImage(data)
	if err != nil {
		return err
	}
	n.Image = im
	return nil
}

// Remaining devuelve la fracción de tiempo que le queda (0..1) y si tiene
// caducidad.
func (n *Notification) Remaining(now time.Time) (float64, bool) {
	if n.Expires.IsZero() || n.Timeout <= 0 {
		return 1, false
	}
	total := n.Expires.Sub(n.Created)
	if total <= 0 {
		return 0, true
	}
	left := n.Expires.Sub(now)
	if left <= 0 {
		return 0, true
	}
	if left > total {
		return 1, true
	}
	return left.Seconds() / total.Seconds(), true
}

// Record es una entrada del historial.
type Record struct {
	Notification
	ClosedAt time.Time `json:"closed_at"`
	Reason   string    `json:"reason"`
}

// Options ajusta el comportamiento del almacén.
type Options struct {
	Timeout         time.Duration // predeterminado para urgencia normal o baja
	CriticalTimeout time.Duration // 0 = las críticas no caducan
	MaxVisible      int           // cuántas se dibujan a la vez (o cuántos grupos)
	GroupVisible    bool          // MaxVisible cuenta grupos, no notificaciones
	HistorySize     int           // entradas que se conservan
	DNDBypass       bool          // las críticas se muestran en modo no molestar
	now             func() time.Time
}

// Store guarda las notificaciones vivas y el historial.
type Store struct {
	mu      sync.Mutex
	opts    Options
	items   []*Notification
	history []Record
	nextID  uint32
	dnd     bool
}

// NewStore crea un almacén con valores razonables.
func NewStore(opts Options) *Store {
	if opts.Timeout <= 0 {
		opts.Timeout = 6 * time.Second
	}
	if opts.MaxVisible <= 0 {
		opts.MaxVisible = 4
	}
	if opts.HistorySize <= 0 {
		opts.HistorySize = 50
	}
	if opts.now == nil {
		opts.now = time.Now
	}
	return &Store{opts: opts, nextID: 1, dnd: false}
}

// SetNow permite fijar el reloj (pruebas).
func (s *Store) SetNow(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn == nil {
		fn = time.Now
	}
	s.opts.now = fn
}

// Add incorpora una notificación. Si ReplacesID coincide con una viva, se
// sustituye conservando su hueco. Devuelve una copia con el ID asignado.
func (s *Store) Add(n Notification) Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.opts.now()
	n.Created = now
	n.closed = false
	n.Progress = -1

	if n.Timeout < 0 {
		// -1 o menor: el cliente pide que no caduque nunca.
		n.Expires = time.Time{}
		n.Timeout = 0
	} else {
		d := time.Duration(n.Timeout) * time.Millisecond
		if d == 0 {
			d = s.defaultTimeout(n.Urgency)
		}
		n.Timeout = int(d / time.Millisecond)
		if d > 0 {
			n.Expires = now.Add(d)
		}
	}
	if n.Urgency == Critical && s.opts.CriticalTimeout <= 0 && n.Timeout == 0 {
		n.Expires = time.Time{}
	}

	if n.ReplacesID != 0 {
		for i, old := range s.items {
			if old.ID == n.ReplacesID {
				old.closed = true
				s.recordLocked(*old, "replaced")
				n.ID = old.ID
				s.items[i] = &n
				return n
			}
		}
	}
	n.ID = s.nextID
	s.nextID++
	s.items = append(s.items, &n)
	return n
}

func (s *Store) defaultTimeout(u Urgency) time.Duration {
	if u == Critical {
		if s.opts.CriticalTimeout > 0 {
			return s.opts.CriticalTimeout
		}
		return 0
	}
	return s.opts.Timeout
}

// Visible devuelve las notificaciones que se deben dibujar, en orden.
func (s *Store) Visible() []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.opts.now()
	out := make([]Notification, 0, len(s.items))
	for _, n := range s.items {
		if !n.Visible(now) {
			continue
		}
		if s.dnd && !(s.opts.DNDBypass && n.Urgency == Critical) {
			continue
		}
		out = append(out, *n)
	}
	if s.opts.GroupVisible {
		out = TrimToGroups(out, s.opts.MaxVisible)
	} else if len(out) > s.opts.MaxVisible {
		out = out[len(out)-s.opts.MaxVisible:]
	}
	return out
}

// Pending devuelve cuántas hay vivas (aunque el modo no molestar las oculte).
func (s *Store) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.opts.now()
	n := 0
	for _, it := range s.items {
		if it.Visible(now) {
			n++
		}
	}
	return n
}

// Get devuelve una notificación viva por ID.
func (s *Store) Get(id uint32) (Notification, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.items {
		if n.ID == id {
			return *n, true
		}
	}
	return Notification{}, false
}

// Dismiss cierra una notificación y la apunta en el historial.
func (s *Store) Dismiss(id uint32, reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, n := range s.items {
		if n.ID != id {
			continue
		}
		n.closed = true
		s.recordLocked(*n, reason)
		s.items = append(s.items[:i], s.items[i+1:]...)
		return true
	}
	return false
}

// Expire cierra las caducadas y devuelve sus IDs (para avisar por D-Bus).
func (s *Store) Expire() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.opts.now()
	var ids []uint32
	keep := s.items[:0]
	for _, n := range s.items {
		if !n.closed && !n.Expires.IsZero() && !now.Before(n.Expires) {
			n.closed = true
			s.recordLocked(*n, "expired")
			ids = append(ids, n.ID)
			continue
		}
		keep = append(keep, n)
	}
	s.items = keep
	return ids
}

// Hold detiene la caducidad de una notificación (por ejemplo, mientras el
// usuario escribe una respuesta). Devuelve false si ya no existe.
func (s *Store) Hold(id uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.items {
		if n.ID == id {
			n.Expires = time.Time{}
			n.Timeout = 0
			return true
		}
	}
	return false
}

// Clear cierra todas las visibles y devuelve sus IDs.
func (s *Store) Clear() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]uint32, 0, len(s.items))
	for _, n := range s.items {
		n.closed = true
		s.recordLocked(*n, "closed")
		ids = append(ids, n.ID)
	}
	s.items = nil
	return ids
}

// SetDND activa o desactiva el modo «no molestar».
func (s *Store) SetDND(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dnd = on
}

// DND indica si está activo el modo «no molestar».
func (s *Store) DND() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dnd
}

// ToggleDND cambia el modo y devuelve el estado nuevo.
func (s *Store) ToggleDND() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dnd = !s.dnd
	return s.dnd
}

// recordLocked añade una entrada al historial.
func (s *Store) recordLocked(n Notification, reason string) {
	s.history = append(s.history, Record{Notification: n, ClosedAt: s.opts.now(), Reason: reason})
	if len(s.history) > s.opts.HistorySize {
		s.history = s.history[len(s.history)-s.opts.HistorySize:]
	}
}

// History devuelve el historial, de la más antigua a la más reciente.
func (s *Store) History() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, len(s.history))
	copy(out, s.history)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ClosedAt.Before(out[j].ClosedAt) })
	return out
}

// ClearHistory vacía el historial.
func (s *Store) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
}

// SaveHistory escribe el historial en JSON.
func (s *Store) SaveHistory(path string) error {
	s.mu.Lock()
	data, err := json.MarshalIndent(s.history, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadHistory lee el historial guardado, si existe.
func (s *Store) LoadHistory(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return fmt.Errorf("historial de notificaciones corrupto: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = recs
	// Las imágenes no se guardan en disco (pueden pesar mucho), pero sí su
	// ruta: se recargan para que el historial se vea igual que en su momento.
	for i := range s.history {
		if s.history[i].ImagePath == "" || s.history[i].Image != nil {
			continue
		}
		if im, err := LoadImageFile(s.history[i].ImagePath); err == nil {
			s.history[i].Image = im
		}
	}
	for _, r := range recs {
		if r.ID >= s.nextID {
			s.nextID = r.ID + 1
		}
	}
	return nil
}

// ─── Texto ──────────────────────────────────────────────────────────────────

// StripMarkup convierte el cuerpo con marcado limitado del protocolo
// (body-markup) en texto plano: quita etiquetas y desescapa entidades.
func StripMarkup(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '<' {
			if n := tagLen(s[i:]); n > 0 {
				i += n
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	replacer := strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"",
		"&apos;", "'", "&#39;", "'", "&#40;", "(", "&#41;", ")",
		"&nbsp;", " ",
	)
	return replacer.Replace(b.String())
}

// tagLen devuelve la longitud de una etiqueta reconocida (incluido '>'), o 0.
func tagLen(s string) int {
	if len(s) < 3 || s[0] != '<' {
		return 0
	}
	end := strings.IndexByte(s, '>')
	if end < 0 || end > 256 {
		return 0
	}
	inner := strings.ToLower(strings.TrimSpace(s[1:end]))
	if inner == "" {
		return 0
	}
	name := strings.TrimPrefix(inner, "/")
	name = strings.TrimSuffix(name, "/")
	if i := strings.IndexAny(name, " \t"); i > 0 {
		name = name[:i]
	}
	switch name {
	case "b", "i", "u", "s", "a", "span", "img", "sub", "sup", "small", "big", "tt":
		return end + 1
	}
	return 0
}

// Sanitize prepara un texto de una línea: sin saltos ni espacios repetidos.
func Sanitize(s string, max int) string {
	s = StripMarkup(s)
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if max > 0 && utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = strings.TrimRight(string(r[:max-1]), " ") + "…"
	}
	return s
}

// MaxReplyLen es la longitud máxima de una respuesta rápida.
const MaxReplyLen = 512

// SanitizeReply limpia el texto de una respuesta: una sola línea, sin espacios
// repetidos ni caracteres de control, y con un límite de longitud. Devuelve ""
// si no queda nada útil.
func SanitizeReply(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			// Los caracteres de control se convierten en espacio: así no se
			// pegan dos palabras al limpiar el texto.
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if r := []rune(out); len(r) > MaxReplyLen {
		out = string(r[:MaxReplyLen])
	}
	return out
}

// Shorten recorta a n caracteres añadiendo puntos suspensivos.
func Shorten(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}

// Wrap divide un texto en líneas que caben en maxW píxeles según la métrica
// indicada (ancho de avance por carácter). Respeta los saltos de línea
// explícitos y corta palabras demasiado largas. Como mucho devuelve maxLines
// líneas; la última termina en puntos suspensivos si hubo recorte.
func Wrap(text string, maxW, advance, maxLines int) []string {
	if advance <= 0 {
		advance = 1
	}
	cols := maxW / advance
	if cols < 1 {
		cols = 1
	}
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimRight(para, " \t")
		if strings.TrimSpace(para) == "" {
			if len(lines) > 0 && maxLines > 0 && len(lines) >= maxLines {
				break
			}
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(para)
		cur := ""
		for _, w := range words {
			// Palabras más largas que una línea: se parten.
			for utf8.RuneCountInString(w) > cols {
				if cur != "" {
					lines = append(lines, cur)
					cur = ""
				}
				r := []rune(w)
				lines = append(lines, string(r[:cols]))
				w = string(r[cols:])
			}
			if cur == "" {
				cur = w
				continue
			}
			if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= cols {
				cur += " " + w
			} else {
				lines = append(lines, cur)
				cur = w
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		last := lines[maxLines-1]
		if utf8.RuneCountInString(last) > cols-1 {
			last = string([]rune(last)[:cols-1])
		}
		lines[maxLines-1] = strings.TrimRight(last, " ") + "…"
	}
	return lines
}
