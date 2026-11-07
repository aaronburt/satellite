package mqtt

import (
	"encoding/json"
	"testing"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/icue"
	"satellite/internal/privacy"
	"satellite/internal/telemetry"
)

func TestRGBDiscoveryRequiresSupportAndOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, supported := range []bool{false, true} {
			cfg := config.DefaultConfig()
			cfg.Expose.ICUERGB = enabled
			found := false
			for _, item := range GetAllDiscoveryItems(cfg, false, capabilities.PlatformCapabilities{ICUERGB: supported}) {
				if item.Key != "icue_rgb" {
					continue
				}
				found = true
				if item.ShouldRun != (enabled && supported) {
					t.Fatal("RGB discovery ignored support or opt-in")
				}
				if !item.ShouldRun {
					continue
				}
				var payload EntityDiscoveryPayload
				if err := json.Unmarshal(item.Payload, &payload); err != nil {
					t.Fatal(err)
				}
				if payload.CommandTopic != "" || payload.JSONAttributesTemplate == "" {
					t.Fatal("RGB must expose read-only attributes")
				}
			}
			if !found {
				t.Fatal("missing RGB discovery item")
			}
		}
	}
}

func TestRGBPublishedAndClearedOnPause(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose.ICUERGB = true
	c := startClient(t, cfg)
	snap := telemetry.Snapshot{RGB: &icue.State{Status: "connected", Devices: []icue.Device{{ID: "keyboard", Model: "Keyboard", LEDs: []icue.LED{{ID: 1, R: 255, A: 255}}}}}}
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "RGB publish", func() bool { return len(b.messages("sat/node1/state")) == 1 })
	var state telemetry.Snapshot
	if err := json.Unmarshal([]byte(b.messages("sat/node1/state")[0].payload), &state); err != nil {
		t.Fatal(err)
	}
	if state.RGB == nil || state.RGB.Devices[0].LEDs[0].R != 255 {
		t.Fatal("MQTT lost LED colours")
	}
	privacy.Default.Pause(time.Minute)
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "RGB pause", func() bool { return len(b.messages("sat/node1/state")) == 2 })
	state = telemetry.Snapshot{}
	if err := json.Unmarshal([]byte(b.messages("sat/node1/state")[1].payload), &state); err != nil {
		t.Fatal(err)
	}
	if state.RGB != nil {
		t.Fatal("MQTT exposed paused RGB")
	}
}
