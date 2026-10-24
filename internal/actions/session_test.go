package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScreenshotSavesAndCopiesPNG(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mode         string
		wantGeometry bool
	}{
		{name: "pantalla completa", mode: "full"},
		{name: "región", mode: "region", wantGeometry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			binDir := t.TempDir()
			clipboardPath := filepath.Join(t.TempDir(), "clipboard.bin")
			geometryPath := filepath.Join(t.TempDir(), "geometry.txt")
			t.Setenv("HOME", home)
			t.Setenv("PATH", binDir)
			t.Setenv("CLIPBOARD_CAPTURE_FILE", clipboardPath)
			t.Setenv("GRIM_GEOMETRY_FILE", geometryPath)

			writeExecutable(t, binDir, "grim", `#!/bin/sh
if [ "$1" = "-g" ]; then
  printf '%s' "$2" > "$GRIM_GEOMETRY_FILE"
  output="$3"
else
  output="$1"
fi
printf 'test-png-data' > "$output"
`)
			writeExecutable(t, binDir, "slurp", "#!/bin/sh\nprintf '20,30 40x50\\n'\n")
			writeExecutable(t, binDir, "wl-copy", `#!/bin/sh
[ "$1" = "--type" ] && [ "$2" = "image/png" ] || exit 2
/usr/bin/cat > "$CLIPBOARD_CAPTURE_FILE"
`)

			if err := Screenshot(tc.mode); err != nil {
				t.Fatalf("Screenshot(%q): %v", tc.mode, err)
			}

			captureDir := filepath.Join(home, "Imágenes", "Capturas")
			files, err := os.ReadDir(captureDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 1 || !strings.HasSuffix(files[0].Name(), ".png") {
				t.Fatalf("se esperaba un PNG guardado, archivos: %v", files)
			}
			capture, err := os.ReadFile(filepath.Join(captureDir, files[0].Name()))
			if err != nil {
				t.Fatal(err)
			}
			clipboard, err := os.ReadFile(clipboardPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(capture) != "test-png-data" || string(clipboard) != string(capture) {
				t.Fatalf("captura/portapapeles distintos: captura=%q clipboard=%q", capture, clipboard)
			}
			if tc.wantGeometry {
				geometry, err := os.ReadFile(geometryPath)
				if err != nil {
					t.Fatal(err)
				}
				if string(geometry) != "20,30 40x50" {
					t.Fatalf("geometría pasada a grim = %q", geometry)
				}
			}
		})
	}
}

func writeExecutable(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("escribiendo %s: %v", path, err)
	}
}
