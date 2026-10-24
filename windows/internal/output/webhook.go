package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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
//
// Failed deliveries are retried in the background following the policy in
// retry.go. At most one request runs at a time and at most one newer
// snapshot waits behind it, so memory stays bounded and MQTT is never
// delayed.
type Webhook struct {
	getConfig func() config.Config
	client    *http.Client
	now       func() time.Time
	after     func(time.Duration) <-chan time.Time
	jitter    func() float64

	ctx    context.Context
	cancel context.CancelFunc

	inFlight atomic.Bool

	mu        sync.Mutex
	pending   *WebhookPayload // newest snapshot queued behind the running delivery
	pendSnap  telemetry.Snapshot
	lastSnap  telemetry.Snapshot
	lastSent  time.Time
	lastURL   string
	lastError string
}

func NewWebhook(getConfig func() config.Config) *Webhook {
	ctx, cancel := context.WithCancel(context.Background())
	return &Webhook{
		getConfig: getConfig,
		client:    &http.Client{Timeout: 10 * time.Second},
		now:       time.Now,
		after:     time.After,
		jitter:    rand.Float64,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (w *Webhook) Name() string { return "webhook" }

// Close cancels any running request or pending retry. Publish is a no-op
// afterwards.
func (w *Webhook) Close() { w.cancel() }

// Publish sends the snapshot in the background when it has changed enough or
// the heartbeat is due. If a delivery is already running (or waiting to
// retry), the snapshot replaces any earlier queued one and is sent next.
func (w *Webhook) Publish(snap telemetry.Snapshot) error {
	if w.ctx.Err() != nil {
		return nil
	}
	cfg := w.getConfig()
	wh := cfg.TelemetryWebhook
	url := strings.TrimSpace(wh.URL)
	if !wh.Enabled || url == "" {
		return nil
	}
	if !w.due(cfg, url, snap) {
		return nil
	}
	payload := BuildPayload(cfg.NodeID, snap, w.now())

	w.mu.Lock()
	if w.inFlight.Load() {
		w.pending = &payload
		w.pendSnap = snap
		w.mu.Unlock()
		return nil
	}
	w.inFlight.Store(true)
	w.mu.Unlock()

	go w.deliver(url, wh.BearerToken, payload, snap)
	return nil
}

// deliver runs one delivery, its retries, and any snapshot queued behind it.
func (w *Webhook) deliver(url, token string, payload WebhookPayload, snap telemetry.Snapshot) {
	defer func() {
		w.mu.Lock()
		w.pending = nil
		w.inFlight.Store(false)
		w.mu.Unlock()
	}()

	attempt, failures := 1, 0
	for {
		if w.ctx.Err() != nil {
			return
		}
		err := send(w.ctx, w.client, url, token, payload)
		if err == nil {
			w.recordSuccess(url, snap)
			failures = 0
			next, nextSnap, ok := w.takePending()
			if !ok {
				return
			}
			payload, snap, attempt = next, nextSnap, 1
			if url, token, ok = w.stillTarget(url, token); !ok {
				return
			}
			continue
		}
		if w.ctx.Err() != nil {
			return
		}
		w.recordFailure(err)
		failures++

		if next, nextSnap, ok := w.takePending(); ok {
			// A newer snapshot supersedes the failed one.
			payload, snap, attempt = next, nextSnap, 0
		} else if !IsRetryable(err) || attempt >= retryMaxAttempts {
			return
		}

		select {
		case <-w.ctx.Done():
			return
		case <-w.after(RetryDelay(failures+1, err, w.jitter())):
		}

		if next, nextSnap, ok := w.takePending(); ok {
			payload, snap, attempt = next, nextSnap, 0
		}
		if w.now().Sub(time.UnixMilli(payload.Timestamp)) > retryMaxAge {
			return
		}
		var ok bool
		if url, token, ok = w.stillTarget(url, token); !ok {
			return
		}
		attempt++
	}
}

// stillTarget re-reads the config. Delivery stops if the webhook was
// disabled or its URL changed; a changed token is picked up.
func (w *Webhook) stillTarget(url, token string) (string, string, bool) {
	wh := w.getConfig().TelemetryWebhook
	if !wh.Enabled || strings.TrimSpace(wh.URL) != url {
		return url, token, false
	}
	return url, wh.BearerToken, true
}

func (w *Webhook) takePending() (WebhookPayload, telemetry.Snapshot, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending == nil {
		return WebhookPayload{}, telemetry.Snapshot{}, false
	}
	p, s := *w.pending, w.pendSnap
	w.pending = nil
	return p, s, true
}

func (w *Webhook) recordFailure(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	msg := err.Error()
	if msg != w.lastError {
		logger.Warn("webhook", fmt.Sprintf("Telemetry webhook failed: %v", err))
	}
	w.lastError = msg
}

func (w *Webhook) recordSuccess(url string, snap telemetry.Snapshot) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lastError != "" {
		logger.Info("webhook", "Telemetry webhook recovered")
	}
	w.lastError = ""
	w.lastSnap = snap
	w.lastSent = w.now()
	w.lastURL = url
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
// Non-2xx answers are returned as *StatusError.
func Send(client *http.Client, url, bearerToken string, payload WebhookPayload) error {
	return send(context.Background(), client, url, bearerToken, payload)
}

func send(ctx context.Context, client *http.Client, url, bearerToken string, payload WebhookPayload) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return &permanentError{fmt.Errorf("webhook URL is empty")}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return &permanentError{err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &permanentError{err}
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
		return &StatusError{Code: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
	}
	return nil
}

// SendTest POSTs a single snapshot right away, ignoring change detection.
func SendTest(wh config.TelemetryWebhookConfig, nodeID string, snap telemetry.Snapshot) error {
	client := &http.Client{Timeout: 10 * time.Second}
	return Send(client, wh.URL, wh.BearerToken, BuildPayload(nodeID, snap, time.Now()))
}
