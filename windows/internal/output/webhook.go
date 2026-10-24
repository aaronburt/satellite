package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"satellite/internal/config"
	"satellite/internal/logger"
	"satellite/internal/telemetry"
)

// WebhookPayload is the JSON body POSTed to the telemetry webhook. The
// envelope matches the Android app's webhook payload so one receiver can
// handle both apps.
type WebhookPayload struct {
	DeviceID   string             `json:"device_id"`
	DeviceName string             `json:"device_name"`
	Platform   string             `json:"platform"`
	AppVersion string             `json:"app_version"`
	Timestamp  int64              `json:"timestamp"` // Unix milliseconds, as on Android.
	State      telemetry.Snapshot `json:"state"`
}

// Webhook is a Sink that POSTs snapshots to config.TelemetryWebhook.URL.
// It reads the live config on every publish, so settings changes apply on
// the next tick without a restart.
type Webhook struct {
	getConfig func() config.Config
	client    *http.Client
	now       func() time.Time

	inFlight atomic.Bool

	mu        sync.Mutex
	lastSnap  telemetry.Snapshot
	lastSent  time.Time
	lastURL   string
	lastError string
}

func NewWebhook(getConfig func() config.Config) *Webhook {
	return &Webhook{
		getConfig: getConfig,
		client:    &http.Client{Timeout: 10 * time.Second},
		now:       time.Now,
	}
}

func (w *Webhook) Name() string { return "webhook" }

// Publish sends the snapshot in the background when it has changed enough or
// the heartbeat is due. If the previous POST is still running the snapshot
// is skipped, so a slow endpoint never delays MQTT.
func (w *Webhook) Publish(snap telemetry.Snapshot) error {
	cfg := w.getConfig()
	wh := cfg.TelemetryWebhook
	url := strings.TrimSpace(wh.URL)
	if !wh.Enabled || url == "" {
		return nil
	}
	if !w.due(cfg, url, snap) {
		return nil
	}
	if !w.inFlight.CompareAndSwap(false, true) {
		return nil
	}

	go func() {
		defer w.inFlight.Store(false)
		err := Send(w.client, url, wh.BearerToken, BuildPayload(cfg.NodeID, snap, w.now()))
		w.mu.Lock()
		defer w.mu.Unlock()
		if err != nil {
			msg := err.Error()
			if msg != w.lastError {
				logger.Warn("webhook", fmt.Sprintf("Telemetry webhook failed: %v", err))
			}
			w.lastError = msg
			return
		}
		if w.lastError != "" {
			logger.Info("webhook", "Telemetry webhook recovered")
		}
		w.lastError = ""
		w.lastSnap = snap
		w.lastSent = w.now()
		w.lastURL = url
	}()
	return nil
}

func (w *Webhook) due(cfg config.Config, url string, snap telemetry.Snapshot) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	if w.lastSent.IsZero() || url != w.lastURL {
		return true
	}
	elapsed := now.Sub(w.lastSent)

	if minGap := time.Duration(cfg.TelemetryWebhook.MinIntervalSec) * time.Second; elapsed < minGap {
		return false
	}

	heartbeat := time.Duration(cfg.HeartbeatSec) * time.Second
	if heartbeat <= 0 {
		heartbeat = 60 * time.Second
	}
	return elapsed >= heartbeat || telemetry.HasSignificantDelta(w.lastSnap, snap)
}

// BuildPayload wraps a snapshot in the webhook envelope.
func BuildPayload(nodeID string, snap telemetry.Snapshot, now time.Time) WebhookPayload {
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = nodeID
	}
	return WebhookPayload{
		DeviceID:   nodeID,
		DeviceName: name,
		Platform:   runtime.GOOS,
		AppVersion: config.Version,
		Timestamp:  now.UnixMilli(),
		State:      snap,
	}
}

// Send POSTs one payload and returns an error unless the endpoint answers 2xx.
func Send(client *http.Client, url, bearerToken string, payload WebhookPayload) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("webhook URL is empty")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Satellite/"+config.Version)
	if token := strings.TrimSpace(bearerToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// SendTest POSTs a single snapshot right away, ignoring change detection.
func SendTest(wh config.TelemetryWebhookConfig, nodeID string, snap telemetry.Snapshot) error {
	client := &http.Client{Timeout: 10 * time.Second}
	return Send(client, wh.URL, wh.BearerToken, BuildPayload(nodeID, snap, time.Now()))
}
