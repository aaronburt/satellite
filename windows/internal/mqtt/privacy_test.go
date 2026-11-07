package mqtt

import (
	"encoding/json"
	"testing"
	"time"

	"satellite/internal/privacy"
	"satellite/internal/telemetry"
)

func TestPrivacyPauseFiltersMQTTAndBlocksThumbnails(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose.ScreenThumbnail = true
	c := startClient(t, cfg)
	load := 42.0
	snap := telemetry.Snapshot{CPUPercent: &load, ActiveWindowTitle: "private document", Media: &telemetry.MediaInfo{Title: "private track"}}
	privacy.Default.Pause(time.Minute)
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "paused telemetry", func() bool { return len(b.messages("sat/node1/state")) > 0 })
	var state telemetry.Snapshot
	messages := b.messages("sat/node1/state")
	if err := json.Unmarshal([]byte(messages[len(messages)-1].payload), &state); err != nil {
		t.Fatal(err)
	}
	if state.ActiveWindowTitle != "" || state.Media != nil || state.CPUPercent == nil {
		t.Fatalf("pause did not mask MQTT telemetry: %+v", state)
	}
	if err := c.PublishScreenThumbnail([]byte("private frame")); err != nil {
		t.Fatal(err)
	}
	if c.ScreenThumbnailEnabled() {
		t.Fatal("thumbnail reporting remained enabled while paused")
	}
	privacy.Default.Resume()
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "resumed telemetry", func() bool { return len(b.messages("sat/node1/state")) > 1 })
	messages = b.messages("sat/node1/state")
	if err := json.Unmarshal([]byte(messages[len(messages)-1].payload), &state); err != nil {
		t.Fatal(err)
	}
	if state.ActiveWindowTitle != "private document" || state.Media == nil {
		t.Fatalf("enabled telemetry did not resume: %+v", state)
	}
	if len(b.messages("sat/node1/screen_thumbnail")) != 0 {
		t.Fatal("thumbnail was sent while paused")
	}
}
