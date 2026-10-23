package mqtt

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"satellite/internal/config"
	"satellite/internal/telemetry"
	"satellite/internal/toast"
)

// testBroker is a real in-process MQTT broker that records every message.
type testBroker struct {
	t      *testing.T
	server *mochi.Server
	addr   string

	mu   sync.Mutex
	msgs []recorded
}

type recorded struct {
	topic   string
	payload string
	retain  bool
}

func startBroker(t *testing.T, user, pass string) *testBroker {
	t.Helper()
	server := mochi.New(&mochi.Options{
		InlineClient: true,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if user == "" {
		if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
			t.Fatal(err)
		}
	} else {
		err := server.AddHook(new(auth.Hook), &auth.Options{Ledger: &auth.Ledger{
			Auth: auth.AuthRules{{Username: auth.RString(user), Password: auth.RString(pass), Allow: true}},
			ACL:  auth.ACLRules{{Filters: auth.Filters{"#": auth.ReadWrite}}},
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	tcp := listeners.NewTCP(listeners.Config{ID: "tcp", Address: "127.0.0.1:0"})
	if err := server.AddListener(tcp); err != nil {
		t.Fatal(err)
	}
	b := &testBroker{t: t, server: server, addr: tcp.Address()}
	if err := server.Subscribe("#", 1, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) {
		b.mu.Lock()
		b.msgs = append(b.msgs, recorded{pk.TopicName, string(pk.Payload), pk.FixedHeader.Retain})
		b.mu.Unlock()
	}); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	return b
}

func (b *testBroker) messages(topic string) []recorded {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []recorded
	for _, m := range b.msgs {
		if m.topic == topic {
			out = append(out, m)
		}
	}
	return out
}

func (b *testBroker) all() []recorded {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]recorded(nil), b.msgs...)
}

// waitFor polls cond until it is true or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (b *testBroker) waitForMessage(t *testing.T, topic, payload string) recorded {
	t.Helper()
	var found recorded
	waitFor(t, "message "+payload+" on "+topic, func() bool {
		for _, m := range b.messages(topic) {
			if m.payload == payload {
				found = m
				return true
			}
		}
		return false
	})
	return found
}

func fastReconnect(t *testing.T) {
	t.Helper()
	origInit, origMax, origStable := initialBackoff, maxBackoff, stableAfter
	t.Cleanup(func() { initialBackoff, maxBackoff, stableAfter = origInit, origMax, origStable })
	initialBackoff, maxBackoff, stableAfter = 20*time.Millisecond, 50*time.Millisecond, time.Hour
}

func brokerConfig(b *testBroker) config.Config {
	cfg := config.DefaultConfig()
	cfg.NodeID = "node1"
	cfg.MQTT.Broker = b.addr
	cfg.MQTT.TopicPrefix = "sat"
	cfg.MQTT.ClientID = "node1-client"
	return cfg
}

func startClient(t *testing.T, cfg config.Config) *Client {
	t.Helper()
	stubHardware(t, nil, nil)
	c := NewClient()
	c.SetCapabilities(allCaps())
	c.Start(cfg)
	t.Cleanup(c.Stop)
	waitFor(t, "client to connect", func() bool { return c.Status() == StatusConnected })
	return c
}

func TestClientAnnouncesOnlineAndPublishesDiscovery(t *testing.T) {
	b := startBroker(t, "user", "pass")
	cfg := brokerConfig(b)
	cfg.MQTT.Username, cfg.MQTT.Password = "user", "pass"
	cfg.Expose = config.ExposeConfig{CPU: true, Memory: true}
	startClient(t, cfg)

	online := b.waitForMessage(t, "sat/node1/status", "online")
	if !online.retain {
		t.Error("online status must be retained so Home Assistant sees it after restarting")
	}

	expected := GetAllDiscoveryItems(cfg, false, allCaps())
	waitFor(t, "all discovery messages", func() bool {
		for _, item := range expected {
			if len(b.messages(item.Topic)) == 0 {
				return false
			}
		}
		return true
	})
	for _, item := range expected {
		m := b.messages(item.Topic)[0]
		if !m.retain {
			t.Errorf("%s: discovery message must be retained", item.Key)
		}
		if item.ShouldRun && m.payload != string(item.Payload) {
			t.Errorf("%s: published %q, want %q", item.Key, m.payload, item.Payload)
		}
		if !item.ShouldRun && m.payload != "" {
			t.Errorf("%s: disabled entity should be cleared with an empty payload, got %q", item.Key, m.payload)
		}
	}
}

func TestClientWithWrongPasswordNeverConnects(t *testing.T) {
	fastReconnect(t)
	b := startBroker(t, "user", "pass")
	cfg := brokerConfig(b)
	cfg.MQTT.Username, cfg.MQTT.Password = "user", "wrong"

	c := NewClient()
	c.Start(cfg)
	defer c.Stop()
	time.Sleep(300 * time.Millisecond) // several reconnect attempts
	if c.Status() == StatusConnected {
		t.Fatal("client connected with a wrong password")
	}
	if len(b.messages("sat/node1/status")) != 0 {
		t.Fatal("rejected client must not publish anything")
	}
}

func TestStopPublishesOfflineAndDisconnects(t *testing.T) {
	b := startBroker(t, "", "")
	c := startClient(t, brokerConfig(b))
	b.waitForMessage(t, "sat/node1/status", "online")

	c.Stop()
	offline := b.waitForMessage(t, "sat/node1/status", "offline")
	if !offline.retain {
		t.Error("offline status must be retained")
	}
	if c.Status() != StatusDisconnected {
		t.Fatalf("status = %s after Stop", c.Status())
	}
	if err := c.PublishTelemetry(telemetry.Snapshot{}); err != nil {
		t.Fatalf("publishing while stopped should be a no-op, got %v", err)
	}
}

// If the agent dies without saying goodbye, the broker must publish the
// last-will "offline", and the agent must reconnect and come back online.
func TestLastWillAndReconnect(t *testing.T) {
	fastReconnect(t)
	b := startBroker(t, "", "")
	c := startClient(t, brokerConfig(b))
	b.waitForMessage(t, "sat/node1/status", "online")

	c.mu.Lock()
	conn := c.netConn
	c.mu.Unlock()
	_ = conn.Close() // simulate a network drop

	b.waitForMessage(t, "sat/node1/status", "offline")
	waitFor(t, "second online message", func() bool {
		n := 0
		for _, m := range b.messages("sat/node1/status") {
			if m.payload == "online" {
				n++
			}
		}
		return n >= 2
	})
	waitFor(t, "reconnected status", func() bool { return c.Status() == StatusConnected })
}

func TestTelemetryPublishingThrottlesUnchangedState(t *testing.T) {
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.HeartbeatSec = 60
	c := startClient(t, cfg)
	stateTopic := "sat/node1/state"

	cpu := 10.0
	snap := telemetry.Snapshot{CPUPercent: &cpu, Timestamp: 1}
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "first state", func() bool { return len(b.messages(stateTopic)) == 1 })
	var got telemetry.Snapshot
	if err := json.Unmarshal([]byte(b.messages(stateTopic)[0].payload), &got); err != nil {
		t.Fatal(err)
	}
	if got.CPUPercent == nil || *got.CPUPercent != 10 || got.Timestamp != 1 {
		t.Fatalf("published state = %+v", got)
	}
	if b.messages(stateTopic)[0].retain {
		t.Error("state messages should not be retained")
	}

	// Same values within the heartbeat window: nothing is sent.
	cpuSmall := 10.4
	_ = c.PublishTelemetry(telemetry.Snapshot{CPUPercent: &cpuSmall, Timestamp: 2})
	// A significant change is sent straight away.
	cpuBig := 50.0
	_ = c.PublishTelemetry(telemetry.Snapshot{CPUPercent: &cpuBig, Timestamp: 3})
	waitFor(t, "changed state", func() bool { return len(b.messages(stateTopic)) == 2 })
	time.Sleep(50 * time.Millisecond)
	if n := len(b.messages(stateTopic)); n != 2 {
		t.Fatalf("got %d state messages, want 2 (unchanged state must be skipped)", n)
	}

	// Once the heartbeat interval has passed, unchanged state is re-sent.
	c.mu.Lock()
	c.lastSentTime = time.Now().Add(-61 * time.Second)
	c.mu.Unlock()
	_ = c.PublishTelemetry(telemetry.Snapshot{CPUPercent: &cpuBig, Timestamp: 4})
	waitFor(t, "heartbeat state", func() bool { return len(b.messages(stateTopic)) == 3 })
}

