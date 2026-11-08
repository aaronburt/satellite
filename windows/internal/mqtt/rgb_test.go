package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/icue"
	"satellite/internal/privacy"
	"satellite/internal/telemetry"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

func TestRGBDiscoveryRequiresSupportAndOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, supported := range []bool{false, true} {
			cfg := config.DefaultConfig()
			cfg.Expose.ICUERGB = enabled
			found := false
			for _, item := range GetAllDiscoveryItems(cfg, false, capabilities.PlatformCapabilities{ICUERGB: supported}) {
				if item.Key != "icue_rgb" {
					continue
				}
				found = true
				if item.ShouldRun != (enabled && supported) {
					t.Fatal("RGB discovery ignored support or opt-in")
				}
				if !item.ShouldRun {
					continue
				}
				var payload EntityDiscoveryPayload
				if err := json.Unmarshal(item.Payload, &payload); err != nil {
					t.Fatal(err)
				}
				if payload.CommandTopic != "" || payload.JSONAttributesTemplate == "" {
					t.Fatal("RGB must expose read-only attributes")
				}
			}
			if !found {
				t.Fatal("missing RGB discovery item")
			}
		}
	}
}

func rgbSnapshot() telemetry.Snapshot {
	return telemetry.Snapshot{RGB: &icue.State{Status: "connected", Devices: []icue.Device{
		{ID: "keyboard", Model: "K70", Type: 1, Average: [3]uint8{255, 64, 0}, LEDs: []icue.LED{{ID: 1, R: 255, G: 64, A: 255}}},
		{ID: "mouse", Model: "M65", Type: 2, Average: [3]uint8{0, 0, 255}, LEDs: []icue.LED{{ID: 2, B: 255, A: 255}}},
	}}}
}

func TestRGBSubdevicesHaveIndependentDiscovery(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NodeID = "desk.local"
	cfg.MQTT.TopicPrefix = "custom"
	seen := make(map[string]bool)
	for _, d := range rgbSnapshot().RGB.Devices {
		item := rgbDiscoveryItem(cfg, d)
		if !haDiscoveryTopic.MatchString(item.Topic) || seen[item.Topic] {
			t.Fatalf("invalid or duplicate discovery topic: %s", item.Topic)
		}
		seen[item.Topic] = true
		var p EntityDiscoveryPayload
		if err := json.Unmarshal(item.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if p.UniqueID != "desk.local_icue_rgb_"+d.ID || p.Device.Identifiers[0] != "satellite_desk.local_icue_rgb_"+d.ID {
			t.Fatalf("unstable entity or device identity: %+v", p)
		}
		if p.Device.Name != d.Model || p.Device.Model != d.Model || p.Device.Manufacturer != "Corsair" || p.Device.ViaDevice != "satellite_desk.local" {
			t.Fatalf("incorrect subdevice metadata: %+v", p.Device)
		}
		if p.Name != "Average colour" || p.CommandTopic != "" || p.StateTopic != "custom/desk.local/state" {
			t.Fatalf("expected a read-only colour sensor: %+v", p)
		}
		if p.AvailabilityTopic != "" || p.AvailabilityMode != "all" || len(p.Availability) != 2 || p.Availability[0].Topic != "custom/desk.local/status" || p.Availability[1].Topic != p.StateTopic {
			t.Fatalf("missing agent and per-device availability: %+v", p)
		}
		for _, tmpl := range []string{p.ValueTemplate, p.JSONAttributesTemplate, p.Availability[1].ValueTemplate} {
			if !strings.Contains(tmpl, `'equalto', "`+d.ID+`"`) {
				t.Fatalf("template does not select the stable device ID: %s", tmpl)
			}
		}
		if !strings.Contains(p.JSONAttributesTemplate, "led_count") || !strings.Contains(p.JSONAttributesTemplate, "'leds': []") {
			t.Fatal("missing per-LED attributes or stale-colour clearing")
		}
	}
	d := rgbSnapshot().RGB.Devices[0]
	first := rgbDiscoveryItem(cfg, d)
	d.Model = "Renamed keyboard"
	d.LEDs[0].R = 1
	second := rgbDiscoveryItem(cfg, d)
	if first.Topic != second.Topic || first.Key != second.Key {
		t.Fatal("model or colour change changed device identity")
	}
}

func TestRGBDiscoveryRetriesFailuresAndSkipsColourOnlyChanges(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Expose.ICUERGB = true
	r := newRGBDiscovery(cfg, allCaps(), nil)
	attempts := 0
	r.publish = func(context.Context, string, []byte) error {
		attempts++
		if attempts == 1 {
			return errors.New("broker unavailable")
		}
		return nil
	}
	snap := rgbSnapshot()
	if err := r.sync(context.Background(), snap.RGB); err == nil {
		t.Fatal("publish failure was ignored")
	}
	if err := r.sync(context.Background(), snap.RGB); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("got %d attempts, want one failure and two successful devices", attempts)
	}
	snap.RGB.Devices[0].Average[0] = 5
	snap.RGB.Devices[0].LEDs[0].R = 5
	if err := r.sync(context.Background(), snap.RGB); err != nil || attempts != 3 {
		t.Fatal("colour changes republished unchanged discovery")
	}
	snap.RGB.Devices[0].Model = "New model"
	if err := r.sync(context.Background(), snap.RGB); err != nil || attempts != 4 {
		t.Fatal("model changes did not refresh discovery")
	}
}

