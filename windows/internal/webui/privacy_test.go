package webui

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/privacy"
	"satellite/internal/screenthumb"
)

func TestTrayPrivacyPauseHidesPreviewsAndPreservesSettings(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	cfg := configWith(allExpose())
	srv, base, token := startTestServer(t, cfg)
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	store.Set(testJPEG)
	srv.SetThumbnailStore(store)
	before := config.Get()
	srv.PausePrivacy(5 * time.Minute)
	if !reflect.DeepEqual(before, config.Get()) {
		t.Fatal("privacy pause changed saved configuration")
	}
	resp, body := apiRequest(t, http.MethodGet, base+"/api/status?thumb=1", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status response: %d %s", resp.StatusCode, body)
	}
	var status struct {
		Pause   privacy.Status `json:"privacy_pause"`
		Sensors []Sensor       `json:"sensors"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if !status.Pause.Paused || status.Pause.RemainingSeconds < 299 || status.Pause.RemainingSeconds > 300 {
		t.Fatalf("pause status: %+v", status.Pause)
	}
	for _, sensor := range status.Sensors {
		switch sensor.ID {
		case "user_presence", "session_lock", "media_control", "active_window", "window_title", "microphone", "webcam", "screen_thumbnail":
			t.Errorf("paused sensor was listed: %s", sensor.ID)
		}
	}
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("paused preview returned %d", resp.StatusCode)
	}
	srv.PausePrivacy(0)
	if privacy.Default.Status().Paused {
		t.Fatal("early resume did not clear pause")
	}
	if _, _, ok := store.Latest(); ok {
		t.Fatal("resume restored a stale preview")
	}
	store.Set(testJPEG)
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("fresh preview after resume returned %d", resp.StatusCode)
	}
}

func TestPrivacyOverviewSeparatesSavedPausedUnsupportedAndCommands(t *testing.T) {
	cfg := configWith(allExpose())
	caps := allCaps()
	caps.Webcam = false
	report := privacyOverview(cfg, caps, true, false)
	permissions := map[string]privacyPermission{}
	for _, permission := range report["permissions"].([]privacyPermission) {
		permissions[permission.Key] = permission
	}
	if p := permissions["window_title"]; !p.Enabled || !p.Supported || !p.Paused {
		t.Fatalf("window title permission = %+v", p)
	}
	if p := permissions["cpu"]; !p.Enabled || !p.Supported || p.Paused {
		t.Fatalf("CPU permission = %+v", p)
	}
	if p := permissions["webcam"]; !p.Enabled || p.Supported {
		t.Fatalf("webcam permission = %+v", p)
	}
	if !report["media_commands"].(map[string]bool)["media_prev"] {
		t.Fatal("privacy pause incorrectly disabled an allowed media command")
	}
	cfg.Expose.MediaPermissions = &config.MediaPermissions{}
	if privacyOverview(cfg, caps, false, false)["media_commands"].(map[string]bool)["media_prev"] {
		t.Fatal("denied media command reported as allowed")
	}
}

func TestPrivacyOverviewDoesNotExposeDestinationSecrets(t *testing.T) {
	cfg := configWith(allExpose())
	cfg.MQTT.Broker = "mqtts://broker-user:broker-pass@broker.example:8883/private-topic"
	cfg.TelemetryWebhook = config.TelemetryWebhookConfig{Enabled: true, URL: "https://hook-user:hook-pass@hooks.example/private-path?token=private-query#private-fragment", BearerToken: "private-bearer"}
	cfg.Webhook = config.WebhookConfig{Enabled: true, URL: "https://logs.example/private-log-path", Secret: "private-secret"}
	cfg.APIKey = "private-api-key"
	raw, err := json.Marshal(privacyOverview(cfg, allCaps(), false, false))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"broker-user", "broker-pass", "private-topic", "hook-user", "hook-pass", "private-path", "private-query", "private-fragment", "private-bearer", "private-log-path", "private-secret", "private-api-key"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("privacy overview leaked %q", secret)
		}
	}
	for _, host := range []string{"broker.example", "hooks.example", "logs.example"} {
		if !strings.Contains(string(raw), host) {
			t.Errorf("missing destination %q", host)
		}
	}
}

func TestPrivacyStatusUsesJSONDatasetWithoutEnablingJSON(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(allExpose()))
	srv.SetJSONEnabled(false)
	resp, body := apiRequest(t, http.MethodGet, base+"/api/status", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d %s", resp.StatusCode, body)
	}
	var status struct {
		Telemetry json.RawMessage `json:"telemetry"`
		Privacy   struct {
			Outputs []struct {
				Name    string `json:"name"`
				Enabled bool   `json:"enabled"`
			} `json:"outputs"`
		} `json:"privacy"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(srv.collector.Last())
	if err != nil {
		t.Fatal(err)
	}
	if string(expected) != string(status.Telemetry) {
		t.Fatalf("privacy dataset differs from collector: %s", status.Telemetry)
	}
	if len(status.Privacy.Outputs) != 4 {
		t.Fatalf("outputs = %+v", status.Privacy.Outputs)
	}
	for _, output := range status.Privacy.Outputs {
		if output.Name == "JSON API" && output.Enabled {
			t.Fatal("JSON API reported as enabled")
		}
	}
	if resp, _ := apiRequest(t, http.MethodGet, base+"/json", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("JSON API unexpectedly enabled: %d", resp.StatusCode)
	}
}