func TestReconnectForcesFullStatePublish(t *testing.T) {
	fastReconnect(t)
	b := startBroker(t, "", "")
	c := startClient(t, brokerConfig(b))
	stateTopic := "sat/node1/state"

	cpu := 10.0
	snap := telemetry.Snapshot{CPUPercent: &cpu}
	_ = c.PublishTelemetry(snap)
	waitFor(t, "first state", func() bool { return len(b.messages(stateTopic)) == 1 })

	c.mu.Lock()
	conn := c.netConn
	c.mu.Unlock()
	_ = conn.Close()
	// The second "online" is published once the new session is fully set up.
	waitFor(t, "reconnect", func() bool {
		n := 0
		for _, m := range b.messages("sat/node1/status") {
			if m.payload == "online" {
				n++
			}
		}
		return n >= 2
	})

	// Identical snapshot, but Home Assistant may have missed state while we
	// were away, so the first publish after reconnecting is always sent.
	_ = c.PublishTelemetry(snap)
	waitFor(t, "state after reconnect", func() bool { return len(b.messages(stateTopic)) == 2 })
}

type recordingRegistry struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingRegistry) handler(name string) func() error {
	return func() error {
		r.mu.Lock()
		r.calls = append(r.calls, name)
		r.mu.Unlock()
		return nil
	}
}

