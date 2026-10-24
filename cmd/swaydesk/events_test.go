package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

func TestParseUdevEvents(t *testing.T) {
	input := `monitor will print the received events for:
UDEV  [123.4] add /devices/pci0000:00/usb1/1-1 (block)
ACTION=add
SUBSYSTEM=block
DEVTYPE=disk
ID_BUS=usb
ID_MODEL=Flash_Drive

UDEV  [130.0] remove /devices/pci0000:00/usb1/1-1 (block)
ACTION=remove
SUBSYSTEM=block
DEVTYPE=disk
ID_BUS=usb
DEVNAME=/dev/sdb
`
	var events []map[string]string
	if err := parseUdevEvents(strings.NewReader(input), func(event map[string]string) {
		events = append(events, event)
	}); err != nil {
		t.Fatalf("parseUdevEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("se esperaban dos eventos, se recibieron %d: %#v", len(events), events)
	}
	if events[0]["ACTION"] != "add" || events[0]["ID_MODEL"] != "Flash_Drive" {
		t.Fatalf("propiedades de conexión inesperadas: %#v", events[0])
	}
	if events[1]["ACTION"] != "remove" || events[1]["DEVNAME"] != "/dev/sdb" {
		t.Fatalf("propiedades de desconexión inesperadas: %#v", events[1])
	}
}

func TestUSBStorageNotificationFiltersAndLabelsDevice(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]string
		want       bool
		summary    string
		body       string
	}{
		{
			name: "conexión USB",
			properties: map[string]string{
				"ACTION": "add", "SUBSYSTEM": "block", "DEVTYPE": "disk",
				"ID_BUS": "usb", "ID_FS_LABEL": "RESPALDO", "DEVNAME": "/dev/sdb",
			},
			want: true, summary: "Dispositivo conectado", body: "Almacenamiento USB: RESPALDO",
		},
		{
			name: "desconexión por ruta USB",
			properties: map[string]string{
				"ACTION": "remove", "SUBSYSTEM": "block", "DEVTYPE": "disk",
				"DEVPATH": "/devices/pci0000:00/usb1/1-1", "DEVNAME": "/dev/sdb",
			},
			want: true, summary: "Dispositivo desconectado", body: "Almacenamiento USB: /dev/sdb",
		},
		{
			name: "partición ignorada",
			properties: map[string]string{
				"ACTION": "add", "SUBSYSTEM": "block", "DEVTYPE": "partition", "ID_BUS": "usb",
			},
		},
		{
			name: "disco interno ignorado",
			properties: map[string]string{
				"ACTION": "add", "SUBSYSTEM": "block", "DEVTYPE": "disk", "DEVPATH": "/devices/pci0000:00/ata1",
			},
		},
		{
			name: "cambio de estado ignorado",
			properties: map[string]string{
				"ACTION": "change", "SUBSYSTEM": "block", "DEVTYPE": "disk", "ID_BUS": "usb",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary, body, ok := usbStorageNotification(test.properties)
			if ok != test.want {
				t.Fatalf("ok = %v, esperaba %v", ok, test.want)
			}
			if !ok {
				return
			}
			if summary != test.summary || body != test.body {
				t.Fatalf("notificación = (%q, %q), esperaba (%q, %q)", summary, body, test.summary, test.body)
			}
		})
	}
}

func TestSwayDeviceNotification(t *testing.T) {
	tests := []struct {
		name    string
		typeID  swayipc.MessageType
		payload string
		want    bool
		summary string
		body    string
	}{
		{
			name: "entrada conectada", typeID: swayipc.EventInput,
			payload: `{"change":"added","input":{"name":"Logitech Mouse","identifier":"usb-Logitech_Mouse","type":"pointer"}}`,
			want: true, summary: "Dispositivo conectado", body: "Logitech Mouse · ratón",
		},
		{
			name: "monitor desconectado", typeID: swayipc.EventOutput,
			payload: `{"change":"disconnected","output":{"name":"HDMI-A-1"}}`,
			want: true, summary: "Monitor desconectado", body: "HDMI-A-1",
		},
		{
			name: "cambio de modo ignorado", typeID: swayipc.EventOutput,
			payload: `{"change":"change","output":{"name":"HDMI-A-1"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := &swayipc.Event{Type: test.typeID, Payload: json.RawMessage(test.payload)}
			summary, body, ok := swayDeviceNotification(event)
			if ok != test.want {
				t.Fatalf("ok = %v, esperaba %v", ok, test.want)
			}
			if !ok {
				return
			}
			if summary != test.summary || body != test.body {
				t.Fatalf("notificación = (%q, %q), esperaba (%q, %q)", summary, body, test.summary, test.body)
			}
		})
	}
}
