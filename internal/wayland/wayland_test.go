package wayland

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestEncoderHeaderAndAlignment(t *testing.T) {
	msg := newEncoder(42, 3).uint32(7).string("hola").int32(-1).bytes()
	if len(msg)%4 != 0 {
		t.Fatalf("mensaje sin alinear: %d bytes", len(msg))
	}
	if got := binary.LittleEndian.Uint32(msg[0:]); got != 42 {
		t.Fatalf("objeto = %d", got)
	}
	if got := binary.LittleEndian.Uint16(msg[4:]); got != 3 {
		t.Fatalf("opcode = %d", got)
	}
	size := int(binary.LittleEndian.Uint16(msg[6:]))
	if size != len(msg) {
		t.Fatalf("tamaño declarado %d, real %d", size, len(msg))
	}
}

func TestFillBGRAFastPathPreservesBytes(t *testing.T) {
	pixels := []byte{1, 2, 3, 255, 4, 5, 6, 255}
	buffer := &Buffer{W: 2, H: 1, Stride: 8, Mem: make([]byte, 8)}
	buffer.FillBGRA(pixels, 2, 1)
	if !bytes.Equal(buffer.Mem, pixels) {
		t.Fatalf("FillBGRA alteró el formato BGRA: %v", buffer.Mem)
	}
}

func TestFillBGRAScalesPixels(t *testing.T) {
	pixel := []byte{10, 20, 30, 255}
	buffer := &Buffer{W: 2, H: 2, Stride: 8, Mem: make([]byte, 16)}
	buffer.FillBGRA(pixel, 1, 1)
	for i := 0; i < len(buffer.Mem); i += 4 {
		if !bytes.Equal(buffer.Mem[i:i+4], pixel) {
			t.Fatalf("píxel escalado %d = %v; se esperaba %v", i/4, buffer.Mem[i:i+4], pixel)
		}
	}
}

func TestDecoderRoundtrip(t *testing.T) {
	enc := newEncoder(9, 1).uint32(11).string("hola").fixed(1.5).int32(-7)
	msg, err := decodeMessage(enc.bytes(), []int{5, 6})
	if err != nil {
		t.Fatal(err)
	}
	if msg.id != 9 || msg.opcode != 1 {
		t.Fatalf("cabecera inesperada: %d/%d", msg.id, msg.opcode)
	}
	if v := msg.args.uint32(); v != 11 {
		t.Fatalf("uint32 = %d", v)
	}
	if s := msg.args.string(); s != "hola" {
		t.Fatalf("string = %q", s)
	}
	if f := msg.args.fixed(); f != 1.5 {
		t.Fatalf("fixed = %f", f)
	}
	if v := msg.args.int32(); v != -7 {
		t.Fatalf("int32 = %d", v)
	}
	if fd := msg.args.fd(); fd != 5 {
		t.Fatalf("fd = %d", fd)
	}
	if fd := msg.args.fd(); fd != 6 {
		t.Fatalf("fd = %d", fd)
	}
	// Sin más descriptores devuelve -1 en vez de entrar en pánico.
	if fd := msg.args.fd(); fd != -1 {
		t.Fatalf("fd extra = %d", fd)
	}
}

