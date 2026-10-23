// Package testutil holds helpers shared by tests in several packages.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// IsolateConfigDir points os.UserConfigDir at dir on every platform, so tests
// that save configuration never touch the real user's settings. Windows reads
// APPDATA, Linux reads XDG_CONFIG_HOME, and macOS derives it from HOME.
func IsolateConfigDir(t testing.TB, dir string) {
	t.Helper()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", filepath.Join(dir, "home"))
}

// RunIsolated is for TestMain: it runs the package's tests with the config
// directory pointed at a throwaway folder, so no test (including ones that
// forget to call IsolateConfigDir) can overwrite the real configuration.
func RunIsolated(m interface{ Run() int }) int {
	dir, err := os.MkdirTemp("", "satellite-test-config-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for _, key := range []string{"APPDATA", "XDG_CONFIG_HOME"} {
		os.Setenv(key, dir)
	}
	os.Setenv("HOME", filepath.Join(dir, "home"))
	return m.Run()
}
