package main

import (
	"bytes"
	"strings"
	"testing"

	"satellite/internal/config"
	"satellite/internal/telemetry"
	"satellite/internal/updater"
)

func f64(v float64) *float64 { return &v }
func u64(v uint64) *uint64   { return &v }
func boolp(v bool) *bool     { return &v }
func intp(v int) *int        { return &v }

func TestRenderCLIShowsEveryReading(t *testing.T) {
	snap := telemetry.Snapshot{
		CPUPercent:    f64(12.34),
		MemoryUsedGB:  f64(7.5),
		MemoryTotalGB: f64(16),
		MemoryPercent: f64(46.9),
		GPUs: []telemetry.GPUInfo{
			{Index: 0, Name: "RTX 4090", CoreUsagePercent: f64(55), MemoryUsedMB: f64(2048), MemoryTotalMB: f64(24576), MemoryPercent: f64(8.3), TemperatureC: f64(61)},
			{Index: 1},
		},
		Drives:            []telemetry.DiskInfo{{Mount: "C:", UsedGB: 100, TotalGB: 500, UsedPercent: 20}},
		NetworkRxKBs:      f64(1.5),
		NetworkTxKBs:      f64(2.5),
		LocalIP:           "192.168.1.20",
		UptimeSeconds:     u64(7300),
		UserIdleSeconds:   u64(400),
		UserActive:        boolp(false),
		SessionLocked:     boolp(true),
		MicrophoneInUse:   boolp(true),
		WebcamInUse:       boolp(true),
		WebcamActiveApp:   "zoom",
		FullscreenActive:  boolp(false),
		SystemTheme:       "dark",
		AudioOutputName:   "Speakers",
		WifiSSID:          "HomeNet",
		WifiSignalPercent: intp(80),
		DisplayPowered:    boolp(false),
		HasBattery:        true,
		BatteryPercent:    intp(77),
		PowerPlugged:      boolp(true),
		ActiveProcess:     "code.exe",
		ActiveWindowTitle: "main.go",
		UpdatePending:     boolp(true),
		Media:             &telemetry.MediaInfo{Status: "playing", Title: "Song", Artist: "Band", AppID: "spotify"},
	}
	update := &updater.CheckStatus{Available: true, LatestVersion: "9.9.9", ReleaseURL: "https://example/rel"}

	var buf bytes.Buffer
	renderCLI(&buf, snap, update)
	out := buf.String()

	want := []string{
		"SATELLITE AGENT (v" + config.Version + " CLI)",
		"Update       : v9.9.9 available! (https://example/rel)",
		"Processor    :   12.3 %",
		"Memory (RAM) :   7.50 /  16.00 GB ( 46.9 %)",
		"GPU (RTX 4090) :  55.0 % | VRAM:  2.0 / 24.0 GB ( 8.3 %) | 61 °C",
		"GPU (GPU 1) : ",
		"Storage (C:) :  100.0 /  500.0 GB ( 20.0 %)",
		"Network I/O  : ↓     1.5 kB/s | ↑     2.5 kB/s",
		"Local IP     : 192.168.1.20",
		"Uptime       : 2 hours",
		"User Status  : Idle (400 sec idle)",
		"Screen Lock  : LOCKED",
		"Microphone   : ACTIVE (On Air)",
		"Webcam       : ACTIVE (On Air) [zoom]",
		"Fullscreen   : No",
		"Theme        : dark",
		"Audio Output : Speakers",
		"Wi-Fi / Net  : HomeNet (80%)",
		"Display      : Standby / Asleep",
		"Battery      : 77 % (AC Power)",
		"Active App   : code.exe",
		"Window Title : main.go",
		"Reboot Req.  : YES (Restart Required)",
		"Media Player : playing - Song (Band) [spotify]",
		"Press Ctrl+C to exit",
	}
	for _, line := range want {
		if !strings.Contains(out, line) {
			t.Errorf("output missing %q\n%s", line, out)
		}
	}
}

func TestRenderCLIOmitsMissingReadings(t *testing.T) {
	var buf bytes.Buffer
	renderCLI(&buf, telemetry.Snapshot{HasBattery: false, BatteryPercent: intp(50), PowerPlugged: boolp(false)}, &updater.CheckStatus{Available: false})
	out := buf.String()
	for _, label := range []string{"Update", "Processor", "Memory", "GPU", "Storage", "Network", "Battery", "Media", "Webcam", "Wi-Fi"} {
		if strings.Contains(out, label) {
			t.Errorf("empty snapshot should not show %q:\n%s", label, out)
		}
	}
	if !strings.Contains(out, "Press Ctrl+C to exit") {
		t.Error("footer missing")
	}

	buf.Reset()
	renderCLI(&buf, telemetry.Snapshot{UserActive: boolp(true), UserIdleSeconds: u64(3), SessionLocked: boolp(false),
		MicrophoneInUse: boolp(false), WebcamInUse: boolp(false), DisplayPowered: boolp(true),
		HasBattery: true, BatteryPercent: intp(10), PowerPlugged: boolp(false), UpdatePending: boolp(false),
		Media: &telemetry.MediaInfo{Status: "idle"}}, nil)
	out = buf.String()
	for _, line := range []string{"User Status  : Active (3 sec idle)", "Screen Lock  : Unlocked", "Microphone   : Inactive",
		"Webcam       : Inactive", "Display      : Powered On", "Battery      : 10 % (Battery)", "Reboot Req.  : No", "Media Player : idle"} {
		if !strings.Contains(out, line) {
			t.Errorf("output missing %q\n%s", line, out)
		}
	}
}
