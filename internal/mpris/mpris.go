// Package mpris vigila reproductores MPRIS del autobús D-Bus de sesión.
// Implementa solo la lectura de metadatos necesaria para los avisos de pista,
// sin playerctl ni dependencias externas.
package mpris

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/dbus"
)

const (
	playerPrefix    = "org.mpris.MediaPlayer2."
	playerPath      = "/org/mpris/MediaPlayer2"
	playerIface     = "org.mpris.MediaPlayer2.Player"
	propertiesIface = "org.freedesktop.DBus.Properties"
)

// Track contiene los metadatos públicos de una pista MPRIS.
type Track struct {
	Player         string
	Title          string
	Artist         string
	Album          string
	TrackID        string
	ArtworkURL     string
	PlaybackStatus string
}

// Valid indica si hay información suficiente para mostrar una tarjeta.
func (t Track) Valid() bool {
	return strings.TrimSpace(t.Title) != "" || strings.TrimSpace(t.Artist) != ""
}

// Key identifica una pista y permite no repetir el OSD por cambios de estado
// que no cambian la pista (play/pause, posición, etc.).
func (t Track) Key() string {
	id := strings.TrimSpace(t.TrackID)
	if id == "/org/mpris/MediaPlayer2/TrackList/NoTrack" {
		id = ""
	}
	return strings.Join([]string{
		strings.TrimSpace(t.Player), id, strings.TrimSpace(t.Title),
		strings.TrimSpace(t.Artist), strings.TrimSpace(t.Album), strings.TrimSpace(t.ArtworkURL),
	}, "\x00")
}

// FromMetadata extrae los campos xesam:* y mpris:artUrl de a{sv}.
func FromMetadata(metadata dbus.Dict) Track {
	artist := stringList(metadata.Value("xesam:artist"))
	if len(artist) == 0 {
		artist = stringList(metadata.Value("xesam:albumArtist"))
	}
	return Track{
		Title:      metadata.String("xesam:title"),
		Artist:     strings.Join(artist, ", "),
		Album:      metadata.String("xesam:album"),
		TrackID:    metadata.String("mpris:trackid"),
		ArtworkURL: metadata.String("mpris:artUrl"),
	}
}

func stringList(value any) []string {
	var values []string
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			values = append(values, strings.TrimSpace(v))
		}
	case []string:
		for _, item := range v {
			if item = strings.TrimSpace(item); item != "" {
				values = append(values, item)
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				values = append(values, strings.TrimSpace(s))
			}
		}
	}
	return values
}

// Watch conecta al autobús de sesión y llama onTrack cada vez que cambia la
// pista activa. El estado inicial se guarda como referencia, sin mostrar un
// OSD al iniciar la sesión. Al cancelar ctx, Watch retorna y cierra D-Bus.
func Watch(ctx context.Context, onTrack func(Track)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := dbus.DialSession()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Hello(); err != nil {
		return err
	}
	if err := addMatch(conn, "type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',path='/org/mpris/MediaPlayer2'"); err != nil {
		return err
	}
	if err := addMatch(conn, "type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged',path='/org/freedesktop/DBus'"); err != nil {
		return err
	}

	signals := make(chan *dbus.Message, 32)
	conn.SetSignalHandler(func(m *dbus.Message) {
		if !RelevantSignal(m) {
			return
		}
		select {
		case signals <- m:
		default: // Las señales repetidas se recuperan en el siguiente refresco.
		}
	})

	current, ok, err := currentTrack(conn, "")
	if err != nil {
		return err
	}
	lastKey := ""
	if ok && current.Valid() {
		lastKey = current.Key()
	}
	refresh := func(preferredOwner string) error {
		track, found, err := currentTrack(conn, preferredOwner)
		if err != nil {
			return err
		}
		key := ""
		if found && track.Valid() {
			key = track.Key()
		}
		if key != lastKey {
			lastKey = key
			if key != "" && onTrack != nil {
				onTrack(track)
			}
		}
		return nil
	}

	// Además de las señales, el refresco periódico recupera cambios si un
	// reproductor no emitió PropertiesChanged o si se perdió una señal.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case signal := <-signals:
			if err := refresh(signalOwner(signal)); err != nil {
				return err
			}
		case <-ticker.C:
			if err := refresh(""); err != nil {
				return err
			}
		}
	}
}

func addMatch(conn *dbus.Conn, rule string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := conn.Call(ctx, dbus.BusName, dbus.BusPath, dbus.BusIface, "AddMatch", "s", rule)
	if err != nil {
		return fmt.Errorf("suscripción MPRIS: %w", err)
	}
	return nil
}

