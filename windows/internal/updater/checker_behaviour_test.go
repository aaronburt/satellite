package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGitHub serves whatever release is currently set and counts requests.
type fakeGitHub struct {
	mu       sync.Mutex
	release  GitHubRelease
	status   int
	requests atomic.Int32
	lastPath string
	lastUA   string
}

func (f *fakeGitHub) set(tag string) {
	f.mu.Lock()
	f.release = GitHubRelease{TagName: tag, HTMLURL: "https://github.com/o/r/releases/tag/" + tag}
	f.mu.Unlock()
}

func (f *fakeGitHub) start(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastPath, f.lastUA = r.URL.Path, r.UserAgent()
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		_ = json.NewEncoder(w).Encode(f.release)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestChecker(t *testing.T, gh *fakeGitHub, current string) (*Checker, chan CheckStatus) {
	notified := make(chan CheckStatus, 10)
	c := NewChecker(" owner/repo ", current, func(s CheckStatus) { notified <- s })
	c.SetAPIBaseURL(gh.start(t).URL + "/")
	return c, notified
}

func expectNotified(t *testing.T, ch chan CheckStatus, tag string) {
	t.Helper()
	select {
	case s := <-ch:
		if s.LatestVersion != tag || !s.Available {
			t.Fatalf("notified with %+v, want %s", s, tag)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected a notification for %s", tag)
	}
}

func expectNotNotified(t *testing.T, ch chan CheckStatus) {
	t.Helper()
	select {
	case s := <-ch:
		t.Fatalf("unexpected notification %+v", s)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCheckRequestsLatestReleaseOfConfiguredRepo(t *testing.T) {
	gh := &fakeGitHub{}
	gh.set("v1.0.0")
	c, _ := newTestChecker(t, gh, "0.9.0")
	if _, err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gh.lastPath != "/repos/owner/repo/releases/latest" {
		t.Errorf("path = %q (repo should be trimmed, base URL slash removed)", gh.lastPath)
	}
	if gh.lastUA != "Satellite-Agent/0.9.0" {
		t.Errorf("User-Agent = %q", gh.lastUA)
	}
}

func TestEachNewVersionIsAnnouncedOnce(t *testing.T) {
	gh := &fakeGitHub{}
	c, notified := newTestChecker(t, gh, "1.0.0")

	gh.set("v1.0.0")
	_, _ = c.Check(context.Background())
	expectNotNotified(t, notified) // same version

	gh.set("v1.1.0")
	_, _ = c.Check(context.Background())
	expectNotified(t, notified, "v1.1.0")
	_, _ = c.Check(context.Background())
	expectNotNotified(t, notified) // already announced

	gh.set("v1.2.0")
	_, _ = c.Check(context.Background())
	expectNotified(t, notified, "v1.2.0")
}

func TestOlderReleaseIsNotAnUpdate(t *testing.T) {
	gh := &fakeGitHub{}
	gh.set("v0.5.0")
	c, notified := newTestChecker(t, gh, "1.0.0")
	s, err := c.Check(context.Background())
	if err != nil || s.Available {
		t.Fatalf("status = %+v, err = %v", s, err)
	}
	expectNotNotified(t, notified)
}

func TestErrorsAreRecordedAndClearedOnSuccess(t *testing.T) {
	gh := &fakeGitHub{status: http.StatusBadGateway}
	c, _ := newTestChecker(t, gh, "1.0.0")

	s, err := c.Check(context.Background())
	if err == nil || !strings.Contains(s.Error, "502") || s.CheckedAt.IsZero() {
		t.Fatalf("status = %+v, err = %v", s, err)
	}
	if c.Status().Error == "" {
		t.Fatal("Status() should expose the last error")
	}

	gh.mu.Lock()
	gh.status = 0
	gh.mu.Unlock()
	gh.set("v1.0.0")
	if s, err := c.Check(context.Background()); err != nil || s.Error != "" {
		t.Fatalf("successful check should clear the error: %+v %v", s, err)
	}
}

func TestOversizedResponseIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","name":"`))
		_, _ = w.Write([]byte(strings.Repeat("x", maxReleaseBody)))
		_, _ = w.Write([]byte(`"}`))
	}))
	defer srv.Close()
	c := NewChecker("o/r", "1.0.0", nil)
	c.SetAPIBaseURL(srv.URL)
	if _, err := c.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckHonoursContextCancellation(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	defer srv.Close()
	defer close(block)

	c := NewChecker("o/r", "1.0.0", nil)
	c.SetAPIBaseURL(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Check(ctx); err == nil {
		t.Fatal("expected cancellation error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("Check did not stop when the context was cancelled")
	}
}

func TestBackgroundChecksStopWhenDisabled(t *testing.T) {
	gh := &fakeGitHub{}
	gh.set("v1.0.0")
	c, _ := newTestChecker(t, gh, "1.0.0")
	c.SetEnabled(false)
	if c.IsEnabled() {
		t.Fatal("SetEnabled(false) ignored")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.StartBackground(ctx, 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if n := gh.requests.Load(); n != 0 {
		t.Fatalf("disabled checker made %d requests", n)
	}

	c.SetEnabled(true)
	deadline := time.Now().Add(2 * time.Second)
	for gh.requests.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if gh.requests.Load() == 0 {
		t.Fatal("re-enabled checker never checked")
	}

	cancel()
	time.Sleep(30 * time.Millisecond)
	after := gh.requests.Load()
	time.Sleep(100 * time.Millisecond)
	if gh.requests.Load() != after {
		t.Fatal("checks continued after the context was cancelled")
	}
}

func TestStartBackgroundChecksImmediately(t *testing.T) {
	gh := &fakeGitHub{}
	gh.set("v2.0.0")
	c, notified := newTestChecker(t, gh, "1.0.0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.StartBackground(ctx, time.Hour)
	expectNotified(t, notified, "v2.0.0")
}

func TestNewCheckerDefaultsRepo(t *testing.T) {
	gh := &fakeGitHub{}
	gh.set("v1.0.0")
	srv := gh.start(t)
	c := NewChecker("  ", "1.0.0", nil)
	c.SetAPIBaseURL(srv.URL)
	_, _ = c.Check(context.Background())
	if gh.lastPath != "/repos/"+DefaultRepo+"/releases/latest" {
		t.Fatalf("path = %q", gh.lastPath)
	}
}
