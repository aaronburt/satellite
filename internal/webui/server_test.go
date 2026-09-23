package webui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"satellite/internal/capabilities"
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

	samePort, err := server.Start(0)
	if err != nil || samePort != port {
		t.Fatalf("expected same port when already running")
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

	reqCaps, _ := http.NewRequest("GET", "/api/capabilities?token="+token, nil)
	rrCaps := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrCaps, reqCaps)
	if rrCaps.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/capabilities, got %d", rrCaps.Code)
	}

	reqRoot, _ := http.NewRequest("GET", "/?token="+token, nil)
	rrRoot := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrRoot, reqRoot)
	if rrRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", rrRoot.Code)
	}

	reqRootBadToken, _ := http.NewRequest("GET", "/?token=badtoken", nil)
	rrRootBadToken := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrRootBadToken, reqRootBadToken)
	if rrRootBadToken.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad token on root")
	}

	reqRootInvalidPath, _ := http.NewRequest("GET", "/nonexistent?token="+token, nil)
	rrRootInvalidPath := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrRootInvalidPath, reqRootInvalidPath)
	if rrRootInvalidPath.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent path")
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

	reqUpdate, _ := http.NewRequest("GET", "/api/update?token="+token, nil)
	rrUpdate := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUpdate, reqUpdate)
	if rrUpdate.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /api/update, got %d", rrUpdate.Code)
	}

	checker := updater.NewChecker("aaronburt/satellite", "0.14.0", nil)
	server.SetUpdater(checker)
	if !checker.IsEnabled() {
		t.Errorf("expected checker to be enabled by default")
	}

	reqUpdateWithChecker, _ := http.NewRequest("GET", "/api/update?token="+token, nil)
	rrUpdateWithChecker := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUpdateWithChecker, reqUpdateWithChecker)
	if rrUpdateWithChecker.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /api/update with checker, got %d", rrUpdateWithChecker.Code)
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

func TestActionsAndUpdatesEndpoints(t *testing.T) {
	collector := telemetry.NewCollector()
	client := mqtt.NewClient()
	server := NewServer(collector, client)
	server.SetCapabilities(capabilities.Detect())
	_, _ = server.Start(0)
	defer server.Stop()
	token := server.Token()

	reqBadActionJSON, _ := http.NewRequest("POST", "/api/action?token="+token, bytes.NewReader([]byte("invalid json")))
	rrBadActionJSON := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrBadActionJSON, reqBadActionJSON)
	if rrBadActionJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad action json")
	}

	reqToast, _ := http.NewRequest("POST", "/api/action?token="+token, bytes.NewReader([]byte(`{"action":"test_toast"}`)))
	rrToast := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrToast, reqToast)
	if rrToast.Code != http.StatusOK {
		t.Fatalf("expected 200 for test_toast action")
	}

	reqUnknownAction, _ := http.NewRequest("POST", "/api/action?token="+token, bytes.NewReader([]byte(`{"action":"unknown_action"}`)))
	rrUnknownAction := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUnknownAction, reqUnknownAction)
	if rrUnknownAction.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown action")
	}

	client.Registry().Register("custom_mock", func() error { return nil })
	reqCustomAction, _ := http.NewRequest("POST", "/api/action?token="+token, bytes.NewReader([]byte(`{"action":"custom_mock"}`)))
	rrCustomAction := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrCustomAction, reqCustomAction)
	if rrCustomAction.Code != http.StatusOK {
		t.Fatalf("expected 200 for registered custom action")
	}

	reqMethodAction, _ := http.NewRequest("GET", "/api/action?token="+token, nil)
	rrMethodAction := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrMethodAction, reqMethodAction)
	if rrMethodAction.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET /api/action")
	}

	reqUpdateCheckNoChecker, _ := http.NewRequest("POST", "/api/update/check?token="+token, nil)
	rrUpdateCheckNoChecker := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUpdateCheckNoChecker, reqUpdateCheckNoChecker)
	if rrUpdateCheckNoChecker.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when update checker is nil")
	}

	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v99.0.0", "html_url": "https://example.com"}`))
	}))
	defer ghServer.Close()

	checker := updater.NewChecker("test/repo", "0.1.0", nil)
	checker.SetAPIBaseURL(ghServer.URL)
	server.SetUpdater(checker)

	reqUpdateCheck, _ := http.NewRequest("POST", "/api/update/check?token="+token, nil)
	rrUpdateCheck := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUpdateCheck, reqUpdateCheck)
	if rrUpdateCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 for update check, got %d", rrUpdateCheck.Code)
	}

	reqUpdateMethod, _ := http.NewRequest("PUT", "/api/update?token="+token, nil)
	rrUpdateMethod := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUpdateMethod, reqUpdateMethod)
	if rrUpdateMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for PUT /api/update")
	}

	reqUpdateCheckMethod, _ := http.NewRequest("GET", "/api/update/check?token="+token, nil)
	rrUpdateCheckMethod := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrUpdateCheckMethod, reqUpdateCheckMethod)
	if rrUpdateCheckMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET /api/update/check")
	}
}

