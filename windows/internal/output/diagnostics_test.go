package output

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"satellite/internal/config"
)

func TestWebhookDiagnosticsRetryAndRecovery(t *testing.T) {
	s := &scripted{statuses: []int{503, 429}}
	srv := httptest.NewServer(s)
	defer srv.Close()
	h := newRetryHarness(t, srv.URL)
	h.onWait = func() {
		d := h.w.Diagnostics()
		if d.Status != "retrying" || d.NextRetry == nil || d.CurrentError == nil || d.LastPublished != nil {
			t.Errorf("retry diagnostics: %+v", d)
		}
	}
	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)
	d := h.w.Diagnostics()
	if d.Status != "healthy" || d.Attempts != 3 || d.Failures != 2 || d.Publishes != 1 || d.CurrentError != nil || d.NextRetry != nil || d.LastPublished == nil {
		t.Fatalf("recovery diagnostics: %+v", d)
	}
	if len(d.RecentErrors) != 2 || d.RecentErrors[0].Code != "server" || d.RecentErrors[1].Code != "rate_limit" {
		t.Fatalf("failure history: %+v", d.RecentErrors)
	}
	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)
	if h.w.Diagnostics().Attempts != 3 {
		t.Fatal("suppressed telemetry must not count as an attempt")
	}
}

func TestWebhookDiagnosticsPermanentFailureAndConfigReset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer srv.Close()
	h := newRetryHarness(t, srv.URL)
	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)
	d := h.w.Diagnostics()
	if d.Status != "failed" || d.Failures != 1 || d.Attempts != 1 || d.Publishes != 0 || d.NextRetry != nil || d.CurrentError.Code != "authorization" {
		t.Fatalf("permanent failure diagnostics: %+v", d)
	}
	h.mu.Lock()
	h.cfg.TelemetryWebhook.BearerToken = "replacement-token"
	h.mu.Unlock()
	d = h.w.Diagnostics()
	if d.Status != "waiting" || d.Failures != 0 || d.CurrentError != nil {
		t.Fatalf("new credentials kept old diagnostics: %+v", d)
	}
	h.mu.Lock()
	h.cfg.TelemetryWebhook = config.TelemetryWebhookConfig{}
	h.mu.Unlock()
	if h.w.Diagnostics().Status != "disabled" {
		t.Fatal("disabled webhook should be shown as off")
	}
}
