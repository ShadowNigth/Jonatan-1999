package main

import "testing"

func TestParseMediaFlags(t *testing.T) {
	vals, positional := parseFlags([]string{
		"--title", "Song from the night",
		"--artist=Aimer & Friends",
		"--album", "Night Sky",
		"--art-url", "file:///tmp/cover%20art.png",
		"--status", "Playing",
		"--timeout=2500",
	}, nil)
	if len(positional) != 0 {
		t.Fatalf("argumentos posicionales inesperados: %v", positional)
	}
	for key, want := range map[string]string{
		"title": "Song from the night", "artist": "Aimer & Friends", "album": "Night Sky",
		"art-url": "file:///tmp/cover%20art.png", "status": "Playing", "timeout": "2500",
	} {
		if got := last(vals, key); got != want {
			t.Errorf("--%s = %q, se esperaba %q", key, got, want)
		}
	}
}
