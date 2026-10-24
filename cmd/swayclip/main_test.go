package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/ui"
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

func TestCliphistCopyDoesNotWaitForPersistentWlCopy(t *testing.T) {
	binDir := t.TempDir()
	tempDir := t.TempDir()
	releasePath := filepath.Join(tempDir, "release-wl-copy")
	finishedPath := filepath.Join(tempDir, "wl-copy-finished")
	t.Setenv("PATH", binDir)
	t.Setenv("CLIPHIST_TEST_RELEASE", releasePath)
	t.Setenv("CLIPHIST_TEST_FINISHED", finishedPath)

	writeSwayclipExecutable(t, binDir, "cliphist", "#!/bin/sh\n[ \"$1\" = decode ] || exit 2\nprintf 'copied text'\n")
	writeSwayclipExecutable(t, binDir, "wl-copy", "#!/bin/sh\n/usr/bin/cat >/dev/null\n(while [ ! -e \"$CLIPHIST_TEST_RELEASE\" ]; do /usr/bin/sleep 0.01; done; : > \"$CLIPHIST_TEST_FINISHED\") &\nexit 0\n")

	copyDone := make(chan error, 1)
	copyReturned := false
	defer func() {
		_ = os.WriteFile(releasePath, []byte("release"), 0o600)
		if !copyReturned {
			select {
			case <-copyDone:
			case <-time.After(2 * time.Second):
			}
		}
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(finishedPath); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	go func() { copyDone <- (cliphist{}).copy(clipItem{id: "7"}) }()
	select {
	case err := <-copyDone:
		copyReturned = true
		if err != nil {
			t.Fatalf("cliphist.copy: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cliphist.copy esperó a que terminara el propietario persistente del portapapeles")
	}
}

func TestDeleteShortcutRecognizesCtrlD(t *testing.T) {
	ctrlD := ui.Key{Rune: 'd', Ctrl: true, Name: "ctrl+d"}
	if !isDeleteShortcut(ctrlD) {
		t.Fatal("Ctrl+D no activa el borrado")
	}
	if isDeleteShortcut(ui.Key{Rune: 'd'}) || isDeleteShortcut(ui.Key{Rune: 'x', Ctrl: true}) ||
		isDeleteShortcut(ui.Key{Rune: 'd', Ctrl: true, Alt: true}) {
		t.Fatal("se reconoció una tecla distinta de Ctrl+D")
	}
}

func TestCliphistRemoveSendsSelectedEntry(t *testing.T) {
	binDir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "cliphist-input.txt")
	t.Setenv("PATH", binDir)
	t.Setenv("CLIPHIST_TEST_INPUT", inputPath)
	writeSwayclipExecutable(t, binDir, "cliphist", "#!/bin/sh\n[ \"$1\" = delete ] || exit 2\n/usr/bin/cat > \"$CLIPHIST_TEST_INPUT\"\n")

	const raw = "42\tpreview seleccionado"
	if err := (cliphist{}).remove(clipItem{raw: raw}); err != nil {
		t.Fatalf("cliphist.remove: %v", err)
	}
	got, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != raw+"\n" {
		t.Fatalf("cliphist recibió %q; se esperaba %q", got, raw+"\n")
	}
}

func TestClipmanRemoveReportsUnsupported(t *testing.T) {
	if err := (clipman{}).remove(clipItem{}); err == nil {
		t.Fatal("clipman no debería informar que borró una entrada individual")
	}
}

func writeSwayclipExecutable(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
		t.Fatalf("escribiendo %s: %v", name, err)
	}
}
