package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"satellite/internal/config"
)

type Event struct {
	Timestamp int64  `json:"timestamp"`
	Time      string `json:"time"`
	NodeID    string `json:"node_id"`
	Level     string `json:"level"`
	Category  string `json:"category"`
	Message   string `json:"message"`
}

type Listener func(Event)

var (
	mu          sync.RWMutex
	enabled     bool
	url         string
	minLevel    string
	secret      string
	nodeID      string
	httpClient  = &http.Client{Timeout: 5 * time.Second}
	listenersMu sync.RWMutex
	listeners   = make(map[uint64]Listener)
	listenerSeq uint64
)

func AddListener(fn Listener) func() {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	listenerSeq++
	id := listenerSeq
	listeners[id] = fn
	return func() {
		listenersMu.Lock()
		defer listenersMu.Unlock()
		delete(listeners, id)
	}
}

func Init(cfg config.WebhookConfig, id string) {
	UpdateConfig(cfg, id)
}

func UpdateConfig(cfg config.WebhookConfig, id string) {
	mu.Lock()
	defer mu.Unlock()
	enabled = cfg.Enabled
	url = strings.TrimSpace(cfg.URL)
	minLevel = strings.ToLower(strings.TrimSpace(cfg.MinLevel))
	if minLevel == "" {
		minLevel = "info"
	}
	secret = strings.TrimSpace(cfg.Secret)
	nodeID = id
}

func levelScore(lvl string) int {
	switch strings.ToUpper(lvl) {
	case "ERROR":
		return 3
	case "WARN", "WARNING":
		return 2
	case "INFO":
		return 1
	default:
		return 0
	}
}

func shouldLog(lvl string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if !enabled || url == "" {
		return false
	}
	return levelScore(lvl) >= levelScore(minLevel)
}

func Log(level, category, message string) {
	mu.RLock()
	id := nodeID
	targetURL := url
	targetSecret := secret
	webhookEnabled := enabled && url != "" && levelScore(level) >= levelScore(minLevel)
	mu.RUnlock()

	listenersMu.RLock()
	hasListeners := len(listeners) > 0
	listenersMu.RUnlock()

	if !webhookEnabled && !hasListeners {
		return
	}

	event := Event{
		Timestamp: time.Now().Unix(),
		Time:      time.Now().UTC().Format(time.RFC3339),
		NodeID:    id,
		Level:     strings.ToUpper(level),
		Category:  category,
		Message:   message,
	}

	if hasListeners {
		listenersMu.RLock()
		for _, fn := range listeners {
			go fn(event)
		}
		listenersMu.RUnlock()
	}

	if webhookEnabled {
		go func() {
			_ = postEvent(targetURL, targetSecret, event)
		}()
	}
}

func Info(category, message string) {
	Log("INFO", category, message)
}

func Warn(category, message string) {
	Log("WARN", category, message)
}

func Error(category, message string) {
	Log("ERROR", category, message)
}

func buildPayload(targetURL string, event Event) ([]byte, error) {
	lowerURL := strings.ToLower(targetURL)
	if strings.Contains(lowerURL, "discord.com/api/webhooks") || strings.Contains(lowerURL, "discordapp.com/api/webhooks") {
		content := fmt.Sprintf("[%s] **%s** (%s): %s", event.Level, event.NodeID, event.Category, event.Message)
		return json.Marshal(map[string]string{"content": content})
	}
	if strings.Contains(lowerURL, "hooks.slack.com") {
		text := fmt.Sprintf("[%s] *%s* (%s): %s", event.Level, event.NodeID, event.Category, event.Message)
		return json.Marshal(map[string]string{"text": text})
	}
	return json.Marshal(event)
}

func postEvent(targetURL, targetSecret string, event Event) error {
	body, err := buildPayload(targetURL, event)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if targetSecret != "" {
		req.Header.Set("Authorization", "Bearer "+targetSecret)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func SendTestWebhook(cfg config.WebhookConfig, id string) error {
	trimmedURL := strings.TrimSpace(cfg.URL)
	if trimmedURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	event := Event{
		Timestamp: time.Now().Unix(),
		Time:      time.Now().UTC().Format(time.RFC3339),
		NodeID:    id,
		Level:     "INFO",
		Category:  "test",
		Message:   "Satellite test webhook message",
	}

	return postEvent(trimmedURL, strings.TrimSpace(cfg.Secret), event)
}