// RelevantSignal indica si la señal podría cambiar el reproductor o sus
// metadatos. signalOwner devuelve el propietario que generó ese cambio.
func RelevantSignal(m *dbus.Message) bool {
	if m == nil || m.Type != dbus.Signal {
		return false
	}
	if m.Interface == "org.freedesktop.DBus" && m.Member == "NameOwnerChanged" && len(m.Body) >= 1 {
		name, _ := m.Body[0].(string)
		return strings.HasPrefix(name, playerPrefix)
	}
	if m.Interface != propertiesIface || m.Member != "PropertiesChanged" || m.Path != playerPath || len(m.Body) < 2 {
		return false
	}
	iface, _ := m.Body[0].(string)
	if iface != playerIface {
		return false
	}
	changed, _ := m.Body[1].(dbus.Dict)
	if changed.Has("Metadata") || changed.Has("PlaybackStatus") {
		return true
	}
	if len(m.Body) > 2 {
		return includesProperty(m.Body[2], "Metadata") || includesProperty(m.Body[2], "PlaybackStatus")
	}
	return false
}

func includesProperty(value any, want string) bool {
	switch v := value.(type) {
	case []string:
		for _, s := range v {
			if s == want {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func signalOwner(m *dbus.Message) string {
	if m == nil {
		return ""
	}
	if m.Interface == propertiesIface {
		return m.Sender
	}
	if m.Interface == "org.freedesktop.DBus" && len(m.Body) >= 3 {
		name, _ := m.Body[0].(string)
		owner, _ := m.Body[2].(string)
		if strings.HasPrefix(name, playerPrefix) {
			return owner
		}
	}
	return ""
}

type playerState struct {
	name  string
	track Track
}

func currentTrack(conn *dbus.Conn, preferredOwner string) (Track, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	reply, err := conn.Call(ctx, dbus.BusName, dbus.BusPath, dbus.BusIface, "ListNames", "")
	cancel()
	if err != nil {
		return Track{}, false, fmt.Errorf("listando reproductores MPRIS: %w", err)
	}
	if len(reply.Body) == 0 {
		return Track{}, false, nil
	}
	names, _ := reply.Body[0].([]any)
	var players []playerState
	for _, value := range names {
		name, ok := value.(string)
		if !ok || !strings.HasPrefix(name, playerPrefix) {
			continue
		}
		track, ok := readPlayer(conn, name)
		if !ok || !track.Valid() {
			continue
		}
		players = append(players, playerState{name: name, track: track})
	}
	if len(players) == 0 {
		return Track{}, false, nil
	}
	sort.Slice(players, func(i, j int) bool { return players[i].name < players[j].name })

	best := 0
	for i := 1; i < len(players); i++ {
		if playbackRank(players[i].track.PlaybackStatus) < playbackRank(players[best].track.PlaybackStatus) {
			best = i
		}
	}
	// Si varias instancias están al mismo nivel de reproducción, priorizar la
	// que emitió la señal; no dejar que una app pausada tape otra que reproduce.
	if preferredOwner != "" {
		for i := range players {
			owner := getNameOwner(conn, players[i].name)
			if owner == preferredOwner && playbackRank(players[i].track.PlaybackStatus) <= playbackRank(players[best].track.PlaybackStatus) {
				return players[i].track, true, nil
			}
		}
	}
	return players[best].track, true, nil
}

func readPlayer(conn *dbus.Conn, name string) (Track, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	reply, err := conn.Call(ctx, name, playerPath, propertiesIface, "GetAll", "s", playerIface)
	if err != nil || len(reply.Body) == 0 {
		return Track{}, false
	}
	props, ok := reply.Body[0].(dbus.Dict)
	if !ok {
		return Track{}, false
	}
	metadata, ok := props.Value("Metadata").(dbus.Dict)
	if !ok {
		return Track{}, false
	}
	track := FromMetadata(metadata)
	track.Player = strings.TrimPrefix(name, playerPrefix)
	track.PlaybackStatus = props.String("PlaybackStatus")
	return track, track.Valid()
}

func getNameOwner(conn *dbus.Conn, name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply, err := conn.Call(ctx, dbus.BusName, dbus.BusPath, dbus.BusIface, "GetNameOwner", "s", name)
	if err != nil || len(reply.Body) == 0 {
		return ""
	}
	owner, _ := reply.Body[0].(string)
	return owner
}

func playbackRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "playing":
		return 0
	case "paused":
		return 1
	case "stopped":
		return 2
	default:
		return 3
	}
}