func TestRGBDiscoveryFailureDoesNotBlockTelemetryAndRetries(t *testing.T) {
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose.ICUERGB = true
	c := startClient(t, cfg)
	c.mu.RLock()
	r := c.rgbDiscovery
	c.mu.RUnlock()
	publish := r.publish
	attempts := 0
	r.publish = func(ctx context.Context, topic string, payload []byte) error {
		attempts++
		if attempts <= 2 {
			return errors.New("discovery temporarily rejected")
		}
		return publish(ctx, topic, payload)
	}
	snap := rgbSnapshot()
	for i := 1; i <= 3; i++ {
		if err := c.PublishTelemetry(snap); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "telemetry despite discovery failure", func() bool { return len(b.messages("sat/node1/state")) == i })
	}
	if attempts != 4 {
		t.Fatalf("got %d attempts, want two failures and two successful discoveries", attempts)
	}
	if err := c.PublishTelemetry(snap); err != nil || attempts != 4 {
		t.Fatal("unchanged discovery was not throttled after recovery")
	}
	c.mu.RLock()
	force := c.forcePublishNext
	c.mu.RUnlock()
	if force {
		t.Fatal("successful discovery should restore normal telemetry throttling")
	}
}

func TestRGBSubdeviceDiscoveryLifecycle(t *testing.T) {
	fastReconnect(t)
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose.ICUERGB = true
	c := startClient(t, cfg)
	snap := rgbSnapshot()
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	items := []DiscoveryItem{rgbDiscoveryItem(cfg, snap.RGB.Devices[0]), rgbDiscoveryItem(cfg, snap.RGB.Devices[1])}
	for _, item := range items {
		m := b.waitForMessage(t, item.Topic, string(item.Payload))
		if !m.retain {
			t.Fatal("subdevice discovery must be retained")
		}
	}
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	snap = rgbSnapshot()
	snap.RGB.Devices = snap.RGB.Devices[:1]
	snap.RGB.Status = "partial"
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "partial state", func() bool { return len(b.messages("sat/node1/state")) == 2 })
	for _, item := range items {
		if len(b.messages(item.Topic)) != 1 {
			t.Fatal("temporary read failure must keep discovery and use state availability")
		}
	}
	replayed := make(chan string, len(items))
	if err := b.server.Subscribe(rgbDiscoveryPrefix(cfg.NodeID)+"+/config", 2, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) {
		replayed <- pk.TopicName
	}); err != nil {
		t.Fatal(err)
	}
	for range items {
		select {
		case <-replayed:
		case <-time.After(2 * time.Second):
			t.Fatal("a restarted Home Assistant would not receive retained discovery")
		}
	}
	if err := b.server.Unsubscribe(rgbDiscoveryPrefix(cfg.NodeID)+"+/config", 2); err != nil {
		t.Fatal(err)
	}
	c.mu.RLock()
	conn := c.netConn
	c.mu.RUnlock()
	_ = conn.Close()
	waitFor(t, "reconnect", func() bool { return len(b.messages("sat/node1/status")) >= 3 && c.Status() == StatusConnected })
	snap = rgbSnapshot()
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		waitFor(t, "discovery after reconnect", func() bool { return len(b.messages(item.Topic)) == 2 })
	}
	c.Stop()
	otherTopic := rgbDiscoveryPrefix("other-node") + "rgb/config"
	if err := b.server.Publish(otherTopic, []byte(`{"unique_id":"other-device"}`), true, 1); err != nil {
		t.Fatal(err)
	}
	cfg.Expose.ICUERGB = false
	startClient(t, cfg)
	for _, item := range items {
		b.waitForMessage(t, item.Topic, "")
	}
	retained := b.server.Topics.Retained.GetAll()
	otherFound := false
	for _, message := range retained {
		if message.TopicName == otherTopic {
			otherFound = true
		}
		for _, item := range items {
			if message.TopicName == item.Topic {
				t.Fatal("disabled device discovery remains retained")
			}
		}
	}
	if !otherFound {
		t.Fatal("another node's retained discovery was removed")
	}
}

