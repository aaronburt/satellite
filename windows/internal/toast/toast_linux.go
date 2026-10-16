//go:build linux

package toast

import (
	"github.com/gen2brain/beeep"
	"github.com/godbus/dbus/v5"
)

func Show(n Notification) error {
	conn, err := dbus.SessionBus()
	if err == nil {
		defer conn.Close()

		hints := make(map[string]dbus.Variant)
		if n.Silent {
			hints["suppress-sound"] = dbus.MakeVariant(true)
		}

		var actions []string
		if n.URL != "" {
			actions = []string{"default", "Open"}
		}

		obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
		var notifID uint32
		call := obj.Call("org.freedesktop.Notifications.Notify", 0, "Satellite", uint32(0), "dialog-information", n.Title, n.Message, actions, hints, int32(5000))
		if call.Err == nil && call.Store(&notifID) == nil {
			return nil
		}
	}

	return beeep.Notify(n.Title, n.Message, "")
}