func TestTestConnectionAndWebhook(t *testing.T) {
	collector := telemetry.NewCollector()
	client := mqtt.NewClient()
	server := NewServer(collector, client)
	_, _ = server.Start(0)
	defer server.Stop()
	token := server.Token()

	reqBadConnJSON, _ := http.NewRequest("POST", "/api/test-connection?token="+token, bytes.NewReader([]byte("invalid")))
	rrBadConnJSON := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrBadConnJSON, reqBadConnJSON)
	if rrBadConnJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad connection json")
	}

	reqBadConn, _ := http.NewRequest("POST", "/api/test-connection?token="+token, bytes.NewReader([]byte(`{"broker":"127.0.0.1","port":65534}`)))
	rrBadConn := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrBadConn, reqBadConn)
	if rrBadConn.Code != http.StatusOK {
		t.Fatalf("expected 200 with success: false")
	}

	reqConnMethod, _ := http.NewRequest("GET", "/api/test-connection?token="+token, nil)
	rrConnMethod := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrConnMethod, reqConnMethod)
	if rrConnMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET /api/test-connection")
	}

	reqBadWebhookJSON, _ := http.NewRequest("POST", "/api/test-webhook?token="+token, bytes.NewReader([]byte("invalid")))
	rrBadWebhookJSON := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrBadWebhookJSON, reqBadWebhookJSON)
	if rrBadWebhookJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad webhook json")
	}

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	validWebhookPayload := fmt.Sprintf(`{"url":"%s"}`, webhookServer.URL)
	reqValidWebhook, _ := http.NewRequest("POST", "/api/test-webhook?token="+token, bytes.NewReader([]byte(validWebhookPayload)))
	rrValidWebhook := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrValidWebhook, reqValidWebhook)
	if rrValidWebhook.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid test webhook")
	}

	reqWebhookMethod, _ := http.NewRequest("GET", "/api/test-webhook?token="+token, nil)
	rrWebhookMethod := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrWebhookMethod, reqWebhookMethod)
	if rrWebhookMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET /api/test-webhook")
	}
}

func TestJSONEndpointAndWebUIDisabled(t *testing.T) {
	collector := telemetry.NewCollector()
	client := mqtt.NewClient()
	server := NewServer(collector, client)
	_, _ = server.Start(0)
	defer server.Stop()
	token := server.Token()

	reqJSONDisabled, _ := http.NewRequest("GET", "/json", nil)
	rrJSONDisabled := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrJSONDisabled, reqJSONDisabled)
	if rrJSONDisabled.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when JSON endpoint is disabled, got %d", rrJSONDisabled.Code)
	}

	reqJSONBadMethod, _ := http.NewRequest("POST", "/json", nil)
	rrJSONBadMethod := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrJSONBadMethod, reqJSONBadMethod)
	if rrJSONBadMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST /json")
	}

	server.SetJSONEnabled(true)
	server.SetAPIKey("secret-api-key")

	reqJSONNoKey, _ := http.NewRequest("GET", "/json", nil)
	rrJSONNoKey := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrJSONNoKey, reqJSONNoKey)
	if rrJSONNoKey.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when API key is missing")
	}

	reqJSONWithHeader, _ := http.NewRequest("GET", "/json", nil)
	reqJSONWithHeader.Header.Set("X-API-Key", "secret-api-key")
	rrJSONWithHeader := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrJSONWithHeader, reqJSONWithHeader)
	if rrJSONWithHeader.Code != http.StatusOK {
		t.Fatalf("expected 200 with X-API-Key header")
	}

	reqJSONWithQuery, _ := http.NewRequest("GET", "/json?api_key=secret-api-key", nil)
	rrJSONWithQuery := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrJSONWithQuery, reqJSONWithQuery)
	if rrJSONWithQuery.Code != http.StatusOK {
		t.Fatalf("expected 200 with api_key query param")
	}

	server.SetWebUIEnabled(false)
	reqWebUIDisabled, _ := http.NewRequest("GET", "/api/status?token="+token, nil)
	rrWebUIDisabled := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrWebUIDisabled, reqWebUIDisabled)
	if rrWebUIDisabled.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for api endpoint when webUI is disabled")
	}

	reqRootWebUIDisabled, _ := http.NewRequest("GET", "/?token="+token, nil)
	rrRootWebUIDisabled := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrRootWebUIDisabled, reqRootWebUIDisabled)
	if rrRootWebUIDisabled.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for root when webUI is disabled")
	}
}

func TestConfigBadRequestsAndPortChange(t *testing.T) {
	collector := telemetry.NewCollector()
	client := mqtt.NewClient()
	server := NewServer(collector, client)
	_, _ = server.Start(0)
	defer server.Stop()
	token := server.Token()

	reqBadConfigJSON, _ := http.NewRequest("POST", "/api/config?token="+token, bytes.NewReader([]byte("not json")))
	rrBadConfigJSON := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrBadConfigJSON, reqBadConfigJSON)
	if rrBadConfigJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad config json")
	}

	reqConfigBadMethod, _ := http.NewRequest("DELETE", "/api/config?token="+token, nil)
	rrConfigBadMethod := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrConfigBadMethod, reqConfigBadMethod)
	if rrConfigBadMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for DELETE /api/config")
	}

	cfg := config.DefaultConfig()
	cfg.JSONEnabled = true
	cfg.APIKey = ""
	cfg.BindAddress = ""
	cfg.NodeID = ""
	cfg.IntervalSec = 0
	bodyBytes, _ := json.Marshal(cfg)
	reqSaveDefault, _ := http.NewRequest("POST", "/api/config?token="+token, bytes.NewReader(bodyBytes))
	rrSaveDefault := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rrSaveDefault, reqSaveDefault)
	if rrSaveDefault.Code != http.StatusOK {
		t.Fatalf("expected 200 for saving config with defaults")
	}
}