func (r *recordingRegistry) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func registerFakes(c *Client) *recordingRegistry {
	rec := &recordingRegistry{}
	for _, a := range []string{"lock", "media_play_pause", "volume_up", "display_sleep"} {
		c.Registry().Register(a, rec.handler(a))
	}
	return rec
}

func TestRemoteCommandsAreGatedBySettings(t *testing.T) {
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose = config.ExposeConfig{MediaControl: true}
	c := startClient(t, cfg)
	rec := registerFakes(c)
	b.waitForMessage(t, "sat/node1/status", "online")
	time.Sleep(50 * time.Millisecond) // let the subscription settle

	for _, payload := range []string{"lock", `{"action":"display_sleep"}`, "volume_up", `{"command":"play_pause"}`} {
		if err := b.server.Publish("sat/node1/command", []byte(payload), false, 1); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "media commands", func() bool { return len(rec.got()) >= 2 })
	time.Sleep(50 * time.Millisecond)
	got := rec.got()
	if len(got) != 2 || got[0] != "volume_up" || got[1] != "media_play_pause" {
		t.Fatalf("executed %v; only media commands should run when only media control is enabled", got)
	}
}

func TestNoCommandsWhenNothingIsEnabled(t *testing.T) {
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose = config.ExposeConfig{CPU: true}
	c := startClient(t, cfg)
	rec := registerFakes(c)
	b.waitForMessage(t, "sat/node1/status", "online")
	time.Sleep(50 * time.Millisecond)

	_ = b.server.Publish("sat/node1/command", []byte("lock"), false, 1)
	time.Sleep(100 * time.Millisecond)
	if got := rec.got(); len(got) != 0 {
		t.Fatalf("commands executed without being enabled: %v", got)
	}
}

func TestDisplaySleepButtonWorksWithOnlyDisplayStateEnabled(t *testing.T) {
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose = config.ExposeConfig{DisplayState: true}
	c := startClient(t, cfg)
	rec := registerFakes(c)
	b.waitForMessage(t, "sat/node1/status", "online")
	time.Sleep(50 * time.Millisecond)

	_ = b.server.Publish("sat/node1/command", []byte("display_sleep"), false, 1)
	waitFor(t, "display sleep", func() bool { return len(rec.got()) == 1 })
}

