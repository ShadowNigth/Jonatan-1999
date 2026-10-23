package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDesktopFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chromium.desktop")
	content := `[Desktop Entry]
Type=Application
Name=Chromium
Name[es]=Navegador Chromium
Comment=Explorar la web
Exec=chromium %u
Icon=chromium
Terminal=false
Categories=Network;WebBrowser;
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entry, ok := ParseDesktopFile(path)
	if !ok {
		t.Fatal("no se pudo parsear")
	}
	if entry.Name != "Chromium" && entry.Name != "Navegador Chromium" {
		t.Fatalf("nombre inesperado: %q", entry.Name)
	}
	if entry.Exec != "chromium %u" || entry.Icon != "chromium" {
		t.Fatalf("entrada inesperada: %+v", entry)
	}
	if entry.Key != "app:chromium" && entry.Key != "app:navegador chromium" {
		t.Fatalf("clave inesperada: %q", entry.Key)
	}
	if entry.Terminal {
		t.Fatal("Terminal debía ser false")
	}
}

func TestParseDesktopOculta(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"nodisplay.desktop": "[Desktop Entry]\nName=X\nExec=x\nNoDisplay=true\n",
		"hidden.desktop":    "[Desktop Entry]\nName=Y\nExec=y\nHidden=true\n",
		"sinexec.desktop":   "[Desktop Entry]\nName=Z\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := ParseDesktopFile(path); ok {
			t.Fatalf("%s no debía cargarse", name)
		}
	}
}

func TestExecArgs(t *testing.T) {
	cases := map[string][]string{
		`chromium %u`:                            {"chromium"},
		`sh -c "echo hola mundo" %f`:             {"sh", "-c", "echo hola mundo"},
		`code --new-window %F`:                   {"code", "--new-window"},
		`env FOO=bar app %i --flag=1`:            {"env", "FOO=bar", "app", "--flag=1"},
		`app "ruta con espacios/archivo.txt" %U`: {"app", "ruta con espacios/archivo.txt"},
		`porcentaje 100%%`:                       {"porcentaje", "100%"},
	}
	for in, want := range cases {
		got := ExecArgs(in)
		if len(got) != len(want) {
			t.Fatalf("ExecArgs(%q) = %q, esperaba %q", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ExecArgs(%q) = %q, esperaba %q", in, got, want)
			}
		}
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	h := LoadHistory(path, 3)
	now := int64(1_700_000_000)

	if h.Score("app:chromium", now) != 0 {
		t.Fatal("una entrada nueva no debe puntuar")
	}
	h.Record("app:chromium", now)
	h.Record("app:chromium", now)
	if got := h.Score("app:chromium", now); got <= 0 {
		t.Fatalf("puntuación = %d", got)
	}
	// Hace 30 días la puntuación debe decaer.
	if h.Score("app:chromium", now+30*86400) >= h.Score("app:chromium", now) {
		t.Fatal("la puntuación no decae con el tiempo")
	}

	// Persistencia y recorte a maxEntries.
	h2 := LoadHistory(path, 3)
	if h2.Score("app:chromium", now) == 0 {
		t.Fatal("el historial no se guardó en disco")
	}
	for i := 0; i < 10; i++ {
		h.Record("cmd:"+string(rune('a'+i)), now)
	}
	h3 := LoadHistory(path, 3)
	if len(h3.entries) > 3 {
		t.Fatalf("el historial no se recortó: %d entradas", len(h3.entries))
	}
}

func TestPathCommands(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "mi-programa")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	notExe := filepath.Join(dir, "solo-lectura")
	if err := os.WriteFile(notExe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	cmds := PathCommands()
	if len(cmds) != 1 || cmds[0].Name != "mi-programa" {
		t.Fatalf("comandos inesperados: %+v", cmds)
	}
}

func TestPreferredTerminal(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := PreferredTerminal(""); got != "" {
		t.Fatalf("sin terminales esperaba cadena vacía, obtuve %q", got)
	}
	dir := t.TempDir()
	foot := filepath.Join(dir, "foot")
	if err := os.WriteFile(foot, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := PreferredTerminal(""); got != "foot" {
		t.Fatalf("PreferredTerminal = %q", got)
	}
	if got := PreferredTerminal("kitty"); got != "foot" {
		t.Fatalf("si el preferido no existe debe usar foot, obtuve %q", got)
	}
}

func TestSplitQuoted(t *testing.T) {
	got := splitQuoted(`a "b c" 'd e' f\ g`)
	want := []string{"a", "b c", "d e", "f g"}
	if len(got) != len(want) {
		t.Fatalf("splitQuoted = %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitQuoted = %q", got)
		}
	}
}
