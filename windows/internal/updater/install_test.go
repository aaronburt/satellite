package updater

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBackgroundDownloadRequiresOptIn(t *testing.T) {
	c := NewChecker("owner/repo", "1.0.0", nil)
	c.client.Transport = updateTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v2.0.0"}`)), Header: make(http.Header)}, nil
	})
	if _, err := c.checkBackground(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CheckAndInstall(context.Background()); err == nil {
		t.Fatal("manual check must report a missing installer")
	}
	if runtime.GOOS != "windows" {
		return
	}
	c.SetAutoUpdate(true)
	if _, err := c.checkBackground(context.Background()); err == nil {
		t.Fatal("enabled updates must attempt installation")
	}
}

func TestRejectUnverifiedInstaller(t *testing.T) {
	for _, asset := range []ReleaseAsset{
		{Name: "setup.exe", URL: "http://github.com/owner/repo/releases/download/v2/setup.exe"},
		{Name: "setup.exe", URL: "https://example.com/owner/repo/releases/download/v2/setup.exe"},
		{Name: "setup.exe", URL: "https://github.com/owner/repo/releases/download/v2/setup.exe", Size: 10},
	} {
		if err := downloadAndLaunch(context.Background(), asset, "owner/repo", "v2"); err == nil {
			t.Fatal("unverified installer accepted")
		}
	}
}
