package telemetry

import (
	"encoding/json"
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/privacy"
)

func sensitiveSnapshot() Snapshot {
	active, idle, load := true, uint64(42), 12.5
	return Snapshot{
		CPUPercent: &load, Timestamp: 1234, LocalIP: "192.0.2.1",
		UserIdleSeconds: &idle, UserActive: &active, SessionLocked: &active,
		Media:         &MediaInfo{Title: "private track", Artist: "private artist"},
		ActiveProcess: "private.exe", ActiveWindowTitle: "private document",
		MicrophoneInUse: &active, WebcamInUse: &active, WebcamActiveApp: "private camera",
	}
}

func assertSensitiveHidden(t *testing.T, snap Snapshot) {
	t.Helper()
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"user_idle_seconds", "user_active", "session_locked", "media", "active_process", "active_window_title", "microphone_in_use", "webcam_in_use", "webcam_active_app"} {
		if _, ok := fields[key]; ok {
			t.Errorf("sensitive field %s was exposed: %s", key, raw)
		}
	}
}

func TestPauseFiltersCachedSnapshotsWithoutChangingOriginal(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	original := sensitiveSnapshot().ApplyPrivacy()
	c := NewCollector()
	c.lastSnapshot = original
	privacy.Default.Pause(time.Minute)
	masked := c.Last()
	assertSensitiveHidden(t, masked)
	if masked.CPUPercent == nil || masked.LocalIP != original.LocalIP || masked.Timestamp != original.Timestamp {
		t.Fatal("pause removed non-sensitive readings")
	}
	if original.ActiveWindowTitle == "" || original.Media == nil {
		t.Fatal("filter changed another output's copy")
	}
	privacy.Default.Resume()
	assertSensitiveHidden(t, c.Last())
	if sensitiveSnapshot().ApplyPrivacy().Media == nil {
		t.Fatal("fresh readings should be allowed after resume")
	}
}

func TestPauseMasksCollectionAndKeepsSavedChoices(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	e := config.ExposeConfig{UserPresence: true, SessionLock: true, MediaControl: true, ActiveWindow: true, WindowTitle: true, Microphone: true, Webcam: true, Uptime: true}
	privacy.Default.Pause(time.Minute)
	snap := NewCollector().Collect(e)
	assertSensitiveHidden(t, snap)
	if snap.UptimeSeconds == nil || !e.Microphone || !e.WindowTitle {
		t.Fatal("normal collection and saved choices should remain available")
	}
}

func TestClearSensitiveDoesNotRestoreOldReadings(t *testing.T) {
	c := NewCollector()
	c.lastSnapshot = sensitiveSnapshot()
	c.ClearSensitive()
	assertSensitiveHidden(t, c.Last())
}
