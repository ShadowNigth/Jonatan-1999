package wallpaper

import (
	"os"
	"path/filepath"
	"strings"
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

func TestVideoFilterDropsExcessFramesBeforeScaling(t *testing.T) {
	filter := videoFilter(1920, 1080, 30, false, false)
	if !strings.HasPrefix(filter, "fps=30,scale=") {
		t.Fatalf("el filtro debe limitar FPS antes del escalado: %s", filter)
	}
}

func TestVAAPIFilterScalesBeforeDownloading(t *testing.T) {
	filter := videoFilter(1360, 768, 30, true, true)
	if !strings.HasPrefix(filter, "scale_vaapi=w=1360:h=768:format=nv12:") {
		t.Fatalf("el filtro debe escalar en VAAPI: %s", filter)
	}
	if !strings.Contains(filter, ",hwdownload,format=nv12,fps=30,crop=1360:768,format=bgra") {
		t.Fatalf("el filtro VAAPI debe descargar el frame ya escalado: %s", filter)
	}
}

func TestVAAPIFilterFallsBackToSoftwareScaling(t *testing.T) {
	filter := videoFilter(1920, 1080, 30, true, false)
	if !strings.HasPrefix(filter, "hwdownload,format=nv12,fps=30,scale=") {
		t.Fatalf("filtro de respaldo VAAPI inesperado: %s", filter)
	}
	if !strings.HasSuffix(filter, ",format=bgra") {
		t.Fatalf("el filtro VAAPI de respaldo no termina en BGRA: %s", filter)
	}
}

func TestCanvasFromBGRARestoresRGBOrder(t *testing.T) {
	canvas := canvasFromBGRA([]byte{10, 20, 30, 255}, 1, 1)
	if canvas == nil || canvas.W != 1 || canvas.H != 1 {
		t.Fatalf("canvas BGRA inválido: %+v", canvas)
	}
	if got := canvas.Pix; len(got) != 3 || got[0] != 30 || got[1] != 20 || got[2] != 10 {
		t.Fatalf("conversión BGRA→RGB inesperada: %v", got)
	}
}

func TestVideoSourceFallsBackWhenVAAPIFails(t *testing.T) {
	binDir := t.TempDir()
	writeVideoTestTool(t, binDir, "ffprobe", "#!/bin/sh\nprintf 'width=4\\nheight=2\\nduration=1.000000\\n'\n")
	writeVideoTestTool(t, binDir, "ffmpeg", "#!/bin/sh\ncase \" $* \" in *\" -hwaccel \"*) echo 'VAAPI unavailable' >&2; exit 1;; esac\nprintf '%064d' 0\n")
	t.Setenv("PATH", binDir)

	video, err := NewVideoSource("ignored.mp4", 30)
	if err != nil {
		t.Fatalf("NewVideoSource: %v", err)
	}
	video.vaapiDevice = "/dev/dri/renderD128" // fuerza el intento en esta prueba sin GPU real
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
		t.Fatal("el decodificador por software no recuperó el vídeo")
	}
	if !video.forceSoftware || video.usingVAAPI {
		t.Fatalf("no se activó la ruta de respaldo por software")
	}
}

func TestVideoSourceStreamsFramesAtRequestedSize(t *testing.T) {
	binDir := t.TempDir()
	writeVideoTestTool(t, binDir, "ffprobe", "#!/bin/sh\nprintf 'width=4\\nheight=2\\nduration=1.000000\\n'\n")
	// El falso ffmpeg escribe exactamente dos fotogramas BGRA de 4x2 píxeles.
	writeVideoTestTool(t, binDir, "ffmpeg", "#!/bin/sh\nprintf '%064d' 0\n")
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
