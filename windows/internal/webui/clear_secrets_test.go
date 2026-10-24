package webui

import (
	"strings"
	"testing"

	"satellite/internal/config"
)

func configWithSecrets() config.Config {
	cfg := config.DefaultConfig()
	cfg.MQTT.Password = "mqtt-pass-xyz"
	cfg.TelemetryWebhook.BearerToken = "tw-token-xyz"
	cfg.Webhook.Secret = "log-secret-xyz"
	return cfg
}

func assertNoSecrets(t *testing.T, body []byte) {
	t.Helper()
	for _, s := range []string{"mqtt-pass-xyz", "tw-token-xyz", "log-secret-xyz", "new-pass"} {
		if strings.Contains(string(body), s) {
			t.Errorf("response leaked %q: %s", s, body)
		}
	}
}

func TestClearSecretsClearsEachSecretIndividually(t *testing.T) {
	cases := []struct {
		name string
		get  func(config.Config) string
	}{
		{"mqtt.password", func(c config.Config) string { return c.MQTT.Password }},
		{"telemetry_webhook.bearer_token", func(c config.Config) string { return c.TelemetryWebhook.BearerToken }},
		{"webhook.secret", func(c config.Config) string { return c.Webhook.Secret }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := configWithSecrets()
			_, base, token := startTestServer(t, before)
			resp, body := apiRequest(t, "POST", base+"/api/config", token,
				map[string]interface{}{"clear_secrets": []string{tc.name}})
			if resp.StatusCode != 200 {
				t.Fatalf("save: %d %s", resp.StatusCode, body)
			}
			saved := config.Get()
			if got := tc.get(saved); got != "" {
				t.Fatalf("%s not cleared", tc.name)
			}
			// The other two secrets are untouched.
			for _, other := range cases {
				if other.name != tc.name && other.get(saved) != other.get(before) {
					t.Errorf("%s changed while clearing %s", other.name, tc.name)
				}
			}
			// Cleared on disk too, not only in memory.
			loaded, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if tc.get(loaded) != "" {
				t.Errorf("%s came back after reload", tc.name)
			}
		})
	}
}

func TestClearSecretsWithOtherChangesInOneRequest(t *testing.T) {
	_, base, token := startTestServer(t, configWithSecrets())
	resp, body := apiRequest(t, "POST", base+"/api/config", token, map[string]interface{}{
		"heartbeat_sec": 42,
		"mqtt":          map[string]interface{}{"password": "new-pass"},
		"clear_secrets": []string{"webhook.secret", "telemetry_webhook.bearer_token", "webhook.secret"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("save: %d %s", resp.StatusCode, body)
	}
	saved := config.Get()
	if saved.HeartbeatSec != 42 || saved.MQTT.Password != "new-pass" ||
		saved.Webhook.Secret != "" || saved.TelemetryWebhook.BearerToken != "" {
		t.Fatalf("unexpected saved config: %+v", saved)
	}
	_, got := apiRequest(t, "GET", base+"/api/config", token, nil)
	assertNoSecrets(t, got)
}

func TestBlankAndOmittedSecretsStillKept(t *testing.T) {
	cfg := configWithSecrets()
	_, base, token := startTestServer(t, cfg)
	for _, payload := range []map[string]interface{}{
		{"heartbeat_sec": 30},
		{"mqtt": map[string]interface{}{"password": ""}, "webhook": map[string]interface{}{"secret": ""},
			"telemetry_webhook": map[string]interface{}{"bearer_token": ""}},
		{"clear_secrets": []string{}},
		{"clear_secrets": nil},
	} {
		if resp, body := apiRequest(t, "POST", base+"/api/config", token, payload); resp.StatusCode != 200 {
			t.Fatalf("save %v: %d %s", payload, resp.StatusCode, body)
		}
		saved := config.Get()
		if saved.MQTT.Password != cfg.MQTT.Password || saved.Webhook.Secret != cfg.Webhook.Secret ||
			saved.TelemetryWebhook.BearerToken != cfg.TelemetryWebhook.BearerToken {
			t.Fatalf("secret lost after %v", payload)
		}
	}
}

func TestClearSecretsRejectsBadRequestsWithoutSaving(t *testing.T) {
	cases := map[string]struct {
		payload interface{}
		want    string
	}{
		"unknown name":   {map[string]interface{}{"clear_secrets": []string{"api_key"}}, "unknown secret"},
		"wrong type":     {`{"clear_secrets":"mqtt.password"}`, "list of strings"},
		"set and clear":  {map[string]interface{}{"mqtt": map[string]interface{}{"password": "new-pass"}, "clear_secrets": []string{"mqtt.password"}}, "both set and cleared"},
		"partly invalid": {map[string]interface{}{"heartbeat_sec": 99, "clear_secrets": []string{"webhook.secret", "nope"}}, "unknown secret"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := configWithSecrets()
			_, base, token := startTestServer(t, cfg)
			before := config.Get()
			resp, body := apiRequest(t, "POST", base+"/api/config", token, tc.payload)
			if resp.StatusCode != 400 || !strings.Contains(string(body), tc.want) {
				t.Fatalf("got %d %s, want 400 containing %q", resp.StatusCode, body, tc.want)
			}
			assertNoSecrets(t, body)
			after := config.Get()
			if after.HeartbeatSec != before.HeartbeatSec || after.Webhook.Secret != before.Webhook.Secret ||
				after.MQTT.Password != before.MQTT.Password {
				t.Fatalf("rejected request changed config")
			}
		})
	}
}
