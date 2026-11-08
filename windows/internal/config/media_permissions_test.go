package config

import "testing"

func TestMediaPermissions(t *testing.T) {
	actions := []string{"media_play_pause", "media_play", "media_pause", "media_stop", "media_next", "media_prev", "volume_mute", "volume_up", "volume_down"}
	for _, action := range actions {
		if (ExposeConfig{}).MediaActionAllowed(action) {
			t.Fatal(action)
		}
		if !(ExposeConfig{MediaControl: true}).MediaActionAllowed(action) {
			t.Fatal(action)
		}
		if (ExposeConfig{MediaControl: true, MediaPermissions: &MediaPermissions{}}).MediaActionAllowed(action) {
			t.Fatal(action)
		}
	}
	permissions := []MediaPermissions{{PlayPause: true}, {Stop: true}, {Next: true}, {Previous: true}, {Mute: true}, {VolumeUp: true}, {VolumeDown: true}}
	groups := [][]string{{"media_play_pause", "media_play", "media_pause"}, {"media_stop"}, {"media_next"}, {"media_prev"}, {"volume_mute"}, {"volume_up"}, {"volume_down"}}
	for i, p := range permissions {
		e := ExposeConfig{MediaControl: true, MediaPermissions: &p}
		for _, action := range actions {
			want := false
			for _, allowed := range groups[i] {
				if action == allowed {
					want = true
				}
			}
			if e.MediaActionAllowed(action) != want {
				t.Errorf("permission %d action %s", i, action)
			}
		}
	}
}

func TestMediaCommandsWithoutReporting(t *testing.T) {
	enabled, disabled := true, false
	e := ExposeConfig{MediaCommands: &enabled, MediaPermissions: &MediaPermissions{Mute: true}}
	if e.MediaControl || !e.MediaActionAllowed("volume_mute") || e.MediaActionAllowed("media_next") {
		t.Fatal("controls must be independent of reporting")
	}
	e.MediaControl = true
	e.MediaCommands = &disabled
	if e.MediaActionAllowed("volume_mute") {
		t.Fatal("reporting must not override disabled controls")
	}
}
