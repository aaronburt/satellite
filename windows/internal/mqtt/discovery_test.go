package mqtt

import (
	"encoding/json"
	"testing"

	"satellite/internal/capabilities"
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

func TestDiscoveryWithBatteryAndAllCapabilities(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NodeID = "battery-node"
	cfg.MQTT.TopicPrefix = "custom_prefix"
	cfg.Expose.Battery = true
	cfg.Expose.Storage = true
	cfg.Expose.GPU = true
	cfg.Expose.Microphone = true
	cfg.Expose.AudioOutput = true
	cfg.Expose.ActiveWindow = true
	cfg.Expose.WindowTitle = true
	cfg.Expose.SystemTheme = true
	cfg.Expose.Network = true
	cfg.Expose.RemoteLock = true
	cfg.Expose.MediaControl = true
	cfg.Expose.Webcam = true
	cfg.Expose.DisplayState = true
	cfg.Expose.Notifications = true
	cfg.Expose.UserPresence = true
	cfg.Expose.Fullscreen = true
	cfg.Expose.UpdatePending = true

	allCaps := capabilities.PlatformCapabilities{
		OS:            "windows",
		CPU:           true,
		Memory:        true,
		Storage:       true,
		Battery:       true,
		Wifi:          true,
		Webcam:        true,
		Microphone:    true,
		AudioOutput:   true,
		ActiveWindow:  true,
		DisplayState:  true,
		SessionLock:   true,
		SystemTheme:   true,
		Network:       true,
		GPU:           true,
		RemoteLock:    true,
		MediaControl:  true,
		Fullscreen:    true,
		UpdatePending: true,
		UserPresence:  true,
	}

	items := GetAllDiscoveryItems(cfg, true, allCaps)
	if len(items) == 0 {
		t.Fatal("expected discovery items with battery and all capabilities")
	}

	hasBattery := false
	for _, item := range items {
		if item.Key == "battery" {
			hasBattery = true
			if !item.ShouldRun {
				t.Error("expected battery ShouldRun to be true")
			}
		}
	}
	if !hasBattery {
		t.Fatal("expected battery discovery item")
	}
}

func TestDiscoveryWithUnsupportedCapabilities(t *testing.T) {
	cfg := config.DefaultConfig()
	unsupportedCaps := capabilities.PlatformCapabilities{
		OS: "linux",
	}

	items := GetAllDiscoveryItems(cfg, true, unsupportedCaps)
	for _, item := range items {
		if item.ShouldRun {
			t.Errorf("expected item %s to have ShouldRun false when all capabilities unsupported", item.Key)
		}
	}
}

func TestDiscoveryExposeToggles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Expose.Webcam = false
	cfg.Expose.DisplayState = false
	cfg.Expose.GPU = false
	cfg.Expose.CPU = false
	cfg.Expose.Memory = false
	cfg.Expose.Storage = false
	cfg.Expose.Battery = false
	cfg.Expose.Wifi = false
	cfg.Expose.Microphone = false
	cfg.Expose.AudioOutput = false
	cfg.Expose.ActiveWindow = false
	cfg.Expose.WindowTitle = false
	cfg.Expose.SystemTheme = false
	cfg.Expose.Network = false
	cfg.Expose.RemoteLock = false
	cfg.Expose.MediaControl = false
	cfg.Expose.UserPresence = false
	cfg.Expose.Fullscreen = false
	cfg.Expose.UpdatePending = false
	cfg.Expose.Uptime = false
	cfg.Expose.SessionLock = false
	cfg.Expose.LocalIP = false

	items := GetAllDiscoveryItems(cfg, false)
	for _, item := range items {
		if item.ShouldRun {
			t.Errorf("expected item %s ShouldRun to be false when all exposes are false", item.Key)
		}
	}
}
