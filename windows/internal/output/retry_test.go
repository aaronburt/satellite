package output

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"satellite/internal/config"
)

func TestIsRetryableClassifiesFailures(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"network", errors.New("dial tcp: connection refused"), true},
		{"408", &StatusError{Code: 408}, true},
		{"425", &StatusError{Code: 425}, true},
		{"429", &StatusError{Code: 429}, true},
		{"500", &StatusError{Code: 500}, true},
		{"503", &StatusError{Code: 503}, true},
		{"400", &StatusError{Code: 400}, false},
		{"401", &StatusError{Code: 401}, false},
		{"404", &StatusError{Code: 404}, false},
		{"bad url", &permanentError{errors.New("bad")}, false},
	}
	for _, c := range cases {
		if got := IsRetryable(c.err); got != c.want {
			t.Errorf("%s: IsRetryable = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRetryDelayBacksOffWithJitterAndCap(t *testing.T) {
	err := errors.New("offline")
	// jitter 1 gives the upper bound, 0 the lower bound (half).
	for next, want := range map[int]time.Duration{2: 2 * time.Second, 3: 4 * time.Second, 4: 8 * time.Second, 10: retryMaxDelay} {
		if got := RetryDelay(next, err, 0.999999); got < want-time.Millisecond || got > want {
			t.Errorf("next=%d upper: got %v, want ~%v", next, got, want)
		}
		if got := RetryDelay(next, err, 0); got != want/2 {
			t.Errorf("next=%d lower: got %v, want %v", next, got, want/2)
		}
	}
}

func TestRetryDelayHonoursRetryAfterWithCap(t *testing.T) {
	if got := RetryDelay(2, &StatusError{Code: 429, RetryAfter: 30 * time.Second}, 0); got != 30*time.Second {
		t.Errorf("Retry-After longer than backoff should win, got %v", got)
	}
	if got := RetryDelay(4, &StatusError{Code: 429, RetryAfter: time.Second}, 1); got < 7*time.Second {
		t.Errorf("shorter Retry-After should not shorten backoff, got %v", got)
	}
	if got := RetryDelay(2, &StatusError{Code: 503, RetryAfter: time.Hour}, 0); got != retryMaxRetryAfter {
		t.Errorf("Retry-After should be capped, got %v", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("12", now); got != 12*time.Second {
		t.Errorf("seconds: %v", got)
	}
	if got := parseRetryAfter(now.Add(90*time.Second).Format(http.TimeFormat), now); got != 90*time.Second {
		t.Errorf("date: %v", got)
	}
	for _, v := range []string{"", "-3", "soon", now.Add(-time.Minute).Format(http.TimeFormat)} {
		if got := parseRetryAfter(v, now); got != 0 {
			t.Errorf("%q: want 0, got %v", v, got)
		}
	}
}

// scripted answers each request with the next status in its list (204 once
// exhausted) and records the payload timestamps it saw.
type scripted struct {
	mu         sync.Mutex
	statuses   []int
	retryAfter string
	stamps     []int64
	cpus       []float64
	auth       []string
}

func (s *scripted) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &recorder{}
	rec.handler(httptest.NewRecorder(), r)
	p := rec.payloads[0]
	s.mu.Lock()
	s.stamps = append(s.stamps, p.Timestamp)
	if p.State.CPUPercent != nil {
		s.cpus = append(s.cpus, *p.State.CPUPercent)
	}
	s.auth = append(s.auth, r.Header.Get("Authorization"))
	code := http.StatusNoContent
	if len(s.statuses) > 0 {
		code, s.statuses = s.statuses[0], s.statuses[1:]
	}
	s.mu.Unlock()
	if s.retryAfter != "" {
		w.Header().Set("Retry-After", s.retryAfter)
	}
	w.WriteHeader(code)
}

func (s *scripted) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.stamps)
}

// retryHarness wires a Webhook to a fake clock: each retry wait is recorded
// and released by the test through hooks.
type retryHarness struct {
	w      *Webhook
	mu     sync.Mutex
	cfg    config.Config
	delays []time.Duration
	now    time.Time
	onWait func() // runs inside the wait, before it is released
}

func newRetryHarness(t *testing.T, url string) *retryHarness {
	h := &retryHarness{now: time.Unix(1_000_000, 0)}
	h.cfg = config.Config{NodeID: "pc", HeartbeatSec: 60, TelemetryWebhook: config.TelemetryWebhookConfig{Enabled: true, URL: url, BearerToken: "a"}}
	h.w = NewWebhook(h.config)
	h.w.now = func() time.Time { h.mu.Lock(); defer h.mu.Unlock(); return h.now }
	h.w.jitter = func() float64 { return 0 }
	h.w.after = func(d time.Duration) <-chan time.Time {
		h.mu.Lock()
		h.delays = append(h.delays, d)
		h.now = h.now.Add(d)
		hook := h.onWait
		h.mu.Unlock()
		if hook != nil {
			hook()
		}
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	t.Cleanup(h.w.Close)
	return h
}

func (h *retryHarness) config() config.Config { h.mu.Lock(); defer h.mu.Unlock(); return h.cfg }

func TestWebhookRetriesTransientFailuresWithOriginalTimestamp(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusRequestTimeout} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			s := &scripted{statuses: []int{status, status}}
			srv := httptest.NewServer(s)
			defer srv.Close()
			h := newRetryHarness(t, srv.URL)

			_ = h.w.Publish(cpu(1))
			waitIdle(t, h.w)

			if s.calls() != 3 {
				t.Fatalf("want 2 failures + 1 success, got %d calls", s.calls())
			}
			if s.stamps[0] != s.stamps[1] || s.stamps[1] != s.stamps[2] {
				t.Errorf("retries must keep the original timestamp: %v", s.stamps)
			}
			if len(h.delays) != 2 || h.delays[0] != time.Second || h.delays[1] != 2*time.Second {
				t.Errorf("unexpected backoff: %v", h.delays)
			}
			if h.w.lastSent.IsZero() {
				t.Error("success should be recorded")
			}
		})
	}
}

