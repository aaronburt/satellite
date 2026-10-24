package output

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/telemetry"
)

type recorder struct {
	mu       sync.Mutex
	payloads []WebhookPayload
	auth     []string
}

func (r *recorder) handler(w http.ResponseWriter, req *http.Request) {
	var p WebhookPayload
	_ = json.NewDecoder(req.Body).Decode(&p)
	r.mu.Lock()
	r.payloads = append(r.payloads, p)
	r.auth = append(r.auth, req.Header.Get("Authorization"))
	r.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.payloads)
}

func waitIdle(t *testing.T, w *Webhook) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for w.inFlight.Load() {
		if time.Now().After(deadline) {
			t.Fatal("webhook post did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func cpu(v float64) telemetry.Snapshot { return telemetry.Snapshot{CPUPercent: &v} }

func TestWebhookSendsEnvelopeAndToken(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	cfg := config.Config{NodeID: "pc-1", TelemetryWebhook: config.TelemetryWebhookConfig{
		Enabled: true, URL: srv.URL, BearerToken: "abc",
	}}
	w := NewWebhook(func() config.Config { return cfg })
	_ = w.Publish(cpu(42))
	waitIdle(t, w)

	if rec.count() != 1 {
		t.Fatalf("expected 1 post, got %d", rec.count())
	}
	p := rec.payloads[0]
	if p.DeviceID != "pc-1" || p.AppVersion != config.Version || p.Platform == "" || p.Timestamp == 0 {
		t.Errorf("unexpected envelope: %+v", p)
	}
	if p.State.CPUPercent == nil || *p.State.CPUPercent != 42 {
		t.Errorf("state not carried through: %+v", p.State)
	}
	if rec.auth[0] != "Bearer abc" {
		t.Errorf("expected bearer token, got %q", rec.auth[0])
	}
}

func TestWebhookDisabledOrNoURLSendsNothing(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	for _, wh := range []config.TelemetryWebhookConfig{
		{Enabled: false, URL: srv.URL},
		{Enabled: true, URL: "  "},
	} {
		cfg := config.Config{TelemetryWebhook: wh}
		w := NewWebhook(func() config.Config { return cfg })
		_ = w.Publish(cpu(1))
		waitIdle(t, w)
	}
	if rec.count() != 0 {
		t.Fatalf("expected no posts, got %d", rec.count())
	}
}

func TestWebhookChangeDetectionAndMinInterval(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	now := time.Unix(1000, 0)
	cfg := config.Config{HeartbeatSec: 60, TelemetryWebhook: config.TelemetryWebhookConfig{
		Enabled: true, URL: srv.URL, MinIntervalSec: 10,
	}}
	w := NewWebhook(func() config.Config { return cfg })
	w.now = func() time.Time { return now }

	step := func(d time.Duration, snap telemetry.Snapshot) {
		now = now.Add(d)
		_ = w.Publish(snap)
		waitIdle(t, w)
	}

	step(0, cpu(10))              // first post
	step(5*time.Second, cpu(90))  // big change but inside min interval
	step(5*time.Second, cpu(10))  // no change since last post
	step(1*time.Second, cpu(50))  // change and past min interval
	step(30*time.Second, cpu(50)) // unchanged, heartbeat not due
	step(31*time.Second, cpu(50)) // heartbeat due

	if got := rec.count(); got != 3 {
		t.Fatalf("expected 3 posts, got %d", got)
	}
}

func TestSendReportsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := SendTest(config.TelemetryWebhookConfig{URL: srv.URL}, "n", telemetry.Snapshot{})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if err := SendTest(config.TelemetryWebhookConfig{}, "n", telemetry.Snapshot{}); err == nil {
		t.Fatal("expected error for empty URL")
	}
}
