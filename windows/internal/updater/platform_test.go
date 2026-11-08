package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDesktopReleaseSelectionAcrossPlatforms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []GitHubRelease{
			{TagName: "android-v9.99", Assets: []ReleaseAsset{{Name: "satellite.exe"}}},
			{TagName: "desktop-v9.99", Prerelease: true, Assets: []ReleaseAsset{{Name: "satellite.exe"}}},
			{TagName: "desktop-v8.99", Draft: true, Assets: []ReleaseAsset{{Name: "satellite.exe"}}},
			{TagName: "desktop-v7.99", Assets: []ReleaseAsset{{Name: "satellite.apk"}}},
			{TagName: "v0.47", Assets: []ReleaseAsset{{Name: "satellite.exe"}}},
			{TagName: "desktop-v0.49", Assets: []ReleaseAsset{{Name: "satellite.exe"}}},
			{TagName: "desktop-v0.48", Assets: []ReleaseAsset{{Name: "satellite.exe"}}},
		}
		json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()
	checker := NewChecker("o/r", "0.47", nil)
	checker.SetAPIBaseURL(server.URL)
	status, err := checker.Check(context.Background())
	if err != nil || !status.Available || status.LatestVersion != "desktop-v0.49" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestDesktopReleaseLookupPaginatesWithoutFollowingExternalLinks(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/repos/o/r/releases" || r.URL.Query().Get("per_page") != "100" {
			t.Errorf("unexpected URL %s", r.URL)
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<https://untrusted.invalid/next>; rel="next"`)
			fmt.Fprint(w, `[{"tag_name":"android-v0.99","assets":[{"name":"satellite.apk"}]}]`)
		} else {
			fmt.Fprint(w, `[{"tag_name":"v0.47","assets":[{"name":"satellite.exe"}]}]`)
		}
	}))
	defer server.Close()
	checker := NewChecker("o/r", "0.46", nil)
	checker.SetAPIBaseURL(server.URL)
	status, err := checker.Check(context.Background())
	if err != nil || status.LatestVersion != "v0.47" || calls != 2 {
		t.Fatalf("status=%+v calls=%d err=%v", status, calls, err)
	}
}

func TestPlatformVersionsCompareWithInstalledNumericVersion(t *testing.T) {
	if !IsNewer("0.47", "desktop-v0.48") || IsNewer("0.48", "desktop-v0.48") || !IsNewer("desktop-v0.48", "desktop-v1.0") {
		t.Fatal("desktop prefix broke numeric version comparison")
	}
}