func TestWebhookRateLimitWaitsForRetryAfter(t *testing.T) {
	s := &scripted{statuses: []int{http.StatusTooManyRequests}, retryAfter: "20"}
	srv := httptest.NewServer(s)
	defer srv.Close()
	h := newRetryHarness(t, srv.URL)

	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)

	if s.calls() != 2 || len(h.delays) != 1 || h.delays[0] != 20*time.Second {
		t.Fatalf("calls=%d delays=%v", s.calls(), h.delays)
	}
}

func TestWebhookDoesNotRetryPermanentClientErrors(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422} {
		s := &scripted{statuses: []int{status, status}}
		srv := httptest.NewServer(s)
		h := newRetryHarness(t, srv.URL)

		_ = h.w.Publish(cpu(1))
		waitIdle(t, h.w)
		srv.Close()

		if s.calls() != 1 || len(h.delays) != 0 {
			t.Errorf("%d: calls=%d delays=%v", status, s.calls(), h.delays)
		}
	}
}

func TestWebhookOfflineGivesUpAfterMaxAttempts(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listening: connection refused
	h := newRetryHarness(t, url)

	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)

	if len(h.delays) != retryMaxAttempts-1 {
		t.Fatalf("want %d waits, got %v", retryMaxAttempts-1, h.delays)
	}
	if h.w.lastError == "" || !h.w.lastSent.IsZero() {
		t.Error("failure should be recorded and nothing marked sent")
	}
}

func TestWebhookTimeoutIsRetried(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()
	defer close(release)
	h := newRetryHarness(t, srv.URL)
	h.w.client.Timeout = 50 * time.Millisecond

	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 || h.w.lastSent.IsZero() {
		t.Fatalf("timeout should be retried and succeed: calls=%d", calls)
	}
}

