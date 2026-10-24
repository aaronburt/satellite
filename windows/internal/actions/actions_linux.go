//go:build linux

package actions

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
)

var errNoMediaPlayer = errors.New("no active media player found on D-Bus")

// Seams for tests: the real implementations talk to D-Bus and run audio
// tools, which are not available (or safe to call) in CI.
var (
	runCommand = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	listSessionBusNames = func() ([]string, error) {
		conn, err := dbus.SessionBus()
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		var names []string
		err = conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
		return names, err
	}
	callMprisPlayer = func(busName, method string) error {
		conn, err := dbus.SessionBus()
		if err != nil {
			return err
		}
		defer conn.Close()
		obj := conn.Object(busName, "/org/mpris/MediaPlayer2")
		return obj.Call("org.mpris.MediaPlayer2.Player."+method, 0).Err
	}
	lockViaLogind = func() error {
		conn, err := dbus.SystemBus()
		if err != nil {
			return err
		}
		defer conn.Close()
		obj := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1/session/auto")
		return obj.Call("org.freedesktop.login1.Session.Lock", 0).Err
	}
)

// firstMprisPlayer returns the first MPRIS media player in a list of D-Bus names.
func firstMprisPlayer(names []string) (string, bool) {
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			return name, true
		}
	}
	return "", false
}

func callMprisMethod(method string) error {
	names, err := listSessionBusNames()
	if err != nil {
		return err
	}
	player, ok := firstMprisPlayer(names)
	if !ok {
		return errNoMediaPlayer
	}
	return callMprisPlayer(player, method)
}

// runWithFallback runs the PipeWire command and, if it fails, the PulseAudio one.
func runWithFallback(primary, fallback []string) error {
	if err := runCommand(primary[0], primary[1:]...); err == nil {
		return nil
	}
	return runCommand(fallback[0], fallback[1:]...)
}

func LockWorkstation() error {
	if err := lockViaLogind(); err == nil {
		return nil
	}
	return runCommand("loginctl", "lock-session")
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
	return runWithFallback(
		[]string{"wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"},
		[]string{"pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle"},
	)
}

func VolumeUp() error {
	return runWithFallback(
		[]string{"wpctl", "set-volume", "-l", "1.5", "@DEFAULT_AUDIO_SINK@", "2%+"},
		[]string{"pactl", "set-sink-volume", "@DEFAULT_SINK@", "+2%"},
	)
}

func VolumeDown() error {
	return runWithFallback(
		[]string{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "2%-"},
		[]string{"pactl", "set-sink-volume", "@DEFAULT_SINK@", "-2%"},
	)
}
