package mqtt

import (
	"encoding/json"
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/privacy"
	"satellite/internal/telemetry"
)

func TestDeviceSummaryDiscoveryIsOptional(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NodeID = "desktop"
	item := findItem(t, GetAllDiscoveryItems(cfg, false, allCaps()), "device_summary")
	if item.ShouldRun || len(item.Payload) != 0 {
		t.Fatal("summary enabled by default")
	}
	cfg.Expose.DeviceSummary = true
	item = findItem(t, GetAllDiscoveryItems(cfg, false, allCaps()), "device_summary")
	var payload map[string]any
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if !item.ShouldRun || payload["name"] != nil || payload["unique_id"] != "desktop_device_summary" {
		t.Fatalf("invalid summary discovery: %s", item.Payload)
	}
	if payload["state_topic"] != "satellite/desktop/state" || payload["json_attributes_topic"] != payload["state_topic"] {
		t.Fatal("summary must reuse telemetry")
	}
	if payload["availability_topic"] != "satellite/desktop/status" {
		t.Fatal("missing availability")
	}
	cfg.Expose.CPU = true
	cpu := findItem(t, GetAllDiscoveryItems(cfg, false, allCaps()), "cpu")
	if !cpu.ShouldRun {
		t.Fatal("summary removed individual sensor")
	}
	cfg.Expose.DeviceSummary = false
	item = findItem(t, GetAllDiscoveryItems(cfg, false, allCaps()), "device_summary")
	if item.ShouldRun || len(item.Payload) != 0 {
		t.Fatal("disabled summary discovery not cleared")
	}
}

func TestSummaryThumbnailVisibilityChangesPublishImmediately(t *testing.T) {
	original := summarySessionLocked
	locked := false
	summarySessionLocked = func() bool { return locked }
	t.Cleanup(func() { summarySessionLocked = original; privacy.Default.Resume() })
	privacy.Default.Resume()
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose.DeviceSummary = true
	cfg.Expose.ScreenThumbnail = true
	c := startClient(t, cfg)
	cpu := 12.0
	snap := telemetry.Snapshot{CPUPercent: &cpu}
	for i, condition := range []string{"visible", "locked", "resumed", "paused"} {
		locked = condition == "locked"
		if condition == "paused" {
			privacy.Default.Pause(time.Minute)
		}
		if err := c.PublishTelemetry(snap); err != nil {
			t.Fatal(err)
		}
		waitFor(t, condition, func() bool { return len(b.messages("sat/node1/state")) == i+1 })
		var state map[string]any
		messages := b.messages("sat/node1/state")
		if err := json.Unmarshal([]byte(messages[len(messages)-1].payload), &state); err != nil {
			t.Fatal(err)
		}
		want := condition == "visible" || condition == "resumed"
		if state["summary_thumbnail_available"] != want || state["cpu_percent"] != cpu {
			t.Fatalf("%s: %+v", condition, state)
		}
		if _, ok := state["media"]; ok {
			t.Fatal("summary introduced media details")
		}
		if _, ok := state["session_locked"]; ok {
			t.Fatal("summary introduced disabled lock sensor")
		}
	}
}
