package swayipc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Modelos de datos
// ---------------------------------------------------------------------------

// Rect es un rectángulo en píxeles.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Workspace describe un workspace tal como lo devuelve GET_WORKSPACES.
type Workspace struct {
	Num     int    `json:"num"`
	Name    string `json:"name"`
	Visible bool   `json:"visible"`
	Focused bool   `json:"focused"`
	Urgent  bool   `json:"urgent"`
	Output  string `json:"output"`
	Rect    Rect   `json:"rect"`
}

// Output describe una salida (monitor) conectada.
type Output struct {
	Name             string `json:"name"`
	Active           bool   `json:"active"`
	DPMS             bool   `json:"dpms"`
	Primary          bool   `json:"primary"`
	Make             string `json:"make"`
	Model            string `json:"model"`
	Serial           string `json:"serial"`
	CurrentWorkspace string `json:"current_workspace"`
	Rect             Rect   `json:"rect"`
}

// Node es un nodo del árbol de contenedores de sway.
type Node struct {
	ID                 int64             `json:"id"`
	Name               string            `json:"name"`
	Type               string            `json:"type"`
	Border             string            `json:"border"`
	CurrentBorderWidth int               `json:"current_border_width"`
	Layout             string            `json:"layout"`
	Percent            *float64          `json:"percent"`
	Focused            bool              `json:"focused"`
	Focus              []int64           `json:"focus"`
	AppID              string            `json:"app_id"`
	PID                int               `json:"pid"`
	Window             *int64            `json:"window"`
	Shell              string            `json:"shell"`
	Urgent             bool              `json:"urgent"`
	Marks              []string          `json:"marks"`
	FullscreenMode     int               `json:"fullscreen_mode"`
	Sticky             bool              `json:"sticky"`
	Floating           string            `json:"floating"`
	Output             string            `json:"output"`
	Rect               Rect              `json:"rect"`
	Visible            *bool             `json:"visible"`
	ScratchpadState    string            `json:"scratchpad_state"`
	Nodes              []*Node           `json:"nodes"`
	FloatingNodes      []*Node           `json:"floating_nodes"`
	Props              map[string]any    `json:"-"`
	Command            string            `json:"-"`
	Extra              map[string]string `json:"-"`
}

// IsWindow indica si el nodo es una ventana real (no un contenedor).
func (n *Node) IsWindow() bool {
	return n.Type == "con" || n.Type == "floating_con"
}

// Title devuelve el mejor título disponible para la ventana.
func (n *Node) Title() string {
	if n.Name != "" {
		return n.Name
	}
	if v, ok := n.Extra["title"]; ok && v != "" {
		return v
	}
	return "sin título"
}

// App devuelve el identificador de la aplicación.
func (n *Node) App() string {
	if n.AppID != "" {
		return n.AppID
	}
	if v, ok := n.Extra["class"]; ok && v != "" {
		return v
	}
	return n.Title()
}

// Walk recorre el árbol en profundidad invocando fn con cada nodo.
func (n *Node) Walk(fn func(*Node) bool) bool {
	if n == nil {
		return true
	}
	if !fn(n) {
		return false
	}
	for _, child := range n.Nodes {
		if !child.Walk(fn) {
			return false
		}
	}
	for _, child := range n.FloatingNodes {
		if !child.Walk(fn) {
			return false
		}
	}
	return true
}

// FindFocused devuelve el primer nodo marcado como enfocado.
func (n *Node) FindFocused() *Node {
	var found *Node
	n.Walk(func(c *Node) bool {
		if c.Focused && found == nil && c.IsWindow() {
			found = c
			return false
		}
		return true
	})
	return found
}

// Leaves devuelve todas las ventanas del árbol.
func (n *Node) Leaves() []*Node {
	var out []*Node
	n.Walk(func(c *Node) bool {
		if c.IsWindow() {
			out = append(out, c)
		}
		return true
	})
	return out
}

// Version es la respuesta de GET_VERSION.
type Version struct {
	Major          int    `json:"major"`
	Minor          int    `json:"minor"`
	Patch          int    `json:"patch"`
	HumanReadable  string `json:"human_readable"`
	LoadedConfig   string `json:"loaded_config_file_name"`
	Architecture   string `json:"architecture"`
	WorkspaceCount int    `json:"num_workspaces"`
}

// Input describe un dispositivo de entrada (sway >= 1.0).
type Input struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Vendor     int    `json:"vendor"`
	Product    int    `json:"product"`
	XkbLayout  string `json:"xkb_active_layout_name"`
}

// Seat describe un asiento (selección de entrada) disponible.
type Seat struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Devices      []Input  `json:"devices"`
	Focus        int64    `json:"focus"`
}

// Reply es una entrada de la respuesta a RUN_COMMAND.
type Reply struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Parse   bool   `json:"parse_error"`
}

// ---------------------------------------------------------------------------
// Peticiones
// ---------------------------------------------------------------------------

