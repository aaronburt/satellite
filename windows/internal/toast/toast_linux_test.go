//go:build linux

package toast

import (
	"errors"
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
)

type call struct {
	title, message string
	actions        []string
	hints          map[string]dbus.Variant
}

func stubNotify(t *testing.T, dbusErr error) (*[]call, *[]string) {
	t.Helper()
	origD, origF := notifyDBus, notifyFallback
	t.Cleanup(func() { notifyDBus, notifyFallback = origD, origF })
	var dbusCalls []call
	var fallback []string
	notifyDBus = func(title, message string, actions []string, hints map[string]dbus.Variant) error {
		dbusCalls = append(dbusCalls, call{title, message, actions, hints})
		return dbusErr
	}
	notifyFallback = func(title, message string, _ any) error {
		fallback = append(fallback, title+"|"+message)
		return nil
	}
	return &dbusCalls, &fallback
}

func TestShowUsesDBusNotifications(t *testing.T) {
	calls, fallback := stubNotify(t, nil)
	if err := Show(Notification{Title: "Door", Message: "Someone is here", Silent: true, URL: "https://ha.local"}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || len(*fallback) != 0 {
		t.Fatalf("dbus calls %d, fallback %d", len(*calls), len(*fallback))
	}
	c := (*calls)[0]
	if c.title != "Door" || c.message != "Someone is here" {
		t.Fatalf("call = %+v", c)
	}
	if !reflect.DeepEqual(c.actions, []string{"default", "Open"}) {
		t.Errorf("actions = %v", c.actions)
	}
	if v, ok := c.hints["suppress-sound"]; !ok || v.Value() != true {
		t.Errorf("silent notification should suppress sound: %v", c.hints)
	}
}

func TestShowWithoutURLOrSilence(t *testing.T) {
	calls, _ := stubNotify(t, nil)
	_ = Show(Notification{Message: "m", URL: "file:///etc/passwd"})
	c := (*calls)[0]
	if len(c.actions) != 0 {
		t.Errorf("unsafe URL must not get an Open action: %v", c.actions)
	}
	if len(c.hints) != 0 {
		t.Errorf("hints = %v", c.hints)
	}
}

func TestShowFallsBackWhenDBusFails(t *testing.T) {
	_, fallback := stubNotify(t, errors.New("no session bus"))
	if err := Show(Notification{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*fallback, []string{"T|M"}) {
		t.Fatalf("fallback = %v", *fallback)
	}
}
