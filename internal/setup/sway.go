package setup

import "github.com/ShadowNigth/swaydesk/internal/swayipc"

// dialSway abre una conexión IPC con la sesión de sway actual.
func dialSway() (*swayipc.Conn, error) {
	return swayipc.Dial()
}
