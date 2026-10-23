package main

import (
	"github.com/ShadowNigth/swaydesk/internal/dbus"
	"github.com/ShadowNigth/swaydesk/internal/notify"
)

// hasNotifyDaemon indica si hay un servidor de notificaciones que hable nuestro
// protocolo extendido (swaynotify) en el bus de sesión.
func hasNotifyDaemon() bool {
	if !dbus.SessionAvailable() {
		return false
	}
	c, err := notify.DialClient()
	if err != nil {
		return false
	}
	defer c.Close()
	return c.Available()
}

// notifySend intenta primero nuestro daemon y, si no está, delega en
// notify-send a través de actions.Notification.
func notifySend(n notify.Notification) bool {
	c, err := notify.DialClient()
	if err != nil {
		return false
	}
	defer c.Close()
	if _, err := c.Send(n); err != nil {
		return false
	}
	return true
}
