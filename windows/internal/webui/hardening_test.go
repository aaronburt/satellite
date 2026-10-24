package webui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/logger"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
)

// startTestServer saves cfg to a temp config dir and starts a real server
// on 127.0.0.1. It returns the server, its base URL and its token.
func startTestServer(t *testing.T, cfg config.Config) (*Server, string, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	logger.UpdateConfig(config.WebhookConfig{}, "test-node")

	cfg.BindAddress = "127.0.0.1"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	srv := NewServer(telemetry.NewCollector(), mqtt.NewClient())
	port, err := srv.Start(0)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(srv.Stop)
	return srv, fmt.Sprintf("http://127.0.0.1:%d", port), srv.Token()
}

var testClient = &http.Client{Timeout: 5 * time.Second}

func apiRequest(t *testing.T, method, url, token string, body interface{}) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(b)
	default:
		data, _ := json.Marshal(b)
		reader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("X-API-Token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// Closing the log stream while events are being logged used to write to the
// response after the handler returned, which panics and kills the agent.
func TestLogStreamClientDisconnectDoesNotCrash(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					logger.Info("burst", "event")
				}
			}
		}()
	}

	for i := 0; i < 25; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", base+"/api/logs/stream?token="+token, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			t.Fatalf("connect: %v", err)
		}
		buf := make([]byte, 512)
		_, _ = resp.Body.Read(buf)
		cancel()
		resp.Body.Close()
	}
	close(stop)
	wg.Wait()
	// Give any late writes a chance to happen; a panic here fails the run.
	time.Sleep(100 * time.Millisecond)
}

func TestLogStreamDeliversEventsInOrder(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/api/logs/stream?token="+token, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type = %q", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() || scanner.Text() != ": connected" {
		t.Fatalf("expected connected comment, got %q", scanner.Text())
	}

	want := []string{"e1", "e2", "e3", "e4", "e5"}
	for _, msg := range want {
		logger.Info("order-test", msg)
	}

	var got []string
	deadline := time.AfterFunc(3*time.Second, cancel)
	defer deadline.Stop()
	for len(got) < len(want) && scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev logger.Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatalf("bad event %q: %v", line, err)
		}
		if ev.Category == "order-test" {
			got = append(got, ev.Message)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

// The page doesn't show every config field. Saving must not wipe the ones
// it leaves out.
func TestSaveKeepsSettingsThePageDoesNotSend(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MQTT.ClientID = "custom-client"
	cfg.HeartbeatSec = 17
	cfg.UpdateRepo = "someone/fork"
	cfg.Webhook = config.WebhookConfig{Enabled: true, URL: "https://hooks.example/x", MinLevel: "warn", Secret: "s3cret"}
	_, base, token := startTestServer(t, cfg)

	payload := map[string]interface{}{
		"node_id":      "node-a",
		"mqtt":         map[string]interface{}{"broker": "", "username": "", "password": "", "topic_prefix": "satellite"},
		"interval_sec": 5,
		"port":         0,
		"webui_port":   0,
	}
	if resp, body := apiRequest(t, "POST", base+"/api/config", token, payload); resp.StatusCode != 200 {
		t.Fatalf("save: %d %s", resp.StatusCode, body)
	}
	saved := config.Get()
	if saved.MQTT.ClientID != "custom-client" || saved.HeartbeatSec != 17 || saved.UpdateRepo != "someone/fork" {
		t.Fatalf("hidden settings lost: client=%q heartbeat=%d repo=%q", saved.MQTT.ClientID, saved.HeartbeatSec, saved.UpdateRepo)
	}
	if saved.Webhook != cfg.Webhook {
		t.Fatalf("log webhook changed: %+v", saved.Webhook)
	}

	// The page sends the log webhook with a blank secret: keep the saved one.
	payload["webhook"] = map[string]interface{}{"enabled": true, "url": "https://hooks.example/x", "min_level": "error", "secret": ""}
	apiRequest(t, "POST", base+"/api/config", token, payload)
	saved = config.Get()
	if saved.Webhook.Secret != "s3cret" || saved.Webhook.MinLevel != "error" {
		t.Fatalf("unexpected log webhook after save: %+v", saved.Webhook)
	}
}

func TestConfigGetRedactsEverySecret(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MQTT.Password = "mqtt-pass-xyz"
	cfg.Webhook.Secret = "log-secret-xyz"
	cfg.TelemetryWebhook.BearerToken = "tw-token-xyz"
	_, base, token := startTestServer(t, cfg)

	_, body := apiRequest(t, "GET", base+"/api/config", token, nil)
	for _, secret := range []string{"mqtt-pass-xyz", "log-secret-xyz", "tw-token-xyz"} {
		if strings.Contains(string(body), secret) {
			t.Errorf("GET /api/config leaked %q", secret)
		}
	}
}

func TestIndexUsesStrictCSPWithNonce(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())

	fetch := func() (*http.Response, string) {
		resp, err := testClient.Get(base + "/?token=" + token)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp, string(body)
	}

	resp, body := fetch()
	csp := resp.Header.Get("Content-Security-Policy")
	m := regexp.MustCompile(`script-src 'nonce-([^']+)'`).FindStringSubmatch(csp)
	if m == nil {
		t.Fatalf("CSP has no script nonce: %q", csp)
	}
	if !strings.Contains(body, `<script nonce="`+m[1]+`">`) {
		t.Fatal("page script tag does not carry the CSP nonce")
	}
	scriptSrc := regexp.MustCompile(`script-src[^;]*`).FindString(csp)
	if strings.Contains(scriptSrc, "unsafe-inline") {
		t.Fatalf("script-src must not allow unsafe-inline: %q", scriptSrc)
	}
	if strings.Contains(body, "{{") {
		t.Fatal("page has an unreplaced template marker")
	}
	// Inline handlers are blocked by the CSP, so the page must not use any.
	if loc := regexp.MustCompile(`<[^>]+\son[a-z]+\s*=`).FindStringIndex(body); loc != nil {
		t.Fatalf("page has an inline event handler: %q", body[loc[0]:loc[1]])
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}

	resp2, _ := fetch()
	if resp2.Header.Get("Content-Security-Policy") == csp {
		t.Fatal("nonce must change on every request")
	}
}

