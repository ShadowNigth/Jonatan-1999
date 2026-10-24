// Package osd dibuja los avisos en pantalla (OSD) del escritorio: tarjetas
// flotantes para el volumen y los metadatos de reproducción MPRIS.
//
// Solo se dibuja lo que hace falta, con la misma estética que la barra y las
// notificaciones: isla redondeada, paleta activa y fuente de mapa de bits.
package osd

import (
	"fmt"
	"strings"
)

// Kind es el tipo de aviso en pantalla.
type Kind string

const (
	// KindVolume son los avisos de volumen del altavoz por defecto.
	KindVolume Kind = "volume"
)

// MaxValue es el tope con el que se dibuja la barra (por encima de 100% el
// volumen se amplifica: PipeWire lo permite).
const MaxValue = 150

// State es lo que se muestra en el OSD.
type State struct {
	Kind   Kind
	Value  int    // porcentaje, puede pasar de 100
	Muted  bool   // silenciado
	Device string // metadato de salida; no se muestra en la tarjeta compacta
	Sink   string // nombre técnico (para depurar)
	// Step es el último cambio (+5, -5, 0 si no se sabe); se usa para el matiz
	// del color al subir o bajar.
	Step int
}

// Volume crea el estado de un OSD de volumen.
func Volume(value int, muted bool, device string) State {
	return State{Kind: KindVolume, Value: value, Muted: muted, Device: device}
}

// Percent devuelve el valor acotado entre 0 y MaxValue.
func (s State) Percent() int {
	if s.Value < 0 {
		return 0
	}
	if s.Value > MaxValue {
		return MaxValue
	}
	return s.Value
}

// Filled devuelve la fracción (0..1) que debe pintarse en la barra. El 100%
// llena la barra: por encima de eso el volumen está amplificado y se marca con
// otro color, no con una barra desbordada.
func (s State) Filled() float64 {
	if s.Muted {
		return 0
	}
	v := s.Percent()
	if v > 100 {
		v = 100
	}
	return float64(v) / 100
}

// Overdrive indica si el volumen está amplificado por encima del 100%.
func (s State) Overdrive() bool { return s.Value > 100 }

// Label es el título del aviso.
func (s State) Label() string {
	switch s.Kind {
	case KindVolume:
		return "volumen"
	default:
		return string(s.Kind)
	}
}

// ValueText es el texto grande: el porcentaje o el estado de silencio.
func (s State) ValueText() string {
	if s.Muted {
		return "silenciado"
	}
	return fmt.Sprintf("%d%%", s.Value)
}

// Detail devuelve el dispositivo asociado, si se conoce; el renderizador
// compacto no lo dibuja.
func (s State) Detail() string {
	d := strings.TrimSpace(s.Device)
	if d == "" {
		return ""
	}
	return d
}

// Key resume el contenido para no repintar de más.
func (s State) Key() string {
	return fmt.Sprintf("%s|%d|%v|%s|%d", s.Kind, s.Value, s.Muted, s.Device, s.Step)
}