func TestDecoderStringPadding(t *testing.T) {
	// "abcd" mide 5 con el NUL y necesita relleno hasta 8.
	e := newEncoder(1, 1).string("abcd").uint32(0xAABBCCDD)
	msg, err := decodeMessage(e.bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := msg.args.string(); s != "abcd" {
		t.Fatalf("string = %q", s)
	}
	if v := msg.args.uint32(); v != 0xAABBCCDD {
		t.Fatalf("el relleno rompió la alineación: %x", v)
	}
	// String vacío (nulo en el protocolo) se lee como cadena vacía.
	e2 := newEncoder(1, 1).string("")
	msg2, err := decodeMessage(e2.bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := msg2.args.string(); s != "" {
		t.Fatalf("string nulo = %q", s)
	}
}

func TestDecodeShortMessage(t *testing.T) {
	if _, err := decodeMessage([]byte{1, 2, 3}, nil); err == nil {
		t.Fatal("se esperaba error con una cabecera incompleta")
	}
}

func TestDetectHelpers(t *testing.T) {
	// Solo comprueban que no entran en pánico sin servidor.
	_ = Detect()
	_ = Available()
	_ = EnvDisplay()
}

type recordedWaylandRequest struct {
	id     uint32
	opcode uint16
	args   []byte
}

type layerSurfaceMockResult struct {
	requests []recordedWaylandRequest
	err      error
}

func readTestWaylandMessage(r io.Reader) (message, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(r, header); err != nil {
		return message{}, err
	}
	size := int(binary.LittleEndian.Uint16(header[6:8]))
	if size < len(header) {
		return message{}, io.ErrUnexpectedEOF
	}
	frame := make([]byte, size)
	copy(frame, header)
	if _, err := io.ReadFull(r, frame[len(header):]); err != nil {
		return message{}, err
	}
	return decodeMessage(frame, nil)
}

func TestDispatchReportsWaylandProtocolError(t *testing.T) {
	runtimeDir := t.TempDir()
	socket := filepath.Join(runtimeDir, "wayland-test")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")

	serverErr := make(chan error, 1)
	go func() {
		server, err := listener.AcceptUnix()
		if err != nil {
			serverErr <- err
			return
		}
		defer server.Close()
		frame := newEncoder(1, displayErrorEvent).
			uint32(17).uint32(2).string("invalid object id").bytes()
		_, err = server.Write(frame)
		serverErr <- err
	}()

	conn, err := NewConn()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = conn.Dispatch(time.Second)
	protocolErr, ok := err.(Error)
	if !ok {
		t.Fatalf("Dispatch error = %T %v, se esperaba Error del compositor", err, err)
	}
	if protocolErr.ObjectID != 17 || protocolErr.Code != 2 || protocolErr.Message != "invalid object id" {
		t.Fatalf("error de protocolo inesperado: %+v", protocolErr)
	}
}

func TestDispatchZeroPollsWithoutBlocking(t *testing.T) {
	runtimeDir := t.TempDir()
	socket := filepath.Join(runtimeDir, "wayland-test")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")

	accepted := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	go func() {
		server, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		defer server.Close()
		close(accepted)
		<-release
	}()

	conn, err := NewConn()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	<-accepted

	type dispatchResult struct {
		processed int
		err       error
	}
	done := make(chan dispatchResult, 1)
	go func() {
		n, err := conn.Dispatch(0)
		done <- dispatchResult{processed: n, err: err}
	}()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("Dispatch(0): %v", result.err)
		}
		if result.processed != 0 {
			t.Fatalf("Dispatch(0) procesó %d eventos sin datos", result.processed)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Dispatch(0) bloqueó el hilo en vez de sondear el socket")
	}
}

func TestLayerSurfaceConstructorsUseBoundObjectIDs(t *testing.T) {
	t.Run("fondo", func(t *testing.T) {
		testLayerSurfaceConstructor(t, -1, func(c *Conn) (bool, error) {
			bg, err := NewBackground(c, 1920, 1080, "test-wallpaper")
			if err != nil {
				return false, err
			}
			return bg.Configured(), nil
		})
	})
	t.Run("panel", func(t *testing.T) {
		testLayerSurfaceConstructor(t, 0, func(c *Conn) (bool, error) {
			panel, err := NewPanel(c, PanelSpec{Namespace: "test-panel"})
			if err != nil {
				return false, err
			}
			return panel.configured, nil
		})
	})
}

func testLayerSurfaceConstructor(t *testing.T, wantExclusiveZone int32, create func(*Conn) (bool, error)) {
	t.Helper()
	runtimeDir := t.TempDir()
	socket := filepath.Join(runtimeDir, "wayland-test")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	t.Setenv("SWAYSOCK", filepath.Join(runtimeDir, "no-sway-socket"))

	mockResult := make(chan layerSurfaceMockResult, 1)
	keepMockOpen := make(chan struct{})
	defer close(keepMockOpen)
	go func() {
		server, err := listener.AcceptUnix()
		if err != nil {
			mockResult <- layerSurfaceMockResult{err: err}
			return
		}
		defer server.Close()

		getRegistry, err := readTestWaylandMessage(server)
		if err != nil {
			mockResult <- layerSurfaceMockResult{err: err}
			return
		}
		syncRequest, err := readTestWaylandMessage(server)
		if err != nil {
			mockResult <- layerSurfaceMockResult{err: err}
			return
		}
		if getRegistry.id != 1 || getRegistry.opcode != displayGetRegist ||
			syncRequest.id != 1 || syncRequest.opcode != displaySync {
			mockResult <- layerSurfaceMockResult{err: io.ErrUnexpectedEOF}
			return
		}
		registryID := getRegistry.args.uint32()
		callbackID := syncRequest.args.uint32()
		var globals []byte
		globals = append(globals, newEncoder(registryID, registryGlobal).
			uint32(10).string("wl_compositor").uint32(6).bytes()...)
		globals = append(globals, newEncoder(registryID, registryGlobal).
			uint32(11).string("zwlr_layer_shell_v1").uint32(4).bytes()...)
		globals = append(globals, newEncoder(callbackID, callbackDoneEvent).uint32(1).bytes()...)
		if _, err := server.Write(globals); err != nil {
			mockResult <- layerSurfaceMockResult{err: err}
			return
		}

		var requests []recordedWaylandRequest
		var shellID, layerID uint32
		for {
			msg, err := readTestWaylandMessage(server)
			if err != nil {
				mockResult <- layerSurfaceMockResult{err: err}
				return
			}
			requests = append(requests, recordedWaylandRequest{
				id: msg.id, opcode: msg.opcode, args: append([]byte(nil), msg.args.buf...),
			})
			if msg.id == registryID && msg.opcode == registryBind {
				d := decoder{buf: msg.args.buf}
				globalName := d.uint32()
				iface := d.string()
				d.uint32() // versión
				id := d.uint32()
				if globalName == 11 && iface == "zwlr_layer_shell_v1" {
					shellID = id
				}
			}
			if shellID != 0 && msg.id == shellID && msg.opcode == layerShellGetSurface {
				d := decoder{buf: msg.args.buf}
				layerID = d.uint32()
			}
			if msg.opcode == surfaceCommit {
				break
			}
		}
		if layerID == 0 {
			mockResult <- layerSurfaceMockResult{requests: requests, err: io.ErrUnexpectedEOF}
			return
		}
		configure := newEncoder(layerID, layerSurfaceConfigureEvent).
			uint32(1).uint32(1920).uint32(1080).bytes()
		if _, err := server.Write(configure); err != nil {
			mockResult <- layerSurfaceMockResult{requests: requests, err: err}
			return
		}
		mockResult <- layerSurfaceMockResult{requests: requests}
		<-keepMockOpen
	}()

	conn, err := NewConn()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.GetRegistry(); err != nil {
		t.Fatal(err)
	}
	configured, err := create(conn)
	if err != nil {
		select {
		case result := <-mockResult:
			t.Fatalf("constructor: %v; mock requests=%+v mock err=%v", err, result.requests, result.err)
		default:
			t.Fatalf("constructor: %v (mock no respondió)", err)
		}
	}
	if !configured {
		t.Fatal("el compositor no confirmó la superficie")
	}

	var result layerSurfaceMockResult
	select {
	case result = <-mockResult:
	case <-time.After(2 * time.Second):
		t.Fatal("el mock Wayland no terminó de procesar las solicitudes")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}

	zoneFound := false
	for _, req := range result.requests {
		if req.opcode != layerSurfaceSetExclusiveZone {
			continue
		}
		zoneFound = true
		got := int32(binary.LittleEndian.Uint32(req.args))
		if got != wantExclusiveZone {
			t.Fatalf("set_exclusive_zone = %d, se esperaba %d", got, wantExclusiveZone)
		}
	}
	if !zoneFound {
		t.Fatal("no se envió zwlr_layer_surface_v1.set_exclusive_zone")
	}

	var registryID, compositorID, shellID, surfaceID, layerID uint32
	layerRequestIndex := -1
	for _, req := range result.requests {
		if req.id == 2 && req.opcode == registryBind {
			d := decoder{buf: req.args}
			globalName := d.uint32()
			iface := d.string()
			d.uint32() // versión
			id := d.uint32()
			registryID = req.id
			if globalName == 10 && iface == "wl_compositor" {
				compositorID = id
			}
			if globalName == 11 && iface == "zwlr_layer_shell_v1" {
				shellID = id
			}
		}
	}
	for i, req := range result.requests {
		if req.id == compositorID && req.opcode == compositorCreateSurface {
			d := decoder{buf: req.args}
			surfaceID = d.uint32()
		}
		if req.id == shellID && req.opcode == layerShellGetSurface {
			layerRequestIndex = i
			d := decoder{buf: req.args}
			layerID = d.uint32()
			surfaceInRequest := d.uint32()
			if surfaceInRequest != surfaceID {
				t.Fatalf("get_layer_surface refiere wl_surface %d; se creó %d", surfaceInRequest, surfaceID)
			}
		}
	}
	if registryID == 0 || compositorID == 0 || shellID == 0 || surfaceID == 0 || layerID == 0 {
		t.Fatalf("faltan ids en el protocolo: registry=%d compositor=%d shell=%d surface=%d layer=%d", registryID, compositorID, shellID, surfaceID, layerID)
	}
	if compositorID == 10 || shellID == 11 {
		t.Fatalf("se usó el nombre de global como id local: compositor=%d shell=%d", compositorID, shellID)
	}
	if layerID == shellID {
		t.Fatalf("get_layer_surface reutilizó el id del shell: %d", layerID)
	}
	if layerRequestIndex < 0 {
		t.Fatal("no se envió zwlr_layer_shell_v1.get_layer_surface")
	}
	for _, req := range result.requests[layerRequestIndex+1:] {
		if req.opcode == surfaceCommit {
			if req.id != surfaceID {
				t.Fatalf("wl_surface.commit enviado al objeto %d; se esperaba %d", req.id, surfaceID)
			}
			continue
		}
		if req.id != layerID {
			t.Fatalf("request zwlr_layer_surface opcode=%d enviado al objeto %d; se esperaba %d", req.opcode, req.id, layerID)
		}
	}
}