func TestNotificationsAreDeliveredOnlyWhenEnabled(t *testing.T) {
	var mu sync.Mutex
	var shown []toast.Notification
	orig := showToast
	t.Cleanup(func() { showToast = orig })
	showToast = func(n toast.Notification) error {
		mu.Lock()
		shown = append(shown, n)
		mu.Unlock()
		return nil
	}
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(shown) }

	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose = config.ExposeConfig{Notifications: true}
	startClient(t, cfg)
	b.waitForMessage(t, "sat/node1/status", "online")
	time.Sleep(50 * time.Millisecond)

	_ = b.server.Publish("sat/node1/notify", []byte(`{"title":"Door","message":"Someone is at the door"}`), false, 1)
	waitFor(t, "direct notification", func() bool { return count() == 1 })
	_ = b.server.Publish("sat/all/notify", []byte(`{"message":"Dinner"}`), false, 1)
	waitFor(t, "broadcast notification", func() bool { return count() == 2 })
	_ = b.server.Publish("sat/othernode/notify", []byte(`{"message":"not for us"}`), false, 1)
	_ = b.server.Publish("sat/node1/notify", []byte(`not json`), false, 1)
	_ = b.server.Publish("sat/node1/notify", []byte(`{"title":"empty","message":"  "}`), false, 1)
	time.Sleep(1500 * time.Millisecond) // the worker spaces toasts 1s apart

	mu.Lock()
	defer mu.Unlock()
	if len(shown) != 2 {
		t.Fatalf("shown %d notifications, want 2: %+v", len(shown), shown)
	}
	if shown[0].Title != "Door" || shown[0].Message != "Someone is at the door" {
		t.Errorf("first notification = %+v", shown[0])
	}
	if shown[1].Title == "" {
		t.Error("notification without a title should get the host name as title")
	}
}

func TestNotificationsIgnoredWhenDisabled(t *testing.T) {
	called := make(chan struct{}, 1)
	orig := showToast
	t.Cleanup(func() { showToast = orig })
	showToast = func(toast.Notification) error { called <- struct{}{}; return nil }

	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose = config.ExposeConfig{CPU: true}
	startClient(t, cfg)
	b.waitForMessage(t, "sat/node1/status", "online")

	_ = b.server.Publish("sat/node1/notify", []byte(`{"message":"hi"}`), false, 1)
	select {
	case <-called:
		t.Fatal("notification shown although notifications are disabled")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestTestConnection(t *testing.T) {
	b := startBroker(t, "user", "pass")
	ok := config.MQTTConfig{Broker: b.addr, Username: "user", Password: "pass"}
	if err := TestConnection(ok); err != nil {
		t.Fatalf("valid credentials: %v", err)
	}
	ok.Broker = "tcp://" + b.addr
	if err := TestConnection(ok); err != nil {
		t.Fatalf("tcp:// scheme: %v", err)
	}
	bad := ok
	bad.Password = "nope"
	if err := TestConnection(bad); err == nil {
		t.Fatal("wrong password should fail")
	}
	if err := TestConnection(config.MQTTConfig{}); err == nil {
		t.Fatal("empty broker should fail")
	}

	// A port with nothing listening.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := ln.Addr().String()
	ln.Close()
	if err := TestConnection(config.MQTTConfig{Broker: closed}); err == nil {
		t.Fatal("closed port should fail")
	}
}

func TestUpdateConfigAndRestartSwitchesTopics(t *testing.T) {
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	c := startClient(t, cfg)
	b.waitForMessage(t, "sat/node1/status", "online")

	cfg.NodeID = "renamed"
	c.UpdateConfigAndRestart(cfg)
	b.waitForMessage(t, "sat/node1/status", "offline")
	b.waitForMessage(t, "sat/renamed/status", "online")
	waitFor(t, "connected after restart", func() bool { return c.Status() == StatusConnected })
}

// A node literally named "all" must not subscribe to the broadcast topic
// twice (it is already its own direct topic).
func TestNodeNamedAllReceivesBroadcastOnce(t *testing.T) {
	var mu sync.Mutex
	n := 0
	orig := showToast
	t.Cleanup(func() { showToast = orig })
	showToast = func(toast.Notification) error { mu.Lock(); n++; mu.Unlock(); return nil }

	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.NodeID = "all"
	cfg.Expose = config.ExposeConfig{Notifications: true}
	startClient(t, cfg)
	b.waitForMessage(t, "sat/all/status", "online")
	time.Sleep(50 * time.Millisecond)

	_ = b.server.Publish("sat/all/notify", []byte(`{"message":"hello"}`), false, 1)
	time.Sleep(1300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if n != 1 {
		t.Fatalf("notification shown %d times, want 1", n)
	}
}
