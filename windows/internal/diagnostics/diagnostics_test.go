package diagnostics

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTrackerFailureRecoveryAndBoundedHistory(t *testing.T) {
	stats := New()
	stats.Attempt()
	stats.Connected()
	stats.Disconnected()
	for range 8 {
		stats.Failed("connect", Explain(errors.New("private-error-text")))
	}
	stats.RetryAt(time.Now().Add(time.Second))
	failed := stats.Snapshot("disconnected")
	if failed.Failures != 8 || len(failed.RecentErrors) != 5 || failed.CurrentError == nil || failed.NextRetry == nil {
		t.Fatalf("failure diagnostics: %+v", failed)
	}
	stats.Attempt()
	stats.Connected()
	stats.Published()
	recovered := stats.Snapshot("connected")
	if recovered.Attempts != 2 || recovered.Reconnects != 1 || recovered.Publishes != 1 || recovered.CurrentError != nil || recovered.NextRetry != nil {
		t.Fatalf("recovery diagnostics: %+v", recovered)
	}
	if recovered.LastPublished == nil || recovered.LastConnected == nil || recovered.LastDisconnected == nil || recovered.LastAttempt == nil {
		t.Fatal("missing lifecycle timestamps")
	}
	recovered.RecentErrors[0].Message = "changed"
	if stats.Snapshot("").RecentErrors[0].Message == "changed" {
		t.Fatal("snapshot aliases the tracker history")
	}
	stats.Failed("publish", Explain(errors.New("failed write")))
	stats.Connected()
	if stats.Snapshot("").CurrentError == nil {
		t.Fatal("connecting must not clear a failed telemetry write")
	}
	stats.Published()
	if stats.Snapshot("").CurrentError != nil {
		t.Fatal("successful telemetry write must clear the active delivery error")
	}
}

func TestProblemsDoNotExposeErrorText(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{&net.DNSError{Err: "secret", Name: "private-host"}, "dns"},
		{fmt.Errorf("secret: %w", context.DeadlineExceeded), "timeout"},
		{x509.UnknownAuthorityError{}, "tls"},
		{&url.Error{Op: "parse", URL: "https://user:password@host/path?token=secret", Err: errors.New("bad")}, "address"},
		{errors.New("https://user:password@host/path?token=secret: connection refused"), "refused"},
		{errors.New("private-error-text"), "connection"},
	}
	for _, tc := range cases {
		p := Explain(tc.err)
		if p.Code != tc.code || p.Message == "" || p.Hint == "" {
			t.Errorf("problem = %+v, want %s", p, tc.code)
		}
		data, _ := json.Marshal(p)
		for _, secret := range []string{"password", "token=secret", "private-host", "private-error-text", "https://"} {
			if strings.Contains(string(data), secret) {
				t.Errorf("diagnostic leaked %q: %s", secret, data)
			}
		}
	}
}

func TestTrackerConcurrentReadersAndWriters(t *testing.T) {
	stats := New()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 100 {
				stats.Attempt()
				stats.Failed("publish", HTTPProblem(503))
				stats.Published()
				_ = stats.Snapshot("healthy")
			}
		})
	}
	wg.Wait()
	if got := stats.Snapshot(""); got.Attempts != 400 || got.Publishes != 400 || got.Failures != 400 {
		t.Fatalf("lost concurrent updates: %+v", got)
	}
}
