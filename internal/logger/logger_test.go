package logger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"satellite/internal/config"
)

func TestLoggerLevelsAndFiltering(t *testing.T) {
	Init(config.WebhookConfig{
		Enabled:  false,
		URL:      "http://example.com",
		MinLevel: "info",
	}, "node-1")

	if shouldLog("INFO") {
		t.Fatal("expected shouldLog to be false when disabled")
	}

	UpdateConfig(config.WebhookConfig{
		Enabled:  true,
		URL:      "",
		MinLevel: "info",
	}, "node-1")

	if shouldLog("INFO") {
		t.Fatal("expected shouldLog to be false when url is empty")
	}

	UpdateConfig(config.WebhookConfig{
		Enabled:  true,
		URL:      "http://example.com",
		MinLevel: "",
	}, "node-1")

	if !shouldLog("INFO") {
		t.Fatal("expected shouldLog to default to info")
	}

	UpdateConfig(config.WebhookConfig{
		Enabled:  true,
		URL:      "http://example.com",
		MinLevel: "warn",
	}, "node-1")

	if shouldLog("INFO") {
		t.Fatal("expected info to be filtered out when minLevel is warn")
	}
	if !shouldLog("WARN") {
		t.Fatal("expected warn to pass")
	}
	if !shouldLog("WARNING") {
		t.Fatal("expected warning to pass")
	}
	if !shouldLog("ERROR") {
		t.Fatal("expected error to pass")
	}
	if shouldLog("DEBUG") {
		t.Fatal("expected unknown level to be filtered")
	}
}

func TestBuildPayloadFormats(t *testing.T) {
	evt := Event{
		Timestamp: 1234567890,
		Time:      "2026-01-01T00:00:00Z",
		NodeID:    "test-node",
		Level:     "INFO",
		Category:  "test-cat",
		Message:   "hello world",
	}

	discordURL := "https://discord.com/api/webhooks/123/abc"
	discordBytes, err := buildPayload(discordURL, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var discordMap map[string]string
	if err := json.Unmarshal(discordBytes, &discordMap); err != nil {
		t.Fatalf("invalid discord json: %v", err)
	}
	if discordMap["content"] == "" {
		t.Fatal("expected content field in discord payload")
	}

	slackURL := "https://hooks.slack.com/services/123"
	slackBytes, err := buildPayload(slackURL, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var slackMap map[string]string
	if err := json.Unmarshal(slackBytes, &slackMap); err != nil {
		t.Fatalf("invalid slack json: %v", err)
	}
	if slackMap["text"] == "" {
		t.Fatal("expected text field in slack payload")
	}

	genericURL := "https://api.example.com/webhook"
	genericBytes, err := buildPayload(genericURL, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var genericEvt Event
	if err := json.Unmarshal(genericBytes, &genericEvt); err != nil {
		t.Fatalf("invalid generic json: %v", err)
	}
	if genericEvt.Message != "hello world" {
		t.Fatalf("expected message hello world, got %s", genericEvt.Message)
	}
}

func TestPostEventAndDispatch(t *testing.T) {
	var receivedCount int32
	var lastAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&receivedCount, 1)
		lastAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	UpdateConfig(config.WebhookConfig{
		Enabled:  true,
		URL:      server.URL,
		MinLevel: "info",
		Secret:   "secret-token",
	}, "test-node")

	Info("general", "info test")
	Warn("general", "warn test")
	Error("general", "error test")

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 3 {
		t.Fatalf("expected 3 events received, got %d", atomic.LoadInt32(&receivedCount))
	}
	if lastAuthHeader != "Bearer secret-token" {
		t.Fatalf("expected Bearer secret-token, got %s", lastAuthHeader)
	}

	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()

	err := postEvent(errServer.URL, "", Event{Message: "fail"})
	if err == nil {
		t.Fatal("expected error on 500 status")
	}

	err = postEvent("http://127.0.0.1:0", "", Event{Message: "fail"})
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestSendTestWebhook(t *testing.T) {
	err := SendTestWebhook(config.WebhookConfig{URL: ""}, "node-1")
	if err == nil {
		t.Fatal("expected error when URL is empty")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err = SendTestWebhook(config.WebhookConfig{URL: server.URL}, "node-1")
	if err != nil {
		t.Fatalf("unexpected error sending test webhook: %v", err)
	}
}
