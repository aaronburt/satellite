package logger

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"satellite/internal/config"
)

type received struct {
	auth        string
	contentType string
	body        []byte
}

// webhookServer records every request it receives on a channel.
func webhookServer(t *testing.T, status int) (*httptest.Server, chan received) {
	t.Helper()
	ch := make(chan received, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ch <- received{r.Header.Get("Authorization"), r.Header.Get("Content-Type"), body}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, ch
}

func expectRequest(t *testing.T, ch chan received) received {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(3 * time.Second):
		t.Fatal("expected a webhook request")
		return received{}
	}
}

func expectNoRequest(t *testing.T, ch chan received) {
	t.Helper()
	select {
	case r := <-ch:
		t.Fatalf("unexpected webhook request: %s", r.body)
	case <-time.After(150 * time.Millisecond):
	}
}

func resetLogger(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { UpdateConfig(config.WebhookConfig{}, "") })
}

func TestShouldLog(t *testing.T) {
	resetLogger(t)
	cases := []struct {
		name string
		cfg  config.WebhookConfig
		lvl  string
		want bool
	}{
		{"disabled", config.WebhookConfig{Enabled: false, URL: "http://x"}, "ERROR", false},
		{"no url", config.WebhookConfig{Enabled: true, URL: "  "}, "ERROR", false},
		{"default min level is info", config.WebhookConfig{Enabled: true, URL: "http://x"}, "INFO", true},
		{"info filtered at warn", config.WebhookConfig{Enabled: true, URL: "http://x", MinLevel: "warn"}, "INFO", false},
		{"warn passes warn", config.WebhookConfig{Enabled: true, URL: "http://x", MinLevel: "WARN"}, "WARN", true},
		{"warning alias", config.WebhookConfig{Enabled: true, URL: "http://x", MinLevel: "warn"}, "warning", true},
		{"error passes warn", config.WebhookConfig{Enabled: true, URL: "http://x", MinLevel: " warn "}, "ERROR", true},
		{"error only", config.WebhookConfig{Enabled: true, URL: "http://x", MinLevel: "error"}, "WARN", false},
		{"unknown level filtered", config.WebhookConfig{Enabled: true, URL: "http://x"}, "DEBUG", false},
	}
	for _, tc := range cases {
		UpdateConfig(tc.cfg, "n")
		if got := shouldLog(tc.lvl); got != tc.want {
			t.Errorf("%s: shouldLog(%q) = %v, want %v", tc.name, tc.lvl, got, tc.want)
		}
	}
}

