package webui

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/telemetry"
)

func ptr[T any](v T) *T { return &v }

func allCaps() capabilities.PlatformCapabilities {
	return capabilities.PlatformCapabilities{
		OS: "windows", CPU: true, Memory: true, Storage: true, Network: true, Uptime: true,
		LocalIP: true, Battery: true, GPU: true, SessionLock: true, UserPresence: true,
		MediaControl: true, RemoteLock: true, Webcam: true, Microphone: true, DisplayState: true,
		AudioOutput: true, Wifi: true, Notifications: true, SystemTheme: true, Fullscreen: true,
		ActiveWindow: true, UpdatePending: true, ScreenThumbnail: true, ICUERGB: true,
	}
}

func allExpose() config.ExposeConfig {
	return config.ExposeConfig{
		CPU: true, Memory: true, Storage: true, Network: true, Uptime: true, UserPresence: true,
		SessionLock: true, Microphone: true, Fullscreen: true, SystemTheme: true, LocalIP: true,
		Battery: true, ActiveWindow: true, WindowTitle: true, UpdatePending: true, MediaControl: true,
		RemoteLock: true, Webcam: true, AudioOutput: true, Wifi: true, DisplayState: true,
		Notifications: true, GPU: true, NvidiaNVML: true, ScreenThumbnail: true, ICUERGB: true,
	}
}

func fullSnapshot() telemetry.Snapshot {
	return telemetry.Snapshot{
		CPUPercent:        ptr(42.46),
		MemoryUsedGB:      ptr(12.34),
		MemoryTotalGB:     ptr(31.9),
		MemoryPercent:     ptr(38.7),
		Drives:            []telemetry.DiskInfo{{Mount: "C:", UsedGB: 400, TotalGB: 476.9, UsedPercent: 83.9}, {Mount: "D:", UsedGB: 10, TotalGB: 1000, UsedPercent: 1}},
		NetworkRxKBs:      ptr(1536.0),
		NetworkTxKBs:      ptr(12.34),
		UptimeSeconds:     ptr(uint64(3*86400 + 4*3600 + 59)),
		UserIdleSeconds:   ptr(uint64(75)),
		UserActive:        ptr(true),
		SessionLocked:     ptr(false),
		MicrophoneInUse:   ptr(false),
		FullscreenActive:  ptr(false),
		SystemTheme:       "dark",
		LocalIP:           "192.168.1.20",
		HasBattery:        true,
		BatteryPercent:    ptr(80),
		PowerPlugged:      ptr(true),
		ActiveProcess:     "code.exe",
		ActiveWindowTitle: "sensors.go - satellite",
		UpdatePending:     ptr(false),
		Media:             &telemetry.MediaInfo{Status: "playing", Title: "Song", Artist: "Band", AppID: "Spotify.exe"},
		WebcamInUse:       ptr(false),
		AudioOutputName:   "Speakers",
		WifiSSID:          "home",
		WifiSignalPercent: ptr(72),
		DisplayPowered:    ptr(true),
		GPUs: []telemetry.GPUInfo{{
			Index: 0, Name: "RTX 4070", Vendor: "NVIDIA",
			CoreUsagePercent: ptr(55.0), MemoryUsedMB: ptr(4096.0), MemoryTotalMB: ptr(12288.0),
			MemoryPercent: ptr(33.3), TemperatureC: ptr(61.4), PowerWatts: ptr(120.4), FanSpeedPercent: ptr(40.0),
		}},
		Timestamp: 1000,
	}
}

var testNow = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

func byID(t *testing.T, list []Sensor, id string) Sensor {
	t.Helper()
	for _, s := range list {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("sensor %q is not listed; have %v", id, ids(list))
	return Sensor{}
}

func ids(list []Sensor) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.ID
	}
	return out
}

func has(list []Sensor, id string) bool {
	for _, s := range list {
		if s.ID == id {
			return true
		}
	}
	return false
}

func TestBuildSensorsListsEveryEnabledSensor(t *testing.T) {
	got := BuildSensors(allExpose(), allCaps(), fullSnapshot(), Frame{Have: true, At: testNow}, testNow)
	want := []string{
		"cpu", "memory", "gpu0", "disk:C:", "disk:D:", "uptime", "battery", "update_pending",
		"network", "local_ip", "wifi",
		"user_presence", "session_lock", "display_state", "fullscreen", "system_theme", "active_window", "window_title",
		"media_control", "audio_output", "microphone", "webcam",
		"screen_thumbnail", "icue_rgb",
	}
	if !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("sensors =\n%v\nwant\n%v", ids(got), want)
	}
}