// Command ejecuta uno o varios comandos de sway y devuelve sus resultados.
// Los comandos múltiples se separan con ';' igual que en el archivo de config.
func (c *Conn) Command(cmds ...string) ([]Reply, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	var payload []byte
	if len(cmds) == 1 {
		payload = []byte(cmds[0])
	} else {
		payload = []byte(strings.Join(cmds, "; "))
	}
	raw, err := c.RoundTripRaw(MsgRunCommand, payload)
	if err != nil {
		return nil, err
	}
	var replies []Reply
	if err := json.Unmarshal(raw, &replies); err != nil {
		return nil, fmt.Errorf("respuesta RUN_COMMAND inesperada: %w", err)
	}
	return replies, nil
}

// CommandOK ejecuta comandos y devuelve error si alguno falla.
func (c *Conn) CommandOK(cmds ...string) error {
	replies, err := c.Command(cmds...)
	if err != nil {
		return err
	}
	for _, r := range replies {
		if !r.Success {
			if r.Error != "" {
				return fmt.Errorf("sway: %s", r.Error)
			}
			return fmt.Errorf("sway rechazó el comando %q", strings.Join(cmds, "; "))
		}
	}
	return nil
}

// Workspaces devuelve los workspaces existentes.
func (c *Conn) Workspaces() ([]Workspace, error) {
	raw, err := c.RoundTrip(MsgGetWorkspaces, nil)
	if err != nil {
		return nil, err
	}
	var ws []Workspace
	if err := json.Unmarshal(raw, &ws); err != nil {
		return nil, err
	}
	return ws, nil
}

// Outputs devuelve las salidas conectadas.
func (c *Conn) Outputs() ([]Output, error) {
	raw, err := c.RoundTrip(MsgGetOutputs, nil)
	if err != nil {
		return nil, err
	}
	var outs []Output
	if err := json.Unmarshal(raw, &outs); err != nil {
		return nil, err
	}
	return outs, nil
}

// Tree devuelve el árbol de contenedores completo.
func (c *Conn) Tree() (*Node, error) {
	raw, err := c.RoundTrip(MsgGetTree, nil)
	if err != nil {
		return nil, err
	}
	return decodeTree(raw)
}

// decodeTree decodifica el árbol preservando los campos extra de cada nodo
// (X11 usa "window_properties" y sway nativo usa otros campos).
func decodeTree(raw []byte) (*Node, error) {
	var root Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	// Segunda pasada para los campos que no están en la estructura tipada.
	var aux struct {
		ID     int64 `json:"id"`
		Nodes  []map[string]json.RawMessage
		Floats []map[string]json.RawMessage `json:"floating_nodes"`
	}
	_ = aux
	index := map[int64]*Node{}
	root.Walk(func(n *Node) bool {
		index[n.ID] = n
		return true
	})
	var visit func(m map[string]json.RawMessage)
	visit = func(m map[string]json.RawMessage) {
		var id int64
		if v, ok := m["id"]; ok {
			_ = json.Unmarshal(v, &id)
		}
		if node, ok := index[id]; ok {
			node.Extra = map[string]string{}
			for _, key := range []string{"app_id", "class", "instance", "title"} {
				if v, ok := m[key]; ok {
					var s string
					if json.Unmarshal(v, &s) == nil {
						node.Extra[key] = s
					}
				}
			}
			if wp, ok := m["window_properties"]; ok {
				var props map[string]string
				if json.Unmarshal(wp, &props) == nil {
					for k, v := range props {
						node.Extra[k] = v
					}
				}
			}
		}
		for _, key := range []string{"nodes", "floating_nodes"} {
			var kids []map[string]json.RawMessage
			if v, ok := m[key]; ok {
				if json.Unmarshal(v, &kids) == nil {
					for _, kid := range kids {
						visit(kid)
					}
				}
			}
		}
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err == nil {
		visit(doc)
	}
	return &root, nil
}

// Version devuelve la versión de sway y el archivo de configuración cargado.
func (c *Conn) Version() (*Version, error) {
	raw, err := c.RoundTrip(MsgGetVersion, nil)
	if err != nil {
		return nil, err
	}
	var v Version
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// Inputs devuelve los dispositivos de entrada detectados.
func (c *Conn) Inputs() ([]Input, error) {
	raw, err := c.RoundTrip(MsgGetInputs, nil)
	if err != nil {
		return nil, err
	}
	var in []Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	return in, nil
}

// Seats devuelve los asientos disponibles.
func (c *Conn) Seats() ([]Seat, error) {
	raw, err := c.RoundTrip(MsgGetSeats, nil)
	if err != nil {
		return nil, err
	}
	var seats []Seat
	if err := json.Unmarshal(raw, &seats); err != nil {
		return nil, err
	}
	return seats, nil
}

// SendTick emite un evento TICK con el payload indicado.
func (c *Conn) SendTick(payload string) (bool, error) {
	raw, err := c.RoundTrip(MsgSendTick, payload)
	if err != nil {
		return false, err
	}
	var ok bool
	err = json.Unmarshal(raw, &ok)
	return ok, err
}

// Sync fuerza la sincronización del bucle de eventos (útil tras enviar
// comandos y esperar sus eventos antes de seguir).
func (c *Conn) Sync() error {
	_, err := c.RoundTrip(MsgSync, nil)
	return err
}
