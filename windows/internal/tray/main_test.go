//go:build windows

package tray

import (
	"os"
	"testing"

	"satellite/internal/testutil"
	"satellite/internal/toast"
)

func TestMain(m *testing.M) {
	openBrowser = func(string) error { return nil }
	showToast = func(toast.Notification) error { return nil }
	os.Exit(testutil.RunIsolated(m))
}