func TestWebhookDropsSnapshotsOlderThanMaxAge(t *testing.T) {
	s := &scripted{statuses: []int{500, 500, 500, 500}}
	srv := httptest.NewServer(s)
	defer srv.Close()
	h := newRetryHarness(t, srv.URL)
	// Each wait jumps the clock past the age limit.
	h.w.after = func(d time.Duration) <-chan time.Time {
		h.mu.Lock()
		h.now = h.now.Add(retryMaxAge + time.Second)
		h.mu.Unlock()
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)

	if s.calls() != 1 {
		t.Fatalf("expired snapshot should not be retried, got %d calls", s.calls())
	}
}

func TestWebhookNewerSnapshotReplacesFailedOne(t *testing.T) {
	s := &scripted{statuses: []int{500}}
	srv := httptest.NewServer(s)
	defer srv.Close()
	h := newRetryHarness(t, srv.URL)
	h.onWait = func() {
		h.mu.Lock()
		h.onWait = nil
		h.now = h.now.Add(2 * time.Minute) // heartbeat due
		h.mu.Unlock()
		_ = h.w.Publish(cpu(2))
		_ = h.w.Publish(cpu(3)) // only the newest is kept
	}

	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)

	if s.calls() != 2 || s.cpus[0] != 1 || s.cpus[1] != 3 {
		t.Fatalf("want failed 1 then newest 3, got %v", s.cpus)
	}
	if s.stamps[1] <= s.stamps[0] {
		t.Errorf("newer snapshot keeps its own capture time: %v", s.stamps)
	}
}

func TestWebhookDisableOrURLChangeCancelsRetries(t *testing.T) {
	for name, mutate := range map[string]func(*config.Config){
		"disabled":    func(c *config.Config) { c.TelemetryWebhook.Enabled = false },
		"url changed": func(c *config.Config) { c.TelemetryWebhook.URL = "http://127.0.0.1:1/other" },
	} {
		t.Run(name, func(t *testing.T) {
			s := &scripted{statuses: []int{500, 500}}
			srv := httptest.NewServer(s)
			defer srv.Close()
			h := newRetryHarness(t, srv.URL)
			h.onWait = func() { h.mu.Lock(); mutate(&h.cfg); h.mu.Unlock() }

			_ = h.w.Publish(cpu(1))
			waitIdle(t, h.w)

			if s.calls() != 1 {
				t.Fatalf("retry should be abandoned, got %d calls", s.calls())
			}
		})
	}
}

func TestWebhookTokenChangeIsUsedOnRetry(t *testing.T) {
	s := &scripted{statuses: []int{500}}
	srv := httptest.NewServer(s)
	defer srv.Close()
	h := newRetryHarness(t, srv.URL)
	h.onWait = func() { h.mu.Lock(); h.cfg.TelemetryWebhook.BearerToken = "b"; h.mu.Unlock() }

	_ = h.w.Publish(cpu(1))
	waitIdle(t, h.w)

	if len(s.auth) != 2 || s.auth[1] != "Bearer b" {
		t.Fatalf("auth headers: %v", s.auth)
	}
}

func TestWebhookCloseCancelsPendingRetryAndFutureSends(t *testing.T) {
	s := &scripted{statuses: []int{500, 500}}
	srv := httptest.NewServer(s)
	defer srv.Close()
	h := newRetryHarness(t, srv.URL)
	block := make(chan time.Time)
	h.w.after = func(time.Duration) <-chan time.Time { return block }

	_ = h.w.Publish(cpu(1))
	deadline := time.Now().Add(2 * time.Second)
	for s.calls() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	h.w.Close()
	waitIdle(t, h.w)
	_ = h.w.Publish(cpu(50))
	time.Sleep(20 * time.Millisecond)

	if s.calls() != 1 {
		t.Fatalf("Close should stop retries and new sends, got %d calls", s.calls())
	}
}

func TestWebhookCloseAbortsRequestInFlight(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	h := newRetryHarness(t, srv.URL)
	h.w.client.Timeout = 0

	_ = h.w.Publish(cpu(1))
	time.Sleep(20 * time.Millisecond)
	h.w.Close()
	waitIdle(t, h.w)
	if len(h.delays) != 0 {
		t.Errorf("a cancelled request must not schedule retries: %v", h.delays)
	}
}
