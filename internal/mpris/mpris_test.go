package mpris

import (
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/dbus"
)

func TestFromMetadata(t *testing.T) {
	metadata := dbus.Dict{
		{Key: "xesam:title", Value: dbus.Variant{Sig: "s", Value: "Hikari"}},
		{Key: "xesam:artist", Value: dbus.Variant{Sig: "as", Value: []any{"Aimer", "Guest Vocal"}}},
		{Key: "xesam:album", Value: dbus.Variant{Sig: "s", Value: "Night Sky"}},
		{Key: "mpris:trackid", Value: dbus.Variant{Sig: "o", Value: "/track/42"}},
		{Key: "mpris:artUrl", Value: dbus.Variant{Sig: "s", Value: "file:///tmp/cover.png"}},
	}
	got := FromMetadata(metadata)
	if got.Title != "Hikari" || got.Artist != "Aimer, Guest Vocal" ||
		got.Album != "Night Sky" || got.TrackID != "/track/42" ||
		got.ArtworkURL != "file:///tmp/cover.png" {
		t.Fatalf("FromMetadata() = %+v", got)
	}
	got.Player = "example"
	got.PlaybackStatus = "Playing"
	if !got.Valid() || got.Key() == "" {
		t.Fatalf("la pista debería ser válida y tener clave: %+v", got)
	}
}

func TestFromMetadataFallsBackToAlbumArtist(t *testing.T) {
	metadata := dbus.Dict{
		{Key: "xesam:title", Value: dbus.Variant{Sig: "s", Value: "Track"}},
		{Key: "xesam:albumArtist", Value: dbus.Variant{Sig: "as", Value: []any{"Band"}}},
	}
	if got := FromMetadata(metadata); got.Artist != "Band" {
		t.Fatalf("artista alternativo = %q", got.Artist)
	}
}

func TestTrackKeyAndNoTrack(t *testing.T) {
	a := Track{Player: "spotify", Title: "Song", Artist: "Artist", TrackID: "/track/1", PlaybackStatus: "Playing"}
	b := a
	b.PlaybackStatus = "Paused"
	if a.Key() != b.Key() {
		t.Fatal("play/pause no debe distinguir una pista")
	}
	b.TrackID = "/track/2"
	if a.Key() == b.Key() {
		t.Fatal("las pistas con ids distintos deben producir claves distintas")
	}
	a.TrackID = "/org/mpris/MediaPlayer2/TrackList/NoTrack"
	b.TrackID = ""
	if a.Key() != b.Key() {
		t.Fatalf("NoTrack no debe distinguir una pista sin id: %q / %q", a.Key(), b.Key())
	}
}

func TestSignalOwner(t *testing.T) {
	properties := &dbus.Message{Interface: propertiesIface, Sender: ":1.25"}
	if got := signalOwner(properties); got != ":1.25" {
		t.Fatalf("propietario de PropertiesChanged = %q", got)
	}
	ownerChanged := &dbus.Message{
		Interface: "org.freedesktop.DBus",
		Body:      []any{playerPrefix + "mpv", ":1.10", ":1.30"},
	}
	if got := signalOwner(ownerChanged); got != ":1.30" {
		t.Fatalf("propietario tras NameOwnerChanged = %q", got)
	}
}

func TestRelevantSignal(t *testing.T) {
	cases := []struct {
		name string
		msg  *dbus.Message
		want bool
	}{
		{
			name: "metadata",
			msg: &dbus.Message{
				Type: dbus.Signal, Interface: propertiesIface, Member: "PropertiesChanged", Path: playerPath,
				Body: []any{playerIface, dbus.Dict{{Key: "Metadata", Value: dbus.Variant{Sig: "a{sv}", Value: dbus.Dict{}}}}, []any{}},
			},
			want: true,
		},
		{
			name: "metadata invalidated",
			msg: &dbus.Message{
				Type: dbus.Signal, Interface: propertiesIface, Member: "PropertiesChanged", Path: playerPath,
				Body: []any{playerIface, dbus.Dict{}, []any{"Metadata"}},
			},
			want: true,
		},
		{
			name: "player owner",
			msg: &dbus.Message{
				Type: dbus.Signal, Interface: "org.freedesktop.DBus", Member: "NameOwnerChanged",
				Body: []any{playerPrefix + "spotify", "", ":1.42"},
			},
			want: true,
		},
		{
			name: "other property",
			msg: &dbus.Message{
				Type: dbus.Signal, Interface: propertiesIface, Member: "PropertiesChanged", Path: playerPath,
				Body: []any{playerIface, dbus.Dict{{Key: "Volume", Value: dbus.Variant{Sig: "d", Value: 0.5}}}, []any{}},
			},
			want: false,
		},
		{
			name: "non player interface",
			msg: &dbus.Message{
				Type: dbus.Signal, Interface: propertiesIface, Member: "PropertiesChanged", Path: playerPath,
				Body: []any{"org.mpris.MediaPlayer2", dbus.Dict{}, []any{"Metadata"}},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RelevantSignal(tc.msg); got != tc.want {
				t.Fatalf("RelevantSignal() = %t, esperado %t", got, tc.want)
			}
		})
	}
}
