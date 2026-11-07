package webui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/icue"
	"satellite/internal/telemetry"
)

func TestRGBSettingRoundTrip(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())
	for _, enabled := range []bool{true, false} {
		payload := map[string]any{"expose": config.ExposeConfig{ICUERGB: enabled}}
		resp, body := apiRequest(t, "POST", base+"/api/config", token, payload)
		if resp.StatusCode != 200 {
			t.Fatalf("save: %d %s", resp.StatusCode, body)
		}
		resp, body = apiRequest(t, "GET", base+"/api/config", token, nil)
		var cfg config.Config
		if resp.StatusCode != 200 {
			t.Fatalf("load: %d %s", resp.StatusCode, body)
		}
		if err := json.Unmarshal(body, &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.Expose.ICUERGB != enabled || config.Get().Expose.ICUERGB != enabled {
			t.Fatal("RGB setting was not preserved")
		}
	}
}

func TestRGBSensorShowsConnectionAndMissingSDK(t *testing.T) {
	for _, status := range []string{"connected", "sdk_missing", "access_denied", "disconnected", "partial"} {
		snap := telemetry.Snapshot{Timestamp: 1, RGB: &icue.State{Status: status, Devices: []icue.Device{{LEDs: []icue.LED{{ID: 1}}}}}}
		sensors := BuildSensors(config.ExposeConfig{ICUERGB: true}, capabilities.PlatformCapabilities{ICUERGB: true}, snap, Frame{}, time.Now())
		if len(sensors) != 1 {
			t.Fatalf("missing RGB sensor for %s", status)
		}
		if status == "connected" && (!strings.Contains(sensors[0].Value, "1 RGB devices") || !strings.Contains(sensors[0].Detail, "1 LEDs")) {
			t.Fatal("missing device and LED counts")
		}
		if status == "sdk_missing" && sensors[0].Value != "SDK not installed" {
			t.Fatal("missing SDK was not explained")
		}
	}
}
