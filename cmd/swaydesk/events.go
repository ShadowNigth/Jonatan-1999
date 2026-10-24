package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ShadowNigth/swaydesk/internal/actions"
	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

const eventWatchRetryDelay = 2 * time.Second

// cmdEvents supervisa cambios de dispositivos durante la sesión de Sway.
func cmdEvents(args []string) {
	if len(args) != 1 || args[0] != "watch" {
		fmt.Fprintln(os.Stderr, "Uso: swaydesk events watch")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	swaySocket := swayipc.SwaySock()
	hasSway := swaySocket != ""
	if hasSway && strings.TrimSpace(os.Getenv("SWAYSOCK")) == "" && strings.TrimSpace(os.Getenv("I3SOCK")) == "" {
		_ = os.Setenv("SWAYSOCK", swaySocket)
	}
	udevadm, udevErr := exec.LookPath("udevadm")
	hasUdev := udevErr == nil
	if !hasSway && !hasUdev {
		fmt.Fprintln(os.Stderr, "swaydesk events: no se encontró una sesión de Sway ni udevadm; no hay eventos que vigilar")
		return
	}
	if !hasSway {
		fmt.Fprintln(os.Stderr, "swaydesk events: SWAYSOCK no disponible; solo se vigilará el almacenamiento USB")
	}
	if !hasUdev {
		fmt.Fprintln(os.Stderr, "swaydesk events: udevadm no está disponible; se omiten las unidades USB")
	}

	var watchers sync.WaitGroup
	if hasSway {
		watchers.Add(1)
		go func() {
			defer watchers.Done()
			watchSwayDevices(ctx)
		}()
	}
	if hasUdev {
		watchers.Add(1)
		go func() {
			defer watchers.Done()
			watchUSBStorage(ctx, udevadm)
		}()
	}

	<-ctx.Done()
	watchers.Wait()
}

// watchSwayDevices recibe las conexiones y desconexiones de entradas y salidas
// publicadas por el protocolo IPC de Sway.
func watchSwayDevices(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := swayipc.Dial()
		if err != nil {
			if !waitForContext(ctx, eventWatchRetryDelay) {
				return
			}
			continue
		}
		if err := conn.Subscribe("input", "output"); err != nil {
			_ = conn.Close()
			if !waitForContext(ctx, eventWatchRetryDelay) {
				return
			}
			continue
		}

		// Next normalmente vence tras diez segundos sin eventos. El watcher debe
		// permanecer suscrito indefinidamente; Close interrumpe la lectura al salir.
		conn.SetTimeout(0)
		events := conn.Events(8)
	readEvents:
		for {
			select {
			case <-ctx.Done():
				_ = conn.Close()
				for range events {
				}
				return
			case event, ok := <-events:
				if !ok {
					break readEvents
				}
				if summary, body, ok := swayDeviceNotification(event); ok {
					actions.Notification(summary, body, "low")
				}
			}
		}
		_ = conn.Close()
		if !waitForContext(ctx, eventWatchRetryDelay) {
			return
		}
	}
}

// watchUSBStorage sigue los eventos de discos USB de udev. Solo notifica discos
// completos para no generar un aviso adicional por cada partición del mismo USB.
func watchUSBStorage(ctx context.Context, udevadm string) {
	retryDelay := time.Second
	for ctx.Err() == nil {
		cmd := exec.CommandContext(ctx, udevadm, "monitor", "--udev", "--property", "--subsystem-match=block")
		cmd.Stderr = os.Stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk events: no se pudo abrir udevadm: %v\n", err)
			if !waitForContext(ctx, retryDelay) {
				return
			}
			retryDelay = nextEventRetryDelay(retryDelay)
			continue
		}
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "swaydesk events: no se pudo iniciar udevadm: %v\n", err)
			if !waitForContext(ctx, retryDelay) {
				return
			}
			retryDelay = nextEventRetryDelay(retryDelay)
			continue
		}

		parseErr := parseUdevEvents(stdout, func(properties map[string]string) {
			if summary, body, ok := usbStorageNotification(properties); ok {
				actions.Notification(summary, body, "low")
			}
		})
		waitErr := cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "swaydesk events: leyendo udevadm: %v\n", parseErr)
		} else if waitErr != nil {
			fmt.Fprintf(os.Stderr, "swaydesk events: udevadm terminó: %v\n", waitErr)
		} else {
			fmt.Fprintln(os.Stderr, "swaydesk events: udevadm terminó inesperadamente")
		}
		if !waitForContext(ctx, retryDelay) {
			return
		}
		retryDelay = nextEventRetryDelay(retryDelay)
	}
}