// Every toggle in the config that reports a reading must show up. remote_lock
// and notifications are actions, and nvidia_nvml only changes how the GPU is
// read, so none of them is a sensor.
func TestEveryReportingToggleHasASensor(t *testing.T) {
	covers := map[string]string{
		"cpu": "cpu", "memory": "memory", "storage": "disk:C:", "network": "network", "uptime": "uptime",
		"user_presence": "user_presence", "session_lock": "session_lock", "microphone": "microphone",
		"fullscreen": "fullscreen", "system_theme": "system_theme", "local_ip": "local_ip", "battery": "battery",
		"active_window": "active_window", "window_title": "window_title", "update_pending": "update_pending",
		"media_control": "media_control", "webcam": "webcam", "audio_output": "audio_output", "wifi": "wifi",
		"display_state": "display_state", "gpu": "gpu0", "screen_thumbnail": "screen_thumbnail", "icue_rgb": "icue_rgb",
	}
	notSensors := map[string]bool{"remote_lock": true, "notifications": true, "nvidia_nvml": true}

	raw, _ := json.Marshal(config.ExposeConfig{})
	var all map[string]bool
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	for key := range all {
		if notSensors[key] {
			continue
		}
		if _, ok := covers[key]; !ok {
			t.Errorf("expose.%s has no sensor; add it to BuildSensors or to notSensors", key)
		}
	}

	list := BuildSensors(allExpose(), allCaps(), fullSnapshot(), Frame{Have: true, At: testNow}, testNow)
	for key, id := range covers {
		if _, ok := all[key]; !ok {
			t.Errorf("covers lists %q, which is not an expose key", key)
		}
		if !has(list, id) {
			t.Errorf("expose.%s is on but %q is not shown", key, id)
		}
	}
}

func TestDisabledSensorsAreNotListed(t *testing.T) {
	got := BuildSensors(config.ExposeConfig{CPU: true}, allCaps(), fullSnapshot(), Frame{}, testNow)
	if !reflect.DeepEqual(ids(got), []string{"cpu"}) {
		t.Fatalf("sensors = %v, want only cpu", ids(got))
	}
}

func TestNothingEnabledListsNothing(t *testing.T) {
	got := BuildSensors(config.ExposeConfig{}, allCaps(), fullSnapshot(), Frame{Have: true, At: testNow}, testNow)
	if len(got) != 0 {
		t.Fatalf("sensors = %v, want none", ids(got))
	}
	if b, _ := json.Marshal(got); string(b) != "[]" {
		t.Errorf("an empty list must encode as [] so the page can count it, got %s", b)
	}
}

func TestUnsupportedSensorsAreNotListed(t *testing.T) {
	linux := capabilities.PlatformCapabilities{OS: "linux", CPU: true, Memory: true}
	got := BuildSensors(allExpose(), linux, fullSnapshot(), Frame{Have: true, At: testNow}, testNow)
	if !reflect.DeepEqual(ids(got), []string{"cpu", "memory"}) {
		t.Fatalf("sensors = %v, want only what the platform supports", ids(got))
	}
}

func TestWindowTitleFollowsTheActiveWindowCapability(t *testing.T) {
	caps := capabilities.PlatformCapabilities{ActiveWindow: true}
	got := BuildSensors(config.ExposeConfig{WindowTitle: true}, caps, fullSnapshot(), Frame{}, testNow)
	if !has(got, "window_title") || has(got, "active_window") {
		t.Fatalf("sensors = %v, want the title alone", ids(got))
	}
	if has(BuildSensors(config.ExposeConfig{WindowTitle: true}, capabilities.PlatformCapabilities{}, fullSnapshot(), Frame{}, testNow), "window_title") {
		t.Fatal("title must not show without the capability")
	}
}

func TestEnabledSensorWithoutReadingShowsAsWaiting(t *testing.T) {
	// Collected once, but the platform could not read the CPU.
	snap := telemetry.Snapshot{Timestamp: 1000}
	got := byID(t, BuildSensors(config.ExposeConfig{CPU: true}, allCaps(), snap, Frame{}, testNow), "cpu")
	if got.State != StateWaiting || got.Value != "No reading" {
		t.Fatalf("cpu = %+v, want waiting with No reading", got)
	}
	if got.Percent != nil {
		t.Error("a missing reading must not draw a gauge")
	}
}

