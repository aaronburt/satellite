package mqtt

import (
	"context"
	"testing"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/telemetry"
	"satellite/internal/toast"
)

func TestClientBasics(t *testing.T) {
	c := NewClient()
	if c.Status() != StatusDisconnected {
		t.Fatalf("expected disconnected status, got %s", c.Status())
	}

	caps := capabilities.Detect()
	c.SetCapabilities(caps)
	c.SetHasBattery(true)

	if c.Registry() == nil {
		t.Fatal("expected non-nil registry")
	}

	_ = c.Registry().Execute("media_play_pause")

	c.mu.Lock()
	c.lastSnapshot = telemetry.Snapshot{
		Media: &telemetry.MediaInfo{Status: "Playing"},
	}
	c.mu.Unlock()

	_ = c.Registry().Execute("media_play_pause")

	cfg := config.DefaultConfig()
	cfg.MQTT.Broker = ""
	c.Start(cfg)

	if c.Status() != StatusDisconnected {
		t.Fatalf("expected status disconnected when broker is empty")
	}

	c.Stop()
	if c.Status() != StatusDisconnected {
		t.Fatalf("expected status disconnected after stop")
	}

	c.UpdateConfigAndRestart(cfg)
	if c.Status() != StatusDisconnected {
		t.Fatalf("expected status disconnected after restart with empty broker")
	}
}

func TestPublishTelemetryDisconnected(t *testing.T) {
	c := NewClient()
	snap := telemetry.Snapshot{
		LocalIP: "127.0.0.1",
	}

	err := c.PublishTelemetry(snap)
	if err != nil {
		t.Fatalf("expected nil error when publishing while disconnected, got %v", err)
	}
}

func TestHandleNotify(t *testing.T) {
	c := NewClient()

	c.handleNotify([]byte(`{"message": "should be ignored when notifications disabled"}`))

	c.cfg.Expose.Notifications = true

	c.handleNotify([]byte(`invalid json`))

	c.handleNotify([]byte(`{"message": ""}`))

	c.handleNotify([]byte(`{"message": "Hello without title"}`))
	select {
	case n := <-c.notifyChan:
		if n.Message != "Hello without title" {
			t.Fatalf("unexpected message: %s", n.Message)
		}
	default:
		t.Fatal("expected notification to be queued")
	}

	for i := 0; i < 5; i++ {
		c.notifyChan <- toast.Notification{Message: "filler"}
	}
	c.handleNotify([]byte(`{"title": "Droppable", "message": "Queue is full"}`))
}

func TestNotifyWorker(t *testing.T) {
	c := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		c.notifyWorker(ctx)
		close(done)
	}()

	c.notifyChan <- toast.Notification{
		Title:   "Worker Test",
		Message: "Toast message",
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("expected notifyWorker to exit upon context cancel")
	}
}

func TestTestConnectionErrors(t *testing.T) {
	err := TestConnection(config.MQTTConfig{Broker: ""})
	if err == nil {
		t.Fatal("expected error with empty broker")
	}

	err = TestConnection(config.MQTTConfig{Broker: "127.0.0.1:0"})
	if err == nil {
		t.Fatal("expected error with port 0")
	}

	err = TestConnection(config.MQTTConfig{Broker: "ssl://127.0.0.1:0", InsecureTLS: true})
	if err == nil {
		t.Fatal("expected error with unreachable ssl broker")
	}

	err = TestConnection(config.MQTTConfig{Broker: "://invalid url"})
	if err == nil {
		t.Fatal("expected error with invalid url")
	}
}

func TestMockBrokerDialErrors(t *testing.T) {
	c := NewClient()
	cfg := config.DefaultConfig()
	cfg.MQTT.Broker = "://invalid url"
	c.cfg = cfg
	err := c.connectAndServe(context.Background())
	if err == nil {
		t.Fatal("expected url parse error")
	}

	cfg.MQTT.Broker = "127.0.0.1:0"
	c.cfg = cfg
	err = c.connectAndServe(context.Background())
	if err == nil {
		t.Fatal("expected dial error")
	}

	cfg.MQTT.Broker = "ssl://127.0.0.1:0"
	cfg.MQTT.InsecureTLS = true
	c.cfg = cfg
	err = c.connectAndServe(context.Background())
	if err == nil {
		t.Fatal("expected tls dial error")
	}
}

func TestRunLoopCancel(t *testing.T) {
	c := NewClient()
	cfg := config.DefaultConfig()
	cfg.MQTT.Broker = "127.0.0.1:0"
	c.cfg = cfg

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		c.runLoop(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("expected runLoop to exit")
	}
}

func TestBroadcastNotificationHandling(t *testing.T) {
	c := NewClient()
	c.cfg.Expose.Notifications = true
	c.cfg.NodeID = "my-pc"

	c.handleNotify([]byte(`{"title": "Broadcast Alert", "message": "Global announcement"}`))

	select {
	case n := <-c.notifyChan:
		if n.Title != "Broadcast Alert" || n.Message != "Global announcement" {
			t.Fatalf("unexpected notification: %+v", n)
		}
	default:
		t.Fatal("expected broadcast notification in queue")
	}
}
