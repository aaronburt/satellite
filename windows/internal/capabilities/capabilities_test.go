package capabilities

import "testing"

func TestDetectCapabilities(t *testing.T) {
	caps := Detect()
	if caps.OS == "" {
		t.Errorf("expected non-empty OS")
	}
	if !caps.CPU {
		t.Errorf("expected CPU capability to be true")
	}
	if !caps.Memory {
		t.Errorf("expected Memory capability to be true")
	}
	if !caps.Storage {
		t.Errorf("expected Storage capability to be true")
	}
	if !caps.Network {
		t.Errorf("expected Network capability to be true")
	}
	if !caps.Uptime {
		t.Errorf("expected Uptime capability to be true")
	}
	if !caps.LocalIP {
		t.Errorf("expected LocalIP capability to be true")
	}
}

func TestIsSupported(t *testing.T) {
	caps := PlatformCapabilities{
		OS:            "test",
		CPU:           true,
		Memory:        true,
		Storage:       true,
		Network:       true,
		Uptime:        true,
		LocalIP:       true,
		Battery:       true,
		GPU:           true,
		SessionLock:   true,
		UserPresence:  true,
		MediaControl:  true,
		RemoteLock:    true,
		Webcam:        true,
		Microphone:    true,
		DisplayState:  true,
		AudioOutput:   true,
		Wifi:          true,
		Notifications: true,
		SystemTheme:   true,
		Fullscreen:    true,
		ActiveWindow:  true,
		UpdatePending: true,
	}

	keys := []string{
		"cpu", "memory", "storage", "network", "uptime", "local_ip",
		"battery", "gpu", "session_lock", "user_presence", "media_control",
		"remote_lock", "webcam", "microphone", "display_state", "audio_output",
		"wifi", "notifications", "system_theme", "fullscreen", "active_window",
		"update_pending",
	}

	for _, k := range keys {
		if !caps.IsSupported(k) {
			t.Errorf("expected %s to be supported", k)
		}
	}

	emptyCaps := PlatformCapabilities{}
	for _, k := range keys {
		if emptyCaps.IsSupported(k) {
			t.Errorf("expected %s to not be supported", k)
		}
	}

	if caps.IsSupported("non_existent_feature") {
		t.Errorf("expected non_existent_feature to return false")
	}
}
