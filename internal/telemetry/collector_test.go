package telemetry

import (
	"testing"

	"satellite/internal/config"
)

func TestHasSignificantDelta(t *testing.T) {
	prev := Snapshot{}
	curr := Snapshot{}

	if HasSignificantDelta(prev, curr) {
		t.Errorf("expected no delta for empty snapshots")
	}

	active := true
	inactive := false
	curr.WebcamInUse = &active
	if !HasSignificantDelta(prev, curr) {
		t.Errorf("expected delta when webcam in use becomes active")
	}

	prev.WebcamInUse = &active
	curr.WebcamInUse = &inactive
	if !HasSignificantDelta(prev, curr) {
		t.Errorf("expected delta when webcam in use changes value")
	}

	prev.WebcamInUse = &active
	curr.WebcamInUse = &active
	prev.WebcamActiveApp = "Zoom"
	curr.WebcamActiveApp = "Teams"
	if !HasSignificantDelta(prev, curr) {
		t.Errorf("expected delta when webcam active app changes")
	}

	prev.WebcamActiveApp = "Teams"
	prev.AudioOutputName = "Speakers"
	curr.AudioOutputName = "Headphones"
	if !HasSignificantDelta(prev, curr) {
		t.Errorf("expected delta when audio output device changes")
	}

	prev.AudioOutputName = "Headphones"
	prev.WifiSSID = "Home-5G"
	curr.WifiSSID = "Office-Guest"
	if !HasSignificantDelta(prev, curr) {
		t.Errorf("expected delta when wifi ssid changes")
	}

	prev.WifiSSID = "Office-Guest"
	dispOn := true
	dispOff := false
	prev.DisplayPowered = &dispOn
	curr.DisplayPowered = &dispOff
	if !HasSignificantDelta(prev, curr) {
		t.Errorf("expected delta when display power changes")
	}
}

func TestTelemetryWin32Functions(t *testing.T) {
	theme := GetWindowsTheme()
	if theme != "dark" && theme != "light" {
		t.Errorf("unexpected theme: %s", theme)
	}

	uptime := GetSystemUptimeSeconds()
	if uptime == 0 {
		t.Errorf("expected uptime > 0")
	}

	drives := GetAllDrives()
	if len(drives) == 0 {
		t.Errorf("expected at least one drive")
	}

	ip := GetLocalIPv4()
	if ip == "" {
		t.Logf("local ip is empty (may be offline)")
	}

	_ = IsSessionLocked()
	_ = IsMicrophoneInUse()
	_ = IsFullscreenActive()
	_ = IsRebootPending()
	_ = IsDisplayPowered()
	_ = GetActiveAudioOutputDevice()

	inUse, app := GetWebcamStatus()
	t.Logf("Webcam in use: %v (app: %s)", inUse, app)

	ssid, sig := GetWifiStatus()
	t.Logf("WiFi SSID: %s (sig: %v)", ssid, sig)
}

func TestCollectorCollect(t *testing.T) {
	c := NewCollector()
	cfg := config.DefaultExpose()
	snap := c.Collect(cfg)

	if snap.Timestamp == 0 {
		t.Errorf("expected non-zero timestamp")
	}
	if snap.WindowsTheme == "" {
		t.Errorf("expected non-empty windows theme")
	}

	last := c.Last()
	if last.Timestamp != snap.Timestamp {
		t.Errorf("expected Last() timestamp to match collected snapshot")
	}
}