func TestLogPostsEventsAboveMinLevel(t *testing.T) {
	resetLogger(t)
	srv, ch := webhookServer(t, http.StatusOK)
	Init(config.WebhookConfig{Enabled: true, URL: " " + srv.URL + " ", MinLevel: "warn", Secret: " s3cret "}, "desk")

	Info("general", "ignored")
	expectNoRequest(t, ch)

	Warn("mqtt", "broker unreachable")
	r := expectRequest(t, ch)
	if r.auth != "Bearer s3cret" {
		t.Errorf("Authorization = %q, want trimmed bearer secret", r.auth)
	}
	if r.contentType != "application/json" {
		t.Errorf("Content-Type = %q", r.contentType)
	}
	var ev Event
	if err := json.Unmarshal(r.body, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Level != "WARN" || ev.Category != "mqtt" || ev.Message != "broker unreachable" || ev.NodeID != "desk" {
		t.Fatalf("event = %+v", ev)
	}
	if ev.Timestamp == 0 {
		t.Fatal("missing timestamp")
	}
	if _, err := time.Parse(time.RFC3339, ev.Time); err != nil {
		t.Fatalf("time %q is not RFC3339: %v", ev.Time, err)
	}

	Error("system", "boom")
	if ev := expectRequest(t, ch); !strings.Contains(string(ev.body), `"level":"ERROR"`) {
		t.Fatalf("error event body = %s", ev.body)
	}
}

func TestLogWithoutSecretSendsNoAuthorization(t *testing.T) {
	resetLogger(t)
	srv, ch := webhookServer(t, http.StatusOK)
	UpdateConfig(config.WebhookConfig{Enabled: true, URL: srv.URL}, "n")
	Info("c", "m")
	if r := expectRequest(t, ch); r.auth != "" {
		t.Fatalf("Authorization = %q, want none", r.auth)
	}
}

func TestBuildPayloadFormats(t *testing.T) {
	ev := Event{Timestamp: 1, Time: "t", NodeID: "desk", Level: "WARN", Category: "mqtt", Message: "hello"}
	cases := []struct {
		url, key, want string
	}{
		{"https://discord.com/api/webhooks/1/abc", "content", "[WARN] **desk** (mqtt): hello"},
		{"https://DISCORDAPP.com/api/webhooks/1/abc", "content", "[WARN] **desk** (mqtt): hello"},
		{"https://hooks.slack.com/services/T/B/X", "text", "[WARN] *desk* (mqtt): hello"},
	}
	for _, tc := range cases {
		b, err := buildPayload(tc.url, ev)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if len(m) != 1 || m[tc.key] != tc.want {
			t.Errorf("%s: payload = %v, want {%s: %q}", tc.url, m, tc.key, tc.want)
		}
	}

	b, _ := buildPayload("https://example.com/hook", ev)
	var generic Event
	if err := json.Unmarshal(b, &generic); err != nil || generic != ev {
		t.Fatalf("generic payload = %s (%v)", b, err)
	}
}

func TestPostEventErrors(t *testing.T) {
	srv, _ := webhookServer(t, http.StatusInternalServerError)
	if err := postEvent(srv.URL, "", Event{}); err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("500 response: err = %v", err)
	}
	if err := postEvent("http://127.0.0.1:1", "", Event{}); err == nil {
		t.Fatal("connection refused should be an error")
	}
	if err := postEvent("::not a url", "", Event{}); err == nil {
		t.Fatal("invalid URL should be an error")
	}
}

func TestSendTestWebhook(t *testing.T) {
	if err := SendTestWebhook(config.WebhookConfig{URL: "   "}, "n"); err == nil {
		t.Fatal("empty URL should fail")
	}
	srv, ch := webhookServer(t, http.StatusOK)
	if err := SendTestWebhook(config.WebhookConfig{URL: srv.URL, Secret: "abc"}, "desk"); err != nil {
		t.Fatal(err)
	}
	r := expectRequest(t, ch)
	var ev Event
	_ = json.Unmarshal(r.body, &ev)
	if r.auth != "Bearer abc" || ev.Category != "test" || ev.NodeID != "desk" {
		t.Fatalf("test webhook = %+v auth=%q", ev, r.auth)
	}

	bad, _ := webhookServer(t, http.StatusForbidden)
	if err := SendTestWebhook(config.WebhookConfig{URL: bad.URL}, "n"); err == nil {
		t.Fatal("non-2xx should be reported to the user")
	}
}

func TestListenersReceiveEventsInOrderAndCanUnsubscribe(t *testing.T) {
	resetLogger(t)
	UpdateConfig(config.WebhookConfig{}, "desk") // webhook off; listeners still get events

	var mu sync.Mutex
	var got []string
	unsub := AddListener(func(ev Event) {
		mu.Lock()
		got = append(got, ev.Level+":"+ev.Message)
		mu.Unlock()
	})
	for i, lvl := range []string{"info", "warn", "error", "debug"} {
		Log(lvl, "cat", string(rune('a'+i)))
	}
	unsub()
	Info("cat", "after unsubscribe")

	mu.Lock()
	defer mu.Unlock()
	want := []string{"INFO:a", "WARN:b", "ERROR:c", "DEBUG:d"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("listener got %v, want %v", got, want)
	}
}

func TestMultipleListenersAndConcurrentLogging(t *testing.T) {
	resetLogger(t)
	var mu sync.Mutex
	counts := map[int]int{}
	var unsubs []func()
	for i := 0; i < 3; i++ {
		id := i
		unsubs = append(unsubs, AddListener(func(Event) {
			mu.Lock()
			counts[id]++
			mu.Unlock()
		}))
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); Info("c", "m") }()
	}
	wg.Wait()
	for _, u := range unsubs {
		u()
	}
	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < 3; i++ {
		if counts[i] != 50 {
			t.Fatalf("listener %d got %d events, want 50", i, counts[i])
		}
	}
}
