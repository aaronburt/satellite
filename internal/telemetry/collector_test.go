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

	prev.DisplayPowered = &dispOn
	curr.DisplayPowered = &dispOn
	usage1 := 10.0
	usage2 := 20.0
	prev.GPUs = []GPUInfo{{Index: 0, CoreUsagePercent: &usage1}}
	curr.GPUs = []GPUInfo{{Index: 0, CoreUsagePercent: &usage2}}
	if !HasSignificantDelta(prev, curr) {
		t.Errorf("expected delta when GPU usage changes")
	}

	c1 := 10.0
	c2 := 12.0
	if !HasSignificantDelta(Snapshot{CPUPercent: &c1}, Snapshot{CPUPercent: &c2}) {
		t.Errorf("expected delta when CPU changes by >= 1%%")
	}
	if !HasSignificantDelta(Snapshot{CPUPercent: &c1}, Snapshot{}) {
		t.Errorf("expected delta when CPU presence changes")
	}

	m1 := 50.0
	m2 := 51.0
	if !HasSignificantDelta(Snapshot{MemoryPercent: &m1}, Snapshot{MemoryPercent: &m2}) {
		t.Errorf("expected delta when Memory changes")
	}
	if !HasSignificantDelta(Snapshot{MemoryPercent: &m1}, Snapshot{}) {
		t.Errorf("expected delta when Memory presence changes")
	}

	rx1 := 10.0
	rx2 := 35.0
	if !HasSignificantDelta(Snapshot{NetworkRxKBs: &rx1}, Snapshot{NetworkRxKBs: &rx2}) {
		t.Errorf("expected delta when NetworkRx changes")
	}
	if !HasSignificantDelta(Snapshot{NetworkRxKBs: &rx1}, Snapshot{}) {
		t.Errorf("expected delta when NetworkRx presence changes")
	}

	tx1 := 10.0
	tx2 := 35.0
	if !HasSignificantDelta(Snapshot{NetworkTxKBs: &tx1}, Snapshot{NetworkTxKBs: &tx2}) {
		t.Errorf("expected delta when NetworkTx changes")
	}
	if !HasSignificantDelta(Snapshot{NetworkTxKBs: &tx1}, Snapshot{}) {
		t.Errorf("expected delta when NetworkTx presence changes")
	}

	tTrue := true
	tFalse := false
	if !HasSignificantDelta(Snapshot{UserActive: &tTrue}, Snapshot{UserActive: &tFalse}) {
		t.Errorf("expected delta when UserActive changes")
	}
	if !HasSignificantDelta(Snapshot{UserActive: &tTrue}, Snapshot{}) {
		t.Errorf("expected delta when UserActive presence changes")
	}

	if !HasSignificantDelta(Snapshot{SessionLocked: &tTrue}, Snapshot{SessionLocked: &tFalse}) {
		t.Errorf("expected delta when SessionLocked changes")
	}
	if !HasSignificantDelta(Snapshot{SessionLocked: &tTrue}, Snapshot{}) {
		t.Errorf("expected delta when SessionLocked presence changes")
	}

	if !HasSignificantDelta(Snapshot{MicrophoneInUse: &tTrue}, Snapshot{MicrophoneInUse: &tFalse}) {
		t.Errorf("expected delta when MicrophoneInUse changes")
	}
	if !HasSignificantDelta(Snapshot{MicrophoneInUse: &tTrue}, Snapshot{}) {
		t.Errorf("expected delta when MicrophoneInUse presence changes")
	}

	if !HasSignificantDelta(Snapshot{FullscreenActive: &tTrue}, Snapshot{FullscreenActive: &tFalse}) {
		t.Errorf("expected delta when FullscreenActive changes")
	}
	if !HasSignificantDelta(Snapshot{FullscreenActive: &tTrue}, Snapshot{}) {
		t.Errorf("expected delta when FullscreenActive presence changes")
	}

	if !HasSignificantDelta(Snapshot{SystemTheme: "dark"}, Snapshot{SystemTheme: "light"}) {
		t.Errorf("expected delta when SystemTheme changes")
	}

	if !HasSignificantDelta(Snapshot{LocalIP: "1.1.1.1"}, Snapshot{LocalIP: "1.1.1.2"}) {
		t.Errorf("expected delta when LocalIP changes")
	}

	if !HasSignificantDelta(Snapshot{PowerPlugged: &tTrue}, Snapshot{PowerPlugged: &tFalse}) {
		t.Errorf("expected delta when PowerPlugged changes")
	}
	if !HasSignificantDelta(Snapshot{PowerPlugged: &tTrue}, Snapshot{}) {
		t.Errorf("expected delta when PowerPlugged presence changes")
	}

	if !HasSignificantDelta(Snapshot{ActiveProcess: "app1"}, Snapshot{ActiveProcess: "app2"}) {
		t.Errorf("expected delta when ActiveProcess changes")
	}

	if !HasSignificantDelta(Snapshot{ActiveWindowTitle: "win1"}, Snapshot{ActiveWindowTitle: "win2"}) {
		t.Errorf("expected delta when ActiveWindowTitle changes")
	}

	if !HasSignificantDelta(Snapshot{UpdatePending: &tTrue}, Snapshot{UpdatePending: &tFalse}) {
		t.Errorf("expected delta when UpdatePending changes")
	}
	if !HasSignificantDelta(Snapshot{UpdatePending: &tTrue}, Snapshot{}) {
		t.Errorf("expected delta when UpdatePending presence changes")
	}

	med1 := MediaInfo{Status: "playing", Title: "Song 1", Artist: "Artist 1", AppID: "Spotify"}
	med2 := MediaInfo{Status: "paused", Title: "Song 1", Artist: "Artist 1", AppID: "Spotify"}
	med3 := MediaInfo{Status: "playing", Title: "Song 2", Artist: "Artist 1", AppID: "Spotify"}
	med4 := MediaInfo{Status: "playing", Title: "Song 1", Artist: "Artist 2", AppID: "Spotify"}
	med5 := MediaInfo{Status: "playing", Title: "Song 1", Artist: "Artist 1", AppID: "VLC"}

	if !HasSignificantDelta(Snapshot{Media: &med1}, Snapshot{Media: &med2}) {
		t.Errorf("expected delta when media status changes")
	}
	if !HasSignificantDelta(Snapshot{Media: &med1}, Snapshot{Media: &med3}) {
		t.Errorf("expected delta when media title changes")
	}
	if !HasSignificantDelta(Snapshot{Media: &med1}, Snapshot{Media: &med4}) {
		t.Errorf("expected delta when media artist changes")
	}
	if !HasSignificantDelta(Snapshot{Media: &med1}, Snapshot{Media: &med5}) {
		t.Errorf("expected delta when media app_id changes")
	}
	if !HasSignificantDelta(Snapshot{Media: &med1}, Snapshot{}) {
		t.Errorf("expected delta when media presence changes")
	}
	if !HasSignificantDelta(Snapshot{}, Snapshot{Media: &med1}) {
		t.Errorf("expected delta when media presence becomes non-nil")
	}

	if !HasSignificantDelta(Snapshot{GPUs: []GPUInfo{{Index: 0}}}, Snapshot{}) {
		t.Errorf("expected delta when GPU slice length changes")
	}

	gMem1 := 10.0
	gMem2 := 12.0
	if !HasSignificantDelta(Snapshot{GPUs: []GPUInfo{{MemoryPercent: &gMem1}}}, Snapshot{GPUs: []GPUInfo{{MemoryPercent: &gMem2}}}) {
		t.Errorf("expected delta when GPU MemoryPercent changes")
	}
	if !HasSignificantDelta(Snapshot{GPUs: []GPUInfo{{MemoryPercent: &gMem1}}}, Snapshot{GPUs: []GPUInfo{{}}}) {
		t.Errorf("expected delta when GPU MemoryPercent presence changes")
	}

	gTemp1 := 40.0
	gTemp2 := 42.0
	if !HasSignificantDelta(Snapshot{GPUs: []GPUInfo{{TemperatureC: &gTemp1}}}, Snapshot{GPUs: []GPUInfo{{TemperatureC: &gTemp2}}}) {
		t.Errorf("expected delta when GPU TemperatureC changes")
	}
	if !HasSignificantDelta(Snapshot{GPUs: []GPUInfo{{TemperatureC: &gTemp1}}}, Snapshot{GPUs: []GPUInfo{{}}}) {
		t.Errorf("expected delta when GPU TemperatureC presence changes")
	}
}