func parseUdevEvents(reader io.Reader, onEvent func(map[string]string)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	properties := make(map[string]string)
	flush := func() {
		if len(properties) == 0 {
			return
		}
		onEvent(properties)
		properties = make(map[string]string)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "UDEV ") || strings.HasPrefix(line, "KERNEL ") {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && key != "" {
			properties[key] = value
		}
	}
	flush()
	return scanner.Err()
}

func swayDeviceNotification(event *swayipc.Event) (summary, body string, ok bool) {
	if event == nil {
		return "", "", false
	}
	var payload struct {
		Change string `json:"change"`
		Input  struct {
			Name       string `json:"name"`
			Identifier string `json:"identifier"`
			Type       string `json:"type"`
		} `json:"input"`
		Output struct {
			Name  string `json:"name"`
			Make  string `json:"make"`
			Model string `json:"model"`
		} `json:"output"`
	}
	if err := event.Decode(&payload); err != nil {
		return "", "", false
	}

	switch event.Type {
	case swayipc.EventInput:
		switch payload.Change {
		case "added":
			summary = "Dispositivo conectado"
		case "removed":
			summary = "Dispositivo desconectado"
		default:
			return "", "", false
		}
		name := firstEventValue(payload.Input.Name, payload.Input.Identifier)
		if name == "" {
			name = "Dispositivo de entrada"
		}
		if inputType := inputTypeName(payload.Input.Type); inputType != "" {
			name += " · " + inputType
		}
		return summary, name, true

	case swayipc.EventOutput:
		switch payload.Change {
		case "connected":
			summary = "Monitor conectado"
		case "disconnected":
			summary = "Monitor desconectado"
		default:
			return "", "", false
		}
		name := firstEventValue(payload.Output.Name, strings.TrimSpace(payload.Output.Make+" "+payload.Output.Model))
		if name == "" {
			name = "Salida de vídeo"
		}
		return summary, name, true
	default:
		return "", "", false
	}
}

func inputTypeName(inputType string) string {
	switch strings.ToLower(strings.TrimSpace(inputType)) {
	case "keyboard":
		return "teclado"
	case "pointer":
		return "ratón"
	case "touch":
		return "pantalla táctil"
	case "tablet_tool":
		return "lápiz de tableta"
	case "tablet_pad":
		return "controles de tableta"
	case "switch":
		return "interruptor"
	default:
		return strings.TrimSpace(inputType)
	}
}

func usbStorageNotification(properties map[string]string) (summary, body string, ok bool) {
	if properties["SUBSYSTEM"] != "block" || properties["DEVTYPE"] != "disk" {
		return "", "", false
	}
	if properties["ID_BUS"] != "usb" && !strings.Contains(properties["DEVPATH"], "/usb") {
		return "", "", false
	}

	switch properties["ACTION"] {
	case "add":
		summary = "Dispositivo conectado"
	case "remove":
		summary = "Dispositivo desconectado"
	default:
		return "", "", false
	}
	name := firstEventValue(
		properties["ID_FS_LABEL"],
		properties["ID_MODEL_FROM_DATABASE"],
		properties["ID_MODEL"],
		properties["DEVNAME"],
	)
	if name == "" {
		name = "Unidad USB"
	}
	return summary, "Almacenamiento USB: " + name, true
}

func firstEventValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func waitForContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextEventRetryDelay(delay time.Duration) time.Duration {
	delay *= 2
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}
