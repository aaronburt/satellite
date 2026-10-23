//go:build linux

package actions

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeShell struct {
	ran  [][]string
	fail map[string]bool // keyed by command name
}

func (f *fakeShell) run(name string, args ...string) error {
	f.ran = append(f.ran, append([]string{name}, args...))
	if f.fail[name] {
		return errors.New(name + " failed")
	}
	return nil
}

func stubLinux(t *testing.T) *fakeShell {
	t.Helper()
	shell := &fakeShell{fail: map[string]bool{}}
	origRun, origList, origCall, origLock := runCommand, listSessionBusNames, callMprisPlayer, lockViaLogind
	t.Cleanup(func() {
		runCommand, listSessionBusNames, callMprisPlayer, lockViaLogind = origRun, origList, origCall, origLock
	})
	runCommand = shell.run
	listSessionBusNames = func() ([]string, error) { return nil, nil }
	callMprisPlayer = func(string, string) error { return errors.New("unexpected MPRIS call") }
	lockViaLogind = func() error { return errors.New("no logind") }
	return shell
}

func TestFirstMprisPlayer(t *testing.T) {
	names := []string{":1.42", "org.freedesktop.Notifications", "org.mpris.MediaPlayer2.spotify", "org.mpris.MediaPlayer2.vlc"}
	got, ok := firstMprisPlayer(names)
	if !ok || got != "org.mpris.MediaPlayer2.spotify" {
		t.Fatalf("firstMprisPlayer = %q, %v", got, ok)
	}
	if _, ok := firstMprisPlayer([]string{"org.mpris.MediaPlayer2"}); ok {
		t.Fatal("bare MPRIS prefix without a player name must not match")
	}
	if _, ok := firstMprisPlayer(nil); ok {
		t.Fatal("empty list must not match")
	}
}

func TestMediaActionsCallMprisOnFirstPlayer(t *testing.T) {
	stubLinux(t)
	listSessionBusNames = func() ([]string, error) {
		return []string{"org.gnome.Shell", "org.mpris.MediaPlayer2.firefox.instance_1", "org.mpris.MediaPlayer2.vlc"}, nil
	}
	var got []string
	callMprisPlayer = func(bus, method string) error {
		got = append(got, bus+"#"+method)
		return nil
	}

	for _, fn := range []func() error{TogglePlayPause, NextTrack, PreviousTrack, Stop} {
		if err := fn(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"org.mpris.MediaPlayer2.firefox.instance_1#PlayPause",
		"org.mpris.MediaPlayer2.firefox.instance_1#Next",
		"org.mpris.MediaPlayer2.firefox.instance_1#Previous",
		"org.mpris.MediaPlayer2.firefox.instance_1#Stop",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MPRIS calls = %v, want %v", got, want)
	}
}

func TestMediaActionsWithoutPlayer(t *testing.T) {
	stubLinux(t)
	listSessionBusNames = func() ([]string, error) { return []string{"org.gnome.Shell"}, nil }
	if err := TogglePlayPause(); !errors.Is(err, errNoMediaPlayer) {
		t.Fatalf("err = %v, want errNoMediaPlayer", err)
	}
}

func TestMediaActionsPropagateBusErrors(t *testing.T) {
	stubLinux(t)
	busErr := errors.New("no session bus")
	listSessionBusNames = func() ([]string, error) { return nil, busErr }
	if err := NextTrack(); !errors.Is(err, busErr) {
		t.Fatalf("err = %v, want %v", err, busErr)
	}

	listSessionBusNames = func() ([]string, error) { return []string{"org.mpris.MediaPlayer2.vlc"}, nil }
	callErr := errors.New("method failed")
	callMprisPlayer = func(string, string) error { return callErr }
	if err := Stop(); !errors.Is(err, callErr) {
		t.Fatalf("err = %v, want %v", err, callErr)
	}
}

func TestAudioActionsPreferPipeWire(t *testing.T) {
	cases := map[string]struct {
		fn   func() error
		want []string
	}{
		"mute": {ToggleMute, []string{"wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"}},
		"up":   {VolumeUp, []string{"wpctl", "set-volume", "-l", "1.5", "@DEFAULT_AUDIO_SINK@", "2%+"}},
		"down": {VolumeDown, []string{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "2%-"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			shell := stubLinux(t)
			if err := tc.fn(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(shell.ran, [][]string{tc.want}) {
				t.Fatalf("ran %v, want only %v", shell.ran, tc.want)
			}
		})
	}
}

func TestAudioActionsFallBackToPulseAudio(t *testing.T) {
	cases := map[string]struct {
		fn   func() error
		want []string
	}{
		"mute": {ToggleMute, []string{"pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle"}},
		"up":   {VolumeUp, []string{"pactl", "set-sink-volume", "@DEFAULT_SINK@", "+2%"}},
		"down": {VolumeDown, []string{"pactl", "set-sink-volume", "@DEFAULT_SINK@", "-2%"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			shell := stubLinux(t)
			shell.fail["wpctl"] = true
			if err := tc.fn(); err != nil {
				t.Fatal(err)
			}
			if len(shell.ran) != 2 || shell.ran[0][0] != "wpctl" || !reflect.DeepEqual(shell.ran[1], tc.want) {
				t.Fatalf("ran %v, want wpctl then %v", shell.ran, tc.want)
			}
		})
	}
}

func TestAudioActionsReportFailureWhenBothToolsFail(t *testing.T) {
	shell := stubLinux(t)
	shell.fail["wpctl"] = true
	shell.fail["pactl"] = true
	err := VolumeUp()
	if err == nil || !strings.Contains(err.Error(), "pactl") {
		t.Fatalf("err = %v, want pactl failure", err)
	}
}

func TestLockPrefersLogindThenLoginctl(t *testing.T) {
	shell := stubLinux(t)
	lockViaLogind = func() error { return nil }
	if err := LockWorkstation(); err != nil {
		t.Fatal(err)
	}
	if len(shell.ran) != 0 {
		t.Fatalf("loginctl should not run when logind succeeds, ran %v", shell.ran)
	}

	lockViaLogind = func() error { return errors.New("denied") }
	if err := LockWorkstation(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(shell.ran, [][]string{{"loginctl", "lock-session"}}) {
		t.Fatalf("ran %v, want loginctl lock-session", shell.ran)
	}

	shell.fail["loginctl"] = true
	if err := LockWorkstation(); err == nil {
		t.Fatal("expected error when both lock methods fail")
	}
}

// The system executor wired into NewRegistry must reach the platform code.
func TestSystemExecutorDelegatesToPlatform(t *testing.T) {
	shell := stubLinux(t)
	reg := NewRegistry()
	if err := reg.Execute("volume_down"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Execute("lock"); err != nil {
		t.Fatal(err)
	}
	if len(shell.ran) != 2 || shell.ran[0][0] != "wpctl" || shell.ran[1][0] != "loginctl" {
		t.Fatalf("ran %v", shell.ran)
	}
}
