package output

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"satellite/internal/privacy"
	"satellite/internal/telemetry"
)

func TestWebhookRetryFiltersOldReadingsAcrossPauseAndResume(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "while paused", true: "after resume"}[resume], func(t *testing.T) {
			t.Cleanup(privacy.Default.Resume)
			var receivedMu sync.Mutex
			var received []WebhookPayload
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var p WebhookPayload
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Error(err)
				}
				receivedMu.Lock()
				received = append(received, p)
				defer receivedMu.Unlock()
				if len(received) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
				} else {
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer srv.Close()
			h := newRetryHarness(t, srv.URL)
			h.onWait = func() {
				privacy.Default.Pause(time.Minute)
				if resume {
					privacy.Default.Resume()
				}
			}
			snap := cpu(12)
			snap.ActiveWindowTitle = "private document"
			snap.Media = &telemetry.MediaInfo{Title: "private track"}
			if err := h.w.Publish(snap); err != nil {
				t.Fatal(err)
			}
			waitIdle(t, h.w)
			receivedMu.Lock()
			defer receivedMu.Unlock()
			if len(received) != 2 || received[0].State.ActiveWindowTitle == "" {
				t.Fatalf("expected original delivery and retry: %+v", received)
			}
			retried := received[1].State
			if retried.ActiveWindowTitle != "" || retried.Media != nil || retried.CPUPercent == nil {
				t.Fatalf("retry did not mask sensitive readings: %+v", retried)
			}
		})
	}
}
