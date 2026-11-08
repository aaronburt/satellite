package mqtt

import (
	"encoding/json"
	"satellite/internal/capabilities"
	"satellite/internal/config"
	"testing"
)

func TestGranularMediaDiscoveryAndCommands(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Expose.MediaControl = true
	cfg.Expose.MediaPermissions = &config.MediaPermissions{Mute: true}
	for _, action := range []string{"media_play_pause", "media_play", "media_pause", "media_stop", "media_next", "media_prev", "volume_up", "volume_down"} {
		if commandAllowed(cfg.Expose, action) {
			t.Errorf("allowed %s", action)
		}
	}
	if !commandAllowed(cfg.Expose, "volume_mute") {
		t.Fatal("mute denied")
	}
	for _, item := range GetAllDiscoveryItems(cfg, false, capabilities.PlatformCapabilities{MediaControl: true, AudioOutput: true}) {
		switch item.Key {
		case "media_play_pause", "media_next", "media_prev", "volume_up", "volume_down":
			if item.ShouldRun {
				t.Errorf("discovered %s", item.Key)
			}
		case "volume_mute":
			if !item.ShouldRun {
				t.Fatal("mute missing")
			}
		case "media_player":
			var payload map[string]interface{}
			if err := json.Unmarshal(item.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"payload_play", "payload_pause", "payload_stop", "payload_next", "payload_previous"} {
				if _, ok := payload[key]; ok {
					t.Errorf("advertised %s", key)
				}
			}
		}
	}
}
