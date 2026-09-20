package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestChecker_CheckUpdateAvailable(t *testing.T) {
	var notifyCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}
		rel := GitHubRelease{
			TagName: "v0.15.0",
			Name:    "Release v0.15.0",
			HTMLURL: "https://github.com/aaronburt/satellite/releases/tag/v0.15.0",
		}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	checker := NewChecker("aaronburt/satellite", "0.14.0", func(status CheckStatus) {
		mu.Lock()
		notifyCount++
		mu.Unlock()
	})
	checker.SetAPIBaseURL(server.URL)

	status, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected check error: %v", err)
	}

	if !status.Available {
		t.Errorf("expected update to be available")
	}
	if status.LatestVersion != "v0.15.0" {
		t.Errorf("expected latest version v0.15.0, got %s", status.LatestVersion)
	}
	if status.ReleaseURL != "https://github.com/aaronburt/satellite/releases/tag/v0.15.0" {
		t.Errorf("unexpected release URL: %s", status.ReleaseURL)
	}

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if notifyCount != 1 {
		t.Errorf("expected 1 notification callback, got %d", notifyCount)
	}
	mu.Unlock()

	_, err = checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected check error on second check: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if notifyCount != 1 {
		t.Errorf("expected notification callback not to fire again for same version, got %d", notifyCount)
	}
	mu.Unlock()
}

func TestChecker_CheckUpToDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := GitHubRelease{
			TagName: "v0.14.0",
			Name:    "Release v0.14.0",
			HTMLURL: "https://github.com/aaronburt/satellite/releases/tag/v0.14.0",
		}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	checker := NewChecker("aaronburt/satellite", "0.14.0", nil)
	checker.SetAPIBaseURL(server.URL)

	status, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected check error: %v", err)
	}

	if status.Available {
		t.Errorf("expected update NOT to be available")
	}
}

func TestChecker_DraftAndPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := GitHubRelease{
			TagName:    "v0.16.0",
			Name:       "Prerelease v0.16.0",
			HTMLURL:    "https://github.com/aaronburt/satellite/releases/tag/v0.16.0",
			Prerelease: true,
		}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	checker := NewChecker("", "0.14.0", nil)
	checker.SetAPIBaseURL(server.URL)

	status, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected check error: %v", err)
	}
	if status.Available {
		t.Errorf("expected prerelease to be ignored")
	}
}

func TestChecker_HTTPErrorCodes(t *testing.T) {
	codeMap := map[int]string{
		http.StatusForbidden: "rate limit",
		http.StatusNotFound:  "not found",
		http.StatusInternalServerError: "failed with status 500",
	}

	for code := range codeMap {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))

		checker := NewChecker("aaronburt/satellite", "0.14.0", nil)
		checker.SetAPIBaseURL(server.URL)

		status, err := checker.Check(context.Background())
		server.Close()

		if err == nil {
			t.Errorf("expected error for code %d", code)
		}
		if status.Error == "" {
			t.Errorf("expected status.Error to be populated for code %d", code)
		}
	}
}

func TestChecker_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	checker := NewChecker("aaronburt/satellite", "0.14.0", nil)
	checker.SetAPIBaseURL(server.URL)

	_, err := checker.Check(context.Background())
	if err == nil {
		t.Errorf("expected JSON unmarshal error")
	}
}

func TestChecker_StartBackground(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := GitHubRelease{
			TagName: "v0.14.0",
			Name:    "v0.14.0",
			HTMLURL: "https://github.com/aaronburt/satellite/releases/tag/v0.14.0",
		}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	checker := NewChecker("aaronburt/satellite", "0.14.0", nil)
	checker.SetAPIBaseURL(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	checker.StartBackground(ctx, 50*time.Millisecond)
	time.Sleep(120 * time.Millisecond)
	cancel()

	status := checker.Status()
	if status.CurrentVersion != "0.14.0" {
		t.Errorf("expected current version 0.14.0, got %s", status.CurrentVersion)
	}
}