func TestBeforeTheFirstCollectionEverythingWaits(t *testing.T) {
	list := BuildSensors(allExpose(), allCaps(), telemetry.Snapshot{}, Frame{}, testNow)
	if len(list) == 0 {
		t.Fatal("enabled sensors must be listed before the first reading")
	}
	for _, s := range list {
		if s.State != StateWaiting {
			t.Errorf("%s state = %q before any reading, want waiting", s.ID, s.State)
		}
		if !strings.Contains(s.Value, "Waiting") {
			t.Errorf("%s value = %q, want a waiting message", s.ID, s.Value)
		}
	}
}

func TestNumericReadings(t *testing.T) {
	list := BuildSensors(allExpose(), allCaps(), fullSnapshot(), Frame{}, testNow)

	cpu := byID(t, list, "cpu")
	if cpu.Value != "42.5%" || cpu.Percent == nil || *cpu.Percent != 42.46 || cpu.Level != "" {
		t.Errorf("cpu = %+v", cpu)
	}
	mem := byID(t, list, "memory")
	if mem.Value != "39%" || mem.Detail != "12.3 / 31.9 GB" {
		t.Errorf("memory = %+v", mem)
	}
	disk := byID(t, list, "disk:C:")
	if disk.Label != "Disk (C:)" || disk.Value != "84%" || disk.Detail != "400.0 / 476.9 GB" || disk.Level != LevelMid {
		t.Errorf("disk C: = %+v", disk)
	}
	net := byID(t, list, "network")
	if net.Value != "Down 1.5 MB/s" || net.Detail != "Up 12.3 kB/s" {
		t.Errorf("network = %+v", net)
	}
	if up := byID(t, list, "uptime"); up.Value != "3d 4h" {
		t.Errorf("uptime = %q", up.Value)
	}
	wifi := byID(t, list, "wifi")
	if wifi.Value != "home" || wifi.Detail != "Signal 72%" || wifi.Percent == nil || *wifi.Percent != 72 {
		t.Errorf("wifi = %+v", wifi)
	}
}

func TestGPUShowsEveryAdapter(t *testing.T) {
	snap := fullSnapshot()
	snap.GPUs = append(snap.GPUs, telemetry.GPUInfo{Index: 1, Name: "Intel UHD", Vendor: "Intel"})
	list := BuildSensors(config.ExposeConfig{GPU: true}, allCaps(), snap, Frame{}, testNow)
	if !reflect.DeepEqual(ids(list), []string{"gpu0", "gpu1"}) {
		t.Fatalf("sensors = %v, want both adapters", ids(list))
	}
	g0 := byID(t, list, "gpu0")
	if g0.Label != "RTX 4070" || g0.Value != "55.0%" || g0.Detail != "VRAM 4.0 / 12.0 GB (33%) · 61°C · 120 W · Fan 40%" {
		t.Errorf("gpu0 = %+v", g0)
	}
	g1 := byID(t, list, "gpu1")
	if g1.Value != "Active" || g1.Detail != "" || g1.Percent != nil {
		t.Errorf("an adapter with only a name still shows: %+v", g1)
	}
}

func TestGPUWithNoAdapterIsSaidPlainly(t *testing.T) {
	snap := fullSnapshot()
	snap.GPUs = nil
	got := byID(t, BuildSensors(config.ExposeConfig{GPU: true}, allCaps(), snap, Frame{}, testNow), "gpu")
	if got.Value != "No GPU found" || got.State != StateIdle {
		t.Fatalf("gpu = %+v", got)
	}
}

func TestStorageWithNoDrivesIsSaidPlainly(t *testing.T) {
	snap := fullSnapshot()
	snap.Drives = nil
	got := byID(t, BuildSensors(config.ExposeConfig{Storage: true}, allCaps(), snap, Frame{}, testNow), "storage")
	if got.Value != "No drives found" || got.State != StateIdle {
		t.Fatalf("storage = %+v", got)
	}
}