func TestTelemetryWin32Functions(t *testing.T) {
	theme := GetSystemTheme()
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
	cfg.SystemTheme = true
	snap := c.Collect(cfg)

	if snap.Timestamp == 0 {
		t.Errorf("expected non-zero timestamp")
	}
	if snap.SystemTheme == "" {
		t.Errorf("expected non-empty system theme")
	}

	last := c.Last()
	if last.Timestamp != snap.Timestamp {
		t.Errorf("expected Last() timestamp to match collected snapshot")
	}

	snap2 := c.Collect(cfg)
	if snap2.Timestamp == 0 {
		t.Errorf("expected non-zero timestamp on second collect")
	}

	emptySnap := c.Collect(config.ExposeConfig{})
	if emptySnap.CPUPercent != nil {
		t.Errorf("expected nil CPUPercent when CPU expose is false")
	}
	if emptySnap.MemoryPercent != nil {
		t.Errorf("expected nil MemoryPercent when Memory expose is false")
	}

	TrimWorkingSet()
	_ = IsMicrophoneInUse()
	_ = IsFullscreenActive()
	_ = IsRebootPending()
	_ = GetActiveMediaInfo()
	_, _ = GetActiveWindowAndProcess()
	_, _ = GetUserIdleSeconds()
	_ = IsSessionLocked()
	_ = GetLocalIPv4()
	_, _, _ = GetBatteryAndPower()
}

func TestGPUInfo(t *testing.T) {
	gpus := GetGPUInfo()
	t.Logf("Detected %d GPU(s)", len(gpus))
	for _, g := range gpus {
		t.Logf("GPU [%d]: %s (Vendor: %s)", g.Index, g.Name, g.Vendor)
		if g.CoreUsagePercent != nil {
			t.Logf("  Core: %.1f%%", *g.CoreUsagePercent)
		}
		if g.MemoryUsedMB != nil && g.MemoryTotalMB != nil {
			t.Logf("  VRAM: %.0f / %.0f MB", *g.MemoryUsedMB, *g.MemoryTotalMB)
		}
		if g.TemperatureC != nil {
			t.Logf("  Temp: %.0f C", *g.TemperatureC)
		}
	}
}
