package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"satellite/internal/config"
	"satellite/internal/diagnostics"
	"satellite/internal/output"
)

func TestDiagnosticsEndpointAuthAndMethods(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MQTT.Broker = ""
	srv, base, token := startTestServer(t, cfg)
	webhook := output.NewWebhook(config.Get)
	t.Cleanup(webhook.Close)
	srv.SetTelemetryWebhook(webhook)
	resp, _ := apiRequest(t, "GET", base+"/api/diagnostics", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("diagnostics accepted an unauthenticated request")
	}
	resp, body := apiRequest(t, "GET", base+"/api/diagnostics", token, nil)
	var result map[string]diagnostics.Snapshot
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || result["mqtt"].Status != "disabled" || result["telemetry_webhook"].Status != "disabled" {
		t.Fatalf("unexpected diagnostics: %s", body)
	}
	for _, method := range []string{"POST", "HEAD", "DELETE"} {
		resp, _ := apiRequest(t, method, base+"/api/diagnostics", token, nil)
		if resp.StatusCode != 405 || resp.Header.Get("Allow") != "GET" {
			t.Errorf("%s: want 405 and Allow GET", method)
		}
	}
	resp, _ = apiRequest(t, "POST", base+"/api/diagnostics/test", "", `{"transport":"mqtt"}`)
	if resp.StatusCode != 401 {
		t.Fatal("test endpoint accepted an unauthenticated request")
	}
	resp, _ = apiRequest(t, "GET", base+"/api/diagnostics/test", token, nil)
	if resp.StatusCode != 405 || resp.Header.Get("Allow") != "POST" {
		t.Fatal("test endpoint accepted GET")
	}
	for _, body := range []string{`invalid`, `{"transport":"unknown"}`, `{"transport":"mqtt"}`, `{"transport":"telemetry_webhook"}`} {
		resp, _ := apiRequest(t, "POST", base+"/api/diagnostics/test", token, body)
		if resp.StatusCode != 400 {
			t.Errorf("invalid or unconfigured test %s: got %d", body, resp.StatusCode)
		}
	}
	_, body = apiRequest(t, "GET", base+"/api/status", token, nil)
	var status struct {
		Connections map[string]diagnostics.Snapshot `json:"connections"`
	}
	if err := json.Unmarshal(body, &status); err != nil || len(status.Connections) != 2 {
		t.Fatalf("status must include both diagnostics: %v, %s", err, body)
	}
	srv.SetWebUIEnabled(false)
	for _, path := range []string{"/api/diagnostics", "/api/diagnostics/test"} {
		resp, _ := apiRequest(t, "GET", base+path, token, nil)
		if resp.StatusCode != 404 {
			t.Errorf("%s is exposed while the web UI is disabled", path)
		}
	}
}

func TestDiagnosticWebhookTestUsesSavedSettingsWithoutChangingLiveCounters(t *testing.T) {
	var requests atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer private-bearer" || r.URL.RawQuery != "token=private-query" {
			t.Error("test did not use saved webhook settings")
		}
		w.WriteHeader(401)
	}))
	defer receiver.Close()
	cfg := config.DefaultConfig()
	cfg.TelemetryWebhook = config.TelemetryWebhookConfig{Enabled: true, URL: receiver.URL + "/private-path?token=private-query", BearerToken: "private-bearer"}
	srv, base, token := startTestServer(t, cfg)
	webhook := output.NewWebhook(config.Get)
	t.Cleanup(webhook.Close)
	srv.SetTelemetryWebhook(webhook)
	for range 2 {
		resp, body := apiRequest(t, "POST", base+"/api/diagnostics/test", token, `{"transport":"telemetry_webhook","url":"https://ignored.invalid"}`)
		var result struct {
			Success bool   `json:"success"`
			Code    string `json:"code"`
			Hint    string `json:"hint"`
		}
		if err := json.Unmarshal(body, &result); err != nil || resp.StatusCode != 200 || result.Success || result.Code != "authorization" || result.Hint == "" {
			t.Fatalf("test response: %s (%v)", body, err)
		}
		for _, secret := range []string{"private-bearer", "private-query", "private-path", receiver.URL} {
			if strings.Contains(string(body), secret) {
				t.Fatalf("test response leaked %q: %s", secret, body)
			}
		}
	}
	if requests.Load() != 2 || webhook.Diagnostics().Attempts != 0 || webhook.Diagnostics().Publishes != 0 || webhook.Diagnostics().Failures != 0 {
		t.Fatal("test requests changed live delivery counters")
	}
}
