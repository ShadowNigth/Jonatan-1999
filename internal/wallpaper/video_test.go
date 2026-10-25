package wallpaper

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScaledVideoHeightUsesSourceAspectRatio(t *testing.T) {
	for _, tc := range []struct {
		width, height, outputWidth, wantHeight int
	}{
		{1920, 1080, 160, 90},
		{1920, 1080, 640, 360},
		{1080, 1920, 540, 960},
	} {
		if got := scaledVideoHeight(tc.width, tc.height, tc.outputWidth); got != tc.wantHeight {
			t.Errorf("scaledVideoHeight(%d,%d,%d) = %d; se esperaba %d",
				tc.width, tc.height, tc.outputWidth, got, tc.wantHeight)
		}
	}
}

func TestVideoSourceStreamsFramesAtRequestedSize(t *testing.T) {
	binDir := t.TempDir()
	writeVideoTestTool(t, binDir, "ffprobe", "#!/bin/sh\nprintf 'width=4\\nheight=2\\nduration=1.000000\\n'\n")
	// El falso ffmpeg escribe exactamente dos fotogramas RGB de 4x2 píxeles.
	writeVideoTestTool(t, binDir, "ffmpeg", "#!/bin/sh\nprintf '%048d' 0\n")
	t.Setenv("PATH", binDir)

	video, err := NewVideoSource("ignored.mp4", 30)
	if err != nil {
		t.Fatalf("NewVideoSource: %v", err)
	}
	defer video.Close()

	var frame *Canvas
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		frame = video.Frame(4, 2)
		if frame != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if frame == nil {
		t.Fatal("el decodificador no devolvió un fotograma")
	}
	if frame.W != 4 || frame.H != 2 || len(frame.Pix) != 4*2*3 {
		t.Fatalf("tamaño del fotograma = %dx%d con %d bytes", frame.W, frame.H, len(frame.Pix))
	}
}

func writeVideoTestTool(t *testing.T, dir, name, script string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