func TestGaugeIsClampedToAValidPercent(t *testing.T) {
	snap := fullSnapshot()
	snap.CPUPercent = ptr(140.0)
	got := byID(t, BuildSensors(config.ExposeConfig{CPU: true}, allCaps(), snap, Frame{}, testNow), "cpu")
	if *got.Percent != 100 || got.Level != LevelHigh {
		t.Fatalf("cpu = %+v, want a full, high bar", got)
	}
	snap.CPUPercent = ptr(-3.0)
	got = byID(t, BuildSensors(config.ExposeConfig{CPU: true}, allCaps(), snap, Frame{}, testNow), "cpu")
	if *got.Percent != 0 {
		t.Fatalf("negative percent = %v, want 0", *got.Percent)
	}
}

func TestUsageLevels(t *testing.T) {
	cases := []struct {
		p    float64
		want string
	}{{0, ""}, {69.9, ""}, {70, LevelMid}, {89.9, LevelMid}, {90, LevelHigh}, {100, LevelHigh}}
	for _, c := range cases {
		if got := usageLevel(c.p); got != c.want {
			t.Errorf("usageLevel(%v) = %q, want %q", c.p, got, c.want)
		}
	}
}

// A full battery is good news, so the bar only warns as it runs low.
func TestBatteryLevelsRunTheOtherWay(t *testing.T) {
	cases := []struct {
		p    float64
		want string
	}{{100, ""}, {31, ""}, {30, LevelMid}, {16, LevelMid}, {15, LevelHigh}, {1, LevelHigh}}
	for _, c := range cases {
		if got := batteryLevel(c.p); got != c.want {
			t.Errorf("batteryLevel(%v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestBatterySensor(t *testing.T) {
	snap := fullSnapshot()
	got := byID(t, BuildSensors(config.ExposeConfig{Battery: true}, allCaps(), snap, Frame{}, testNow), "battery")
	if got.Value != "80%" || got.Detail != "Plugged in" || got.State != StateActive {
		t.Errorf("plugged in: %+v", got)
	}
	snap.PowerPlugged = ptr(false)
	snap.BatteryPercent = ptr(10)
	got = byID(t, BuildSensors(config.ExposeConfig{Battery: true}, allCaps(), snap, Frame{}, testNow), "battery")
	if got.Detail != "On battery" || got.Level != LevelHigh {
		t.Errorf("low on battery: %+v", got)
	}
}

func TestBatterySensorIsHiddenOnDesktops(t *testing.T) {
	snap := fullSnapshot()
	snap.HasBattery = false
	snap.BatteryPercent, snap.PowerPlugged = nil, nil
	if has(BuildSensors(config.ExposeConfig{Battery: true}, allCaps(), snap, Frame{}, testNow), "battery") {
		t.Fatal("a computer with no battery must not list one")
	}
}

func TestPrivacySensorsWarnWhileInUse(t *testing.T) {
	snap := fullSnapshot()
	snap.MicrophoneInUse = ptr(true)
	snap.WebcamInUse = ptr(true)
	snap.WebcamActiveApp = "zoom.exe"
	snap.SessionLocked = ptr(true)
	snap.UpdatePending = ptr(true)
	list := BuildSensors(allExpose(), allCaps(), snap, Frame{}, testNow)

	for _, c := range []struct{ id, value string }{
		{"microphone", "In use"}, {"webcam", "In use"}, {"session_lock", "Locked"}, {"update_pending", "Restart pending"},
	} {
		s := byID(t, list, c.id)
		if s.State != StateWarn || s.Value != c.value {
			t.Errorf("%s = %+v, want warn %q", c.id, s, c.value)
		}
	}
	if d := byID(t, list, "webcam").Detail; d != "zoom.exe" {
		t.Errorf("webcam detail = %q, want the app using it", d)
	}

	quiet := BuildSensors(allExpose(), allCaps(), fullSnapshot(), Frame{}, testNow)
	for _, id := range []string{"microphone", "webcam"} {
		if s := byID(t, quiet, id); s.State != StateIdle || s.Value != "Not in use" {
			t.Errorf("%s = %+v, want idle", id, s)
		}
	}
	for _, s := range quiet {
		if s.State == StateWarn {
			t.Errorf("%s warns on a quiet machine", s.ID)
		}
	}
}

func TestPresenceAndDisplay(t *testing.T) {
	snap := fullSnapshot()
	list := BuildSensors(allExpose(), allCaps(), snap, Frame{}, testNow)
	if u := byID(t, list, "user_presence"); u.Value != "Active" || u.Detail != "Idle for 1m 15s" || u.State != StateActive {
		t.Errorf("user = %+v", u)
	}
	snap.UserActive = ptr(false)
	snap.DisplayPowered = ptr(false)
	snap.FullscreenActive = ptr(true)
	list = BuildSensors(allExpose(), allCaps(), snap, Frame{}, testNow)
	if u := byID(t, list, "user_presence"); u.Value != "Away" || u.State != StateIdle {
		t.Errorf("user away = %+v", u)
	}
	if d := byID(t, list, "display_state"); d.Value != "Asleep" || d.State != StateIdle {
		t.Errorf("display = %+v", d)
	}
	if f := byID(t, list, "fullscreen"); f.Value != "Fullscreen app" {
		t.Errorf("fullscreen = %+v", f)
	}
}

func TestTextSensors(t *testing.T) {
	list := BuildSensors(allExpose(), allCaps(), fullSnapshot(), Frame{}, testNow)
	for id, want := range map[string]string{
		"system_theme": "Dark", "local_ip": "192.168.1.20", "active_window": "code.exe",
		"window_title": "sensors.go - satellite", "audio_output": "Speakers",
	} {
		if got := byID(t, list, id).Value; got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}

// After the first collection an empty string is a real answer, not a gap.
func TestEmptyTextReadingsAreAnswersNotGaps(t *testing.T) {
	snap := telemetry.Snapshot{Timestamp: 1000}
	ex := config.ExposeConfig{ActiveWindow: true, WindowTitle: true, AudioOutput: true, LocalIP: true, Wifi: true}
	list := BuildSensors(ex, allCaps(), snap, Frame{}, testNow)
	for id, want := range map[string]string{
		"active_window": "None", "window_title": "None", "audio_output": "None",
		"local_ip": "Not connected", "wifi": "Not connected",
	} {
		s := byID(t, list, id)
		if s.Value != want || s.State != StateIdle {
			t.Errorf("%s = %+v, want idle %q", id, s, want)
		}
	}
}

func TestMediaSensor(t *testing.T) {
	ex := config.ExposeConfig{MediaControl: true}
	snap := fullSnapshot()

	playing := byID(t, BuildSensors(ex, allCaps(), snap, Frame{}, testNow), "media_control")
	if playing.Value != "Song" || playing.Detail != "Playing · Band · Spotify.exe" || playing.State != StateActive {
		t.Errorf("playing = %+v", playing)
	}

	snap.Media = &telemetry.MediaInfo{Status: "paused", Title: "Song"}
	paused := byID(t, BuildSensors(ex, allCaps(), snap, Frame{}, testNow), "media_control")
	if paused.Detail != "Paused" || paused.State != StateOK {
		t.Errorf("paused = %+v", paused)
	}

	snap.Media = &telemetry.MediaInfo{Status: "playing"}
	if m := byID(t, BuildSensors(ex, allCaps(), snap, Frame{}, testNow), "media_control"); m.Value != "Unknown track" {
		t.Errorf("untitled = %+v", m)
	}

	for name, m := range map[string]*telemetry.MediaInfo{"nil": nil, "stopped": {Status: "stopped", Title: "old"}, "empty": {}} {
		got := byID(t, BuildSensors(ex, allCaps(), telemetry.Snapshot{Timestamp: 1000, Media: m}, Frame{}, testNow), "media_control")
		if got.Value != "Nothing playing" || got.State != StateIdle || got.Detail != "" {
			t.Errorf("%s media = %+v", name, got)
		}
	}
}

func TestScreenPreviewSensor(t *testing.T) {
	ex := config.ExposeConfig{ScreenThumbnail: true}

	waiting := byID(t, BuildSensors(ex, allCaps(), fullSnapshot(), Frame{}, testNow), "screen_thumbnail")
	if !waiting.Image || waiting.FrameTS != 0 || waiting.State != StateWaiting {
		t.Errorf("no frame yet = %+v", waiting)
	}

	at := testNow.Add(-7 * time.Second)
	live := byID(t, BuildSensors(ex, allCaps(), fullSnapshot(), Frame{Have: true, At: at}, testNow), "screen_thumbnail")
	if !live.Image || live.FrameTS != at.Unix() || live.Detail != "Updated 7s ago" || live.Value != "Live" {
		t.Errorf("with a frame = %+v", live)
	}
	if live.Group != GroupScreen {
		t.Errorf("group = %q", live.Group)
	}
}

func TestScreenPreviewNeedsTheCapability(t *testing.T) {
	caps := allCaps()
	caps.ScreenThumbnail = false
	got := BuildSensors(config.ExposeConfig{ScreenThumbnail: true}, caps, fullSnapshot(), Frame{Have: true, At: testNow}, testNow)
	if len(got) != 0 {
		t.Fatalf("sensors = %v, want none on a platform that cannot capture", ids(got))
	}
}

func TestIDsAreUnique(t *testing.T) {
	snap := fullSnapshot()
	snap.GPUs = append(snap.GPUs, telemetry.GPUInfo{Index: 1, Name: "Second"})
	seen := map[string]bool{}
	for _, s := range BuildSensors(allExpose(), allCaps(), snap, Frame{Have: true, At: testNow}, testNow) {
		if seen[s.ID] {
			t.Errorf("duplicate sensor id %q: the page keys cards by id", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestEverySensorHasLabelGroupAndState(t *testing.T) {
	groups := map[string]bool{GroupSystem: true, GroupNetwork: true, GroupSession: true, GroupDevices: true, GroupScreen: true}
	states := map[string]bool{StateOK: true, StateActive: true, StateWarn: true, StateIdle: true, StateWaiting: true}
	for _, s := range BuildSensors(allExpose(), allCaps(), fullSnapshot(), Frame{Have: true, At: testNow}, testNow) {
		if s.Label == "" || s.Value == "" {
			t.Errorf("%s has no label or value: %+v", s.ID, s)
		}
		if !groups[s.Group] {
			t.Errorf("%s group %q is not one the page knows", s.ID, s.Group)
		}
		if !states[s.State] {
			t.Errorf("%s state %q is not one the page styles", s.ID, s.State)
		}
		if s.Level != "" && s.Percent == nil {
			t.Errorf("%s has a level but no gauge", s.ID)
		}
	}
}

func TestSensorJSONOmitsEmptyOptionalFields(t *testing.T) {
	b, err := json.Marshal(Sensor{ID: "x", Group: GroupSystem, Label: "X", Value: "1", State: StateOK})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"detail", "percent", "level", "image", "frame_ts"} {
		if _, ok := m[k]; ok {
			t.Errorf("%q should be omitted when empty: %s", k, b)
		}
	}
	// The page tells a gauge apart by "percent" being present, so a real 0%
	// must still be sent.
	zero := 0.0
	b, _ = json.Marshal(Sensor{ID: "x", Percent: &zero})
	if !strings.Contains(string(b), `"percent":0`) {
		t.Errorf("a 0%% gauge must be sent: %s", b)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[uint64]string{
		0: "0s", 59: "59s", 60: "1m 0s", 3599: "59m 59s", 3600: "1h 0m", 5*3600 + 12*60 + 9: "5h 12m",
		86399: "23h 59m", 86400: "1d 0h", 3*86400 + 4*3600 + 59: "3d 4h",
	}
	for in, want := range cases {
		if got := formatDuration(in); got != want {
			t.Errorf("formatDuration(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatRate(t *testing.T) {
	cases := map[float64]string{0: "0.0 kB/s", 0.64: "0.6 kB/s", 1023.9: "1023.9 kB/s", 1024: "1.0 MB/s", 2560: "2.5 MB/s"}
	for in, want := range cases {
		if got := formatRate(in); got != want {
			t.Errorf("formatRate(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatAge(t *testing.T) {
	cases := map[time.Duration]string{
		-5 * time.Second: "just now", 0: "just now", 1500 * time.Millisecond: "just now",
		2 * time.Second: "2s ago", 119 * time.Second: "119s ago", 120 * time.Second: "2m ago",
		59 * time.Minute: "59m ago", 2 * time.Hour: "2h ago", 30 * time.Hour: "30h ago",
	}
	for in, want := range cases {
		if got := formatAge(in); got != want {
			t.Errorf("formatAge(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestTitleCase(t *testing.T) {
	for in, want := range map[string]string{"": "", "dark": "Dark", "LIGHT": "Light", "playing": "Playing"} {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}
