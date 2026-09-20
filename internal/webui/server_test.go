package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"satellite/internal/config"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
	"satellite/internal/updater"
)

func TestAuthMiddlewareAndEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	origAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", origAppData)
	os.Setenv("APPDATA", tmpDir)

	_ = config.Save(config.DefaultConfig())
	collector := telemetry.NewCollector()
	client := mqtt.NewClient()
	server := NewServer(collector, client)

	port, err := server.Start(0)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	if port <= 0 {
		t.Errorf("expected positive port, got %d", port)
	}

	token := server.Token()
	if token == "" {
		t.Fatalf("expected non-empty ephemeral token")
	}

	reqNoAuth, _ := http.NewRequest("GET", "/api/status", nil)
	rrNoAuth := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrNoAuth, reqNoAuth)
	if rrNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized without token, got %d", rrNoAuth.Code)
	}

	reqAuth, _ := http.NewRequest("GET", "/api/status?token="+token, nil)
	rrAuth := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrAuth, reqAuth)
	if rrAuth.Code != http.StatusOK {
		t.Errorf("expected 200 OK with token, got %d", rrAuth.Code)
	}

	var statusResp map[string]interface{}
	if err := json.Unmarshal(rrAuth.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if _, ok := statusResp["mqtt_status"]; !ok {
		t.Errorf("expected mqtt_status in status response")
	}

	reqCfg, _ := http.NewRequest("GET", "/api/config?token="+token, nil)
	rrCfg := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrCfg, reqCfg)
	if rrCfg.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /api/config, got %d", rrCfg.Code)
	}

	var cfgResp config.Config
	if err := json.Unmarshal(rrCfg.Body.Bytes(), &cfgResp); err != nil {
		t.Fatalf("failed to decode config response: %v", err)
	}
	if !cfgResp.Expose.Webcam {
		t.Errorf("expected webcam expose to be true")
	}

	reqUpdate, _ := http.NewRequest("GET", "/api/update?token="+token, nil)
	rrUpdate := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUpdate, reqUpdate)
	if rrUpdate.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /api/update, got %d", rrUpdate.Code)
	}

	var updateResp map[string]interface{}
	if err := json.Unmarshal(rrUpdate.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if _, ok := updateResp["available"]; !ok {
		t.Errorf("expected available field in update response")
	}

	checker := updater.NewChecker("aaronburt/satellite", "0.14.0", nil)
	server.SetUpdater(checker)
	if !checker.IsEnabled() {
		t.Errorf("expected checker to be enabled by default")
	}

	cfgResp.CheckUpdates = false
	bodyBytes, _ := json.Marshal(cfgResp)
	reqSave, _ := http.NewRequest("POST", "/api/config?token="+token, bytes.NewReader(bodyBytes))
	rrSave := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrSave, reqSave)
	if rrSave.Code != http.StatusOK {
		t.Errorf("expected 200 OK for POST /api/config, got %d", rrSave.Code)
	}

	if checker.IsEnabled() {
		t.Errorf("expected checker to be disabled after saving config with CheckUpdates: false")
	}

	savedCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if savedCfg.CheckUpdates {
		t.Errorf("expected saved config CheckUpdates to be false")
	}
}
