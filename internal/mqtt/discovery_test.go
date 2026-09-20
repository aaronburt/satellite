package mqtt

import (
	"encoding/json"
	"testing"

	"satellite/internal/config"
)

func TestGetAllDiscoveryItems(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NodeID = "testbox"

	items := GetAllDiscoveryItems(cfg, false)
	if len(items) == 0 {
		t.Fatalf("expected discovery items, got 0")
	}

	foundKeys := make(map[string]bool)
	for _, item := range items {
		foundKeys[item.Key] = true
		if item.ShouldRun {
			if len(item.Payload) == 0 {
				t.Errorf("expected non-empty payload for key %q", item.Key)
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(item.Payload, &parsed); err != nil {
				t.Errorf("failed to parse discovery json for key %q: %v", item.Key, err)
			}
		}
	}

	requiredKeys := []string{
		"webcam",
		"audio_output",
		"wifi_ssid",
		"wifi_signal",
		"display_state",
		"display_sleep",
		"cpu",
		"memory_percent",
		"local_ip",
		"uptime",
		"session_locked",
	}

	for _, k := range requiredKeys {
		if !foundKeys[k] {
			t.Errorf("missing expected discovery item key: %s", k)
		}
	}
}

func TestDiscoveryExposeToggles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Expose.Webcam = false
	cfg.Expose.DisplayState = false
	cfg.Expose.GPU = false

	items := GetAllDiscoveryItems(cfg, false)
	for _, item := range items {
		if item.Key == "webcam" && item.ShouldRun {
			t.Errorf("expected webcam discovery ShouldRun to be false")
		}
		if item.Key == "display_state" && item.ShouldRun {
			t.Errorf("expected display_state discovery ShouldRun to be false")
		}
		if item.Key == "display_sleep" && item.ShouldRun {
			t.Errorf("expected display_sleep discovery ShouldRun to be false")
		}
		if (item.Key == "gpu_0_usage" || item.Key == "gpu_0_mem_pct") && item.ShouldRun {
			t.Errorf("expected gpu discovery to not run when Expose.GPU is false")
		}
	}
}