func TestRGBSubdevicesRequireSupportOptInAndReadableDevices(t *testing.T) {
	for _, test := range []struct {
		name      string
		enabled   bool
		supported bool
		state     *icue.State
	}{
		{"disabled", false, true, rgbSnapshot().RGB},
		{"unsupported", true, false, rgbSnapshot().RGB},
		{"privacy pause", true, true, nil},
		{"read failure", true, true, &icue.State{Status: "read_failed", Devices: rgbSnapshot().RGB.Devices}},
		{"unreadable devices", true, true, &icue.State{Status: "connected", Devices: []icue.Device{{ID: "no-leds"}, {LEDs: []icue.LED{{ID: 1}}}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Expose.ICUERGB = test.enabled
			r := newRGBDiscovery(cfg, capabilities.PlatformCapabilities{ICUERGB: test.supported}, nil)
			r.publish = func(context.Context, string, []byte) error {
				t.Fatal("discovered disabled or unreadable RGB devices")
				return nil
			}
			if err := r.sync(context.Background(), test.state); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRGBPublishedAndClearedOnPause(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose.ICUERGB = true
	c := startClient(t, cfg)
	snap := telemetry.Snapshot{RGB: &icue.State{Status: "connected", Devices: []icue.Device{{ID: "keyboard", Model: "Keyboard", LEDs: []icue.LED{{ID: 1, R: 255, A: 255}}}}}}
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "RGB publish", func() bool { return len(b.messages("sat/node1/state")) == 1 })
	var state telemetry.Snapshot
	if err := json.Unmarshal([]byte(b.messages("sat/node1/state")[0].payload), &state); err != nil {
		t.Fatal(err)
	}
	if state.RGB == nil || state.RGB.Devices[0].LEDs[0].R != 255 {
		t.Fatal("MQTT lost LED colours")
	}
	privacy.Default.Pause(time.Minute)
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "RGB pause", func() bool { return len(b.messages("sat/node1/state")) == 2 })
	state = telemetry.Snapshot{}
	if err := json.Unmarshal([]byte(b.messages("sat/node1/state")[1].payload), &state); err != nil {
		t.Fatal(err)
	}
	if state.RGB != nil {
		t.Fatal("MQTT exposed paused RGB")
	}
}