func TestSecurityHeadersOnAPI(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())
	resp, _ := apiRequest(t, "GET", base+"/api/status", token, nil)
	want := map[string]string{
		"Referrer-Policy":        "no-referrer",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Cache-Control":          "no-store",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestDiskHTMLOnlyInDevMode(t *testing.T) {
	srv, base, token := startTestServer(t, config.DefaultConfig())

	dir := t.TempDir()
	path := filepath.Join(dir, filepath.FromSlash(devIndexPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("DISK-COPY"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	get := func() string {
		resp, err := testClient.Get(base + "/?token=" + token)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	if strings.Contains(get(), "DISK-COPY") {
		t.Fatal("served the disk copy outside dev mode")
	}
	srv.SetDevMode(true)
	if !strings.Contains(get(), "DISK-COPY") {
		t.Fatal("dev mode should serve the disk copy")
	}
}

// Changing the port used to close the request's own connection and throw
// away the token, leaving the page dead.
func TestPortChangeKeepsTokenAndAnswers(t *testing.T) {
	srv, base, token := startTestServer(t, config.DefaultConfig())
	oldPort := srv.Port()
	newPort := freePort(t)

	resp, body := apiRequest(t, "POST", base+"/api/config", token, map[string]interface{}{"port": newPort, "webui_port": newPort})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %d %s", resp.StatusCode, body)
	}
	var result struct {
		Success bool         `json:"success"`
		Listen  listenResult `json:"listen"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Success || !result.Listen.Moved || result.Listen.Port != newPort || result.Listen.Host != "127.0.0.1" {
		t.Fatalf("unexpected result: %s", body)
	}

	if srv.Token() != token {
		t.Fatal("token changed on restart")
	}
	newBase := fmt.Sprintf("http://127.0.0.1:%d", newPort)
	if resp, _ := apiRequest(t, "GET", newBase+"/api/status", token, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("new address with old token: %d", resp.StatusCode)
	}

	// The old server's Serve returns after the move; it must not mark the
	// new server as stopped.
	time.Sleep(200 * time.Millisecond)
	if !srv.IsRunning() {
		t.Fatal("server reported as stopped after moving")
	}
	if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", oldPort), time.Second); err == nil {
		conn.Close()
		t.Fatal("old port still accepting connections")
	}
}

func TestBindChangeOnSamePort(t *testing.T) {
	srv, base, token := startTestServer(t, config.DefaultConfig())
	port := srv.Port()

	resp, body := apiRequest(t, "POST", base+"/api/config", token, map[string]interface{}{"bind_address": "0.0.0.0", "port": port, "webui_port": port})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %d %s", resp.StatusCode, body)
	}
	var result struct {
		Listen listenResult `json:"listen"`
	}
	_ = json.Unmarshal(body, &result)
	if !result.Listen.Moved || result.Listen.Port != port || result.Listen.Host != "" {
		t.Fatalf("unexpected listen result: %s", body)
	}
	if resp, _ := apiRequest(t, "GET", base+"/api/status", token, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("same port after bind change: %d", resp.StatusCode)
	}
}

func TestUnusableListenAddressIsNotSaved(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())

	for _, bind := range []string{"not-an-ip", "192.0.2.1"} {
		resp, body := apiRequest(t, "POST", base+"/api/config", token, map[string]interface{}{"bind_address": bind})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("bind %q: expected 400, got %d %s", bind, resp.StatusCode, body)
		}
		if got := config.Get().BindAddress; got != "127.0.0.1" {
			t.Fatalf("bind %q was saved as %q", bind, got)
		}
		if resp, _ := apiRequest(t, "GET", base+"/api/status", token, nil); resp.StatusCode != http.StatusOK {
			t.Fatalf("server stopped answering after bad bind %q", bind)
		}
	}
}

func TestTestEndpointsDoNotSendSavedSecretsElsewhere(t *testing.T) {
	type hit struct{ auth string }
	var mu sync.Mutex
	hits := map[string][]hit{}
	hook := func(name string) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			hits[name] = append(hits[name], hit{r.Header.Get("Authorization")})
			mu.Unlock()
		}))
		t.Cleanup(s.Close)
		return s
	}
	saved, other := hook("saved"), hook("other")
	last := func(name string) string {
		mu.Lock()
		defer mu.Unlock()
		h := hits[name]
		if len(h) == 0 {
			t.Fatalf("%s got no request", name)
		}
		return h[len(h)-1].auth
	}

	cfg := config.DefaultConfig()
	cfg.TelemetryWebhook = config.TelemetryWebhookConfig{URL: saved.URL, BearerToken: "tw-secret"}
	cfg.Webhook = config.WebhookConfig{URL: saved.URL, Secret: "log-secret"}
	_, base, token := startTestServer(t, cfg)

	cases := []struct {
		path, field, secret string
	}{
		{"/api/test-telemetry-webhook", "bearer_token", "tw-secret"},
		{"/api/test-webhook", "secret", "log-secret"},
	}
	for _, c := range cases {
		apiRequest(t, "POST", base+c.path, token, map[string]string{"url": other.URL})
		if got := last("other"); got != "" {
			t.Errorf("%s sent the saved secret to a new URL: %q", c.path, got)
		}
		apiRequest(t, "POST", base+c.path, token, map[string]string{})
		if got := last("saved"); got != "Bearer "+c.secret {
			t.Errorf("%s with blank URL: auth = %q", c.path, got)
		}
		apiRequest(t, "POST", base+c.path, token, map[string]string{"url": saved.URL})
		if got := last("saved"); got != "Bearer "+c.secret {
			t.Errorf("%s with saved URL: auth = %q", c.path, got)
		}
	}
}

func TestWithSavedMQTTPassword(t *testing.T) {
	saved := config.MQTTConfig{Broker: "tcp://home:1883", Username: "ha", Password: "pw"}
	cases := []struct {
		name string
		req  config.MQTTConfig
		want string
	}{
		{"same broker and user", config.MQTTConfig{Broker: " tcp://home:1883 ", Username: "ha"}, "pw"},
		{"other broker", config.MQTTConfig{Broker: "tcp://evil:1883", Username: "ha"}, ""},
		{"other user", config.MQTTConfig{Broker: "tcp://home:1883", Username: "root"}, ""},
		{"explicit password", config.MQTTConfig{Broker: "tcp://evil:1883", Username: "ha", Password: "typed"}, "typed"},
	}
	for _, c := range cases {
		if got := withSavedMQTTPassword(c.req, saved).Password; got != c.want {
			t.Errorf("%s: password = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSecretMatches(t *testing.T) {
	cases := []struct {
		provided, expected string
		want               bool
	}{
		{"abc", "abc", true},
		{"abd", "abc", false},
		{"ab", "abc", false},
		{"", "", false},
		{"abc", "", false},
		{"", "abc", false},
	}
	for _, c := range cases {
		if got := secretMatches(c.provided, c.expected); got != c.want {
			t.Errorf("secretMatches(%q, %q) = %v", c.provided, c.expected, got)
		}
	}
}

func TestJSONImportResetsNodeIDByDefault(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())

	other := config.DefaultConfig()
	other.NodeID = "some-other-machine"
	export, err := config.ExportConfig(other, "")
	if err != nil {
		t.Fatal(err)
	}
	resp, body := apiRequest(t, "POST", base+"/api/config/import", token, map[string]string{"content": string(export)})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import: %d %s", resp.StatusCode, body)
	}
	if got, want := config.Get().NodeID, config.DefaultConfig().NodeID; got != want {
		t.Fatalf("node id = %q, want this machine's %q", got, want)
	}
}

func TestRequestBodyIsCapped(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())
	huge := `{"node_id":"` + strings.Repeat("a", maxBodyBytes+10) + `"}`
	resp, _ := apiRequest(t, "POST", base+"/api/config", token, huge)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", resp.StatusCode)
	}
}

func TestErrorResponsesAreJSON(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())
	for _, path := range []string{"/api/test-connection", "/api/test-webhook", "/api/action", "/api/config"} {
		resp, body := apiRequest(t, "POST", base+path, token, "not json")
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s: content type %q", path, ct)
		}
		if !json.Valid(body) {
			t.Errorf("%s: body is not JSON: %s", path, body)
		}
	}
}

func TestExportFilenameIsHeaderSafe(t *testing.T) {
	got := exportFilename("pc\"1\r\n/../x")
	if got != "satellite-config-pc_1___.._x.satconfig" {
		t.Fatalf("filename = %q", got)
	}
	if got := exportFilename(""); got != "satellite-config-node.satconfig" {
		t.Fatalf("empty node id filename = %q", got)
	}
}

func TestBrowseHostAndValidateListen(t *testing.T) {
	hosts := map[string]string{
		"":            "127.0.0.1",
		"127.0.0.1":   "127.0.0.1",
		"0.0.0.0":     "",
		"::":          "",
		"[::1]":       "::1",
		"192.168.1.5": "192.168.1.5",
		"localhost":   "localhost",
	}
	for in, want := range hosts {
		if got := browseHost(in); got != want {
			t.Errorf("browseHost(%q) = %q, want %q", in, got, want)
		}
	}

	valid := []string{"", "127.0.0.1", "0.0.0.0", "::", "[::1]", "localhost", "192.168.1.5"}
	for _, b := range valid {
		if err := validateListen(b, 8080); err != nil {
			t.Errorf("validateListen(%q) = %v", b, err)
		}
	}
	for _, b := range []string{"example.com", "127.0.0.1:80", "not an ip"} {
		if err := validateListen(b, 8080); err == nil {
			t.Errorf("validateListen(%q) accepted", b)
		}
	}
	if err := validateListen("127.0.0.1", 70000); err == nil {
		t.Error("port 70000 accepted")
	}
}

func TestURLPointsAtBindAddress(t *testing.T) {
	srv, _, token := startTestServer(t, config.DefaultConfig())
	want := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", srv.Port(), token)
	if got := srv.URL(); got != want {
		t.Fatalf("URL() = %q, want %q", got, want)
	}

	srv.mu.Lock()
	srv.host = "::1"
	srv.mu.Unlock()
	if got := srv.URL(); !strings.HasPrefix(got, fmt.Sprintf("http://[::1]:%d/", srv.Port())) {
		t.Fatalf("IPv6 URL = %q", got)
	}
}

func TestStopForgetsToken(t *testing.T) {
	srv, _, token := startTestServer(t, config.DefaultConfig())
	srv.Stop()
	if srv.IsRunning() || srv.Token() != "" {
		t.Fatal("Stop should stop the server and clear the token")
	}
	if _, err := srv.Start(0); err != nil {
		t.Fatal(err)
	}
	if srv.Token() == token {
		t.Fatal("a fresh start should issue a new token")
	}
}

func TestReadEndpointsEnforceGET(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())
	paths := []string{"/api/status", "/api/capabilities", "/api/logs/stream"}

	for _, path := range paths {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead} {
			before := logger.ListenerCount()
			resp, _ := apiRequest(t, method, base+path, token, nil)
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status %d, want 405", method, path, resp.StatusCode)
			}
			if got := resp.Header.Get("Allow"); got != http.MethodGet {
				t.Errorf("%s %s: Allow %q, want GET", method, path, got)
			}
			if after := logger.ListenerCount(); after != before {
				t.Errorf("%s %s: listener count %d -> %d", method, path, before, after)
			}
		}
		// Auth still runs first: a bad token gets 401, not 405.
		resp, _ := apiRequest(t, http.MethodPost, base+path, "wrong", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("POST %s bad token: status %d, want 401", path, resp.StatusCode)
		}
	}

	for _, path := range paths[:2] {
		resp, _ := apiRequest(t, http.MethodGet, base+path, token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestReadEndpointsHiddenWhenWebUIDisabled(t *testing.T) {
	srv, base, token := startTestServer(t, config.DefaultConfig())
	srv.SetWebUIEnabled(false)
	for _, path := range []string{"/api/status", "/api/capabilities", "/api/logs/stream"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			resp, _ := apiRequest(t, method, base+path, token, nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s %s with web UI off: status %d, want 404", method, path, resp.StatusCode)
			}
		}
	}
}
