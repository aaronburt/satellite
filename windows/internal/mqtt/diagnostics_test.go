package mqtt

import (
	"encoding/json"
	"strings"
	"testing"

	"satellite/internal/config"
	"satellite/internal/telemetry"
)

func TestDiagnosticsTracksLivePublishAndReconnect(t *testing.T) {
	fastReconnect(t)
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	c := startClient(t, cfg)
	b.waitForMessage(t, "sat/node1/status", "online")
	snap := telemetry.Snapshot{LocalIP: "127.0.0.1"}
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	if err := c.PublishTelemetry(snap); err != nil {
		t.Fatal(err)
	}
	d := c.Diagnostics()
	if d.Status != "connected" || d.Attempts != 1 || d.Publishes != 1 || d.LastPublished == nil || d.Reconnects != 0 {
		t.Fatalf("initial diagnostics: %+v", d)
	}
	if err := TestConnection(cfg.MQTT); err != nil {
		t.Fatal(err)
	}
	if c.Status() != StatusConnected || c.Diagnostics().Attempts != 1 || c.Diagnostics().Publishes != 1 {
		t.Fatal("test connection disturbed the live client or counters")
	}
	c.mu.RLock()
	conn := c.netConn
	c.mu.RUnlock()
	_ = conn.Close()
	waitFor(t, "diagnostic reconnect", func() bool { return c.Diagnostics().Reconnects == 1 })
	d = c.Diagnostics()
	if d.LastDisconnected == nil || d.Attempts < 2 || d.CurrentError != nil || len(d.RecentErrors) == 0 {
		t.Fatalf("reconnect diagnostics: %+v", d)
	}
	c.Stop()
	if c.Diagnostics().NextRetry != nil {
		t.Fatal("stopped client must not advertise a retry")
	}
}

func TestDiagnosticsClassifiesBrokerAuthenticationFailure(t *testing.T) {
	b := startBroker(t, "user", "correct-secret")
	cfg := brokerConfig(b)
	cfg.MQTT.Username = "user"
	cfg.MQTT.Password = "private-password"
	c := NewClient()
	c.Start(cfg)
	t.Cleanup(c.Stop)
	waitFor(t, "authentication diagnostic", func() bool { return c.Diagnostics().CurrentError != nil })
	d := c.Diagnostics()
	if d.CurrentError.Code != "authorization" || d.Publishes != 0 || d.LastPublished != nil {
		t.Fatalf("authentication diagnostic: %+v", d)
	}
	data, _ := json.Marshal(d)
	for _, secret := range []string{cfg.MQTT.Password, b.addr, cfg.MQTT.Username} {
		if strings.Contains(string(data), `"`+secret+`"`) {
			t.Fatalf("diagnostic contains credentials or address: %s", data)
		}
	}
	cfg.MQTT = config.MQTTConfig{}
	c.UpdateConfigAndRestart(cfg)
	if got := c.Diagnostics(); got.Status != "disabled" || got.Failures != 0 || got.CurrentError != nil {
		t.Fatalf("changed configuration kept stale diagnostics: %+v", got)
	}
}
