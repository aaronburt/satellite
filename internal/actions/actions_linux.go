//go:build linux

package actions

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
)

var errNoMediaPlayer = errors.New("no active media player found on D-Bus")

func callMprisMethod(method string) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	var names []string
	if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return err
	}

	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			obj := conn.Object(name, "/org/mpris/MediaPlayer2")
			call := obj.Call("org.mpris.MediaPlayer2.Player."+method, 0)
			return call.Err
		}
	}
	return errNoMediaPlayer
}

func LockWorkstation() error {
	conn, err := dbus.SystemBus()
	if err == nil {
		defer conn.Close()
		obj := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1/session/auto")
		call := obj.Call("org.freedesktop.login1.Session.Lock", 0)
		if call.Err == nil {
			return nil
		}
	}

	cmd := exec.Command("loginctl", "lock-session")
	return cmd.Run()
}

func TogglePlayPause() error {
	return callMprisMethod("PlayPause")
}

func NextTrack() error {
	return callMprisMethod("Next")
}

func PreviousTrack() error {
	return callMprisMethod("Previous")
}

func Stop() error {
	return callMprisMethod("Stop")
}

func ToggleMute() error {
	cmd := exec.Command("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle")
	if err := cmd.Run(); err == nil {
		return nil
	}
	fallback := exec.Command("pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle")
	return fallback.Run()
}

func VolumeUp() error {
	cmd := exec.Command("wpctl", "set-volume", "-l", "1.5", "@DEFAULT_AUDIO_SINK@", "2%+")
	if err := cmd.Run(); err == nil {
		return nil
	}
	fallback := exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", "+2%")
	return fallback.Run()
}

func VolumeDown() error {
	cmd := exec.Command("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "2%-")
	if err := cmd.Run(); err == nil {
		return nil
	}
	fallback := exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", "-2%")
	return fallback.Run()
}
