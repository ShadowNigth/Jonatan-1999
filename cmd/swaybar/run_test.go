package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
	"github.com/ShadowNigth/swaydesk/internal/launcher"
)

func TestPowerMenuExecCommandUsesTerminalAndTUIArgs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "path with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	foot := filepath.Join(dir, "foot")
	if err := os.WriteFile(foot, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	menu := filepath.Join(dir, "swaymenu with spaces")

	command, err := powerMenuExecCommand(&config.Config{Terminal: "foot"}, menu)
	if err != nil {
		t.Fatal(err)
	}
	argv := []string{
		foot,
		"--app-id=swaydesk-menu",
		"--title=swaydesk",
		"-W", "58x14",
		"-e", menu,
		"--no-float", "--power",
	}
	want := "exec --no-startup-id " + launcher.ShellQuoteArgs(argv)
	if command != want {
		t.Fatalf("powerMenuExecCommand() = %q, esperado %q", command, want)
	}
}
