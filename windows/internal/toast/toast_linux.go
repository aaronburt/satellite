//go:build linux

package toast

import (
	"github.com/gen2brain/beeep"
	"github.com/godbus/dbus/v5"
)

// Seams so tests don't show real notifications.
var (
	notifyDBus = func(title, message string, actions []string, hints map[string]dbus.Variant) error {
		conn, err := dbus.SessionBus()
		if err != nil {
			return err
		}
		defer conn.Close()
		obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
		var notifID uint32
		call := obj.Call("org.freedesktop.Notifications.Notify", 0, "Satellite", uint32(0), "dialog-information", title, message, actions, hints, int32(5000))
		if call.Err != nil {
			return call.Err
		}
		return call.Store(&notifID)
	}
	notifyFallback = beeep.Notify
)

func Show(n Notification) error {
	hints := make(map[string]dbus.Variant)
	if n.Silent {
		hints["suppress-sound"] = dbus.MakeVariant(true)
	}
	var actions []string
	if _, ok := SafeURL(n.URL); ok {
		actions = []string{"default", "Open"}
	}
	if err := notifyDBus(n.Title, n.Message, actions, hints); err == nil {
		return nil
	}
	return notifyFallback(n.Title, n.Message, "")
}
