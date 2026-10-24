package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCliphistListImageEntries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		line       string
		wantMIME   string
		wantLabel  string
		wantRaw    string
	}{
		{
			name:      "metadata MIME",
			line:      "42\timage/png\t[[ binary data 24 KiB png 1920x1080 ]]",
			wantMIME:  "image/png",
			wantLabel: "Imagen (PNG)",
			wantRaw:   "42\t[[ binary data 24 KiB png 1920x1080 ]]",
		},
		{
			name:      "formato antiguo",
			line:      "41\t[[ binary data 24 KiB png 1920x1080 ]]",
			wantMIME:  "image/png",
			wantLabel: "Imagen (PNG)",
			wantRaw:   "41\t[[ binary data 24 KiB png 1920x1080 ]]",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := parseCliphistList(tc.line)
			if len(items) != 1 {
				t.Fatalf("parseCliphistList devolvió %d entradas; se esperaba 1", len(items))
			}
			item := items[0]
			if item.imageMIME != tc.wantMIME || item.label != tc.wantLabel || item.raw != tc.wantRaw {
				t.Fatalf("entrada inesperada: %+v", item)
			}
		})
	}
}

func TestParseCliphistDoesNotTreatTextAsImage(t *testing.T) {
	items := parseCliphistList("7\ttext/plain;charset=utf-8\tla cadena image/png no es una imagen")
	if len(items) != 1 {
		t.Fatalf("parseCliphistList devolvió %d entradas; se esperaba 1", len(items))
	}
	if items[0].imageMIME != "" || items[0].content != "la cadena image/png no es una imagen" {
		t.Fatalf("se confundió texto con imagen: %+v", items[0])
	}
}

func TestCliphistCopyPreservesImageMIME(t *testing.T) {
	binDir := t.TempDir()
	clipboardPath := filepath.Join(t.TempDir(), "clipboard.bin")
	argsPath := filepath.Join(t.TempDir(), "wl-copy-args.txt")
	t.Setenv("PATH", binDir)
	t.Setenv("CLIPHIST_TEST_ARGS", argsPath)
	t.Setenv("CLIPHIST_TEST_CLIPBOARD", clipboardPath)

	writeSwayclipExecutable(t, binDir, "cliphist", "#!/bin/sh\n[ \"$1\" = decode ] && [ \"$2\" = 42 ] || exit 2\nprintf 'png-bytes'\n")
	writeSwayclipExecutable(t, binDir, "wl-copy", "#!/bin/sh\nprintf '%s' \"$*\" > \"$CLIPHIST_TEST_ARGS\"\n/usr/bin/cat > \"$CLIPHIST_TEST_CLIPBOARD\"\n")

	if err := (cliphist{}).copy(clipItem{id: "42", imageMIME: "image/png"}); err != nil {
		t.Fatalf("cliphist.copy: %v", err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(args) != "--type image/png" {
		t.Fatalf("wl-copy recibió argumentos %q", args)
	}
	clipboard, err := os.ReadFile(clipboardPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(clipboard) != "png-bytes" {
		t.Fatalf("datos copiados inesperados: %q", clipboard)
	}
}

func writeSwayclipExecutable(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
		t.Fatalf("escribiendo %s: %v", name, err)
	}
}
