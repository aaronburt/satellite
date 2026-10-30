//go:build windows

package toast

import (
	"errors"
	"testing"
)

func stubWindows(t *testing.T, notifyErr error) (*[]string, *[]string) {
	t.Helper()
	origN, origO := notify, openURL
	t.Cleanup(func() { notify, openURL = origN, origO })
	var shown, opened []string
	notify = func(title, message string, _ any) error {
		shown = append(shown, title+"|"+message)
		return notifyErr
	}
	openURL = func(u string) error {
		opened = append(opened, u)
		return nil
	}
	t.Setenv("APPDATA", t.TempDir())
	return &shown, &opened
}

func TestShowDefaultsTitleAndOpensSafeURL(t *testing.T) {
	shown, opened := stubWindows(t, nil)
	if err := Show(Notification{Title: "  ", Message: "Update ready", URL: " https://github.com/aaronburt/satellite/releases "}); err != nil {
		t.Fatal(err)
	}
	if len(*shown) != 1 || (*shown)[0] != "Satellite|Update ready" {
		t.Fatalf("shown = %v", *shown)
	}
	if len(*opened) != 1 || (*opened)[0] != "https://github.com/aaronburt/satellite/releases" {
		t.Fatalf("opened = %v", *opened)
	}
}

// A notification arriving over MQTT must never be able to run a program.
func TestShowNeverOpensUnsafeURLs(t *testing.T) {
	_, opened := stubWindows(t, nil)
	for _, u := range []string{"https://x.com&calc.exe", "file:///C:/Windows/System32/calc.exe", "calc.exe", "ms-settings:"} {
		_ = Show(Notification{Message: "m", URL: u})
	}
	if len(*opened) != 0 {
		t.Fatalf("opened unsafe URLs: %v", *opened)
	}
}

func TestShowReportsNotifyFailureAndDoesNotOpenURL(t *testing.T) {
	_, opened := stubWindows(t, errors.New("toast service unavailable"))
	if err := Show(Notification{Message: "m", URL: "https://example.com"}); err == nil {
		t.Fatal("expected error")
	}
	if len(*opened) != 0 {
		t.Fatal("URL opened although the notification failed")
	}
}

func TestOpenBrowserRejectsUnsafeURLs(t *testing.T) {
	for _, u := range []string{"", "calc.exe", "file:///C:/Windows/System32/calc.exe", "http://127.0.0.1/?a=1&calc"} {
		if err := OpenBrowser(u); err == nil {
			t.Fatalf("expected OpenBrowser(%q) to fail", u)
		}
	}
}
