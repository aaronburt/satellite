package config

import (
	"os"
	"testing"

	"satellite/internal/testutil"
)

func TestScreenThumbnailIsOffByDefault(t *testing.T) {
	if DefaultConfig().Expose.ScreenThumbnail {
		t.Fatal("screen thumbnail must be opt-in")
	}
}

func TestScreenThumbnailStaysOffForOlderConfigFiles(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		t.Fatal(err)
	}
	old := `{"node_id":"old","expose":{"cpu":true,"webcam":true,"window_title":true}}`
	if err := os.WriteFile(ConfigFilePath(), []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Expose.ScreenThumbnail {
		t.Fatal("an older config file must not switch screen thumbnails on")
	}
}

func TestScreenThumbnailSettingRoundTrips(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	cfg := DefaultConfig()
	cfg.Expose.ScreenThumbnail = true
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Expose.ScreenThumbnail {
		t.Fatal("enabled setting was lost on save/load")
	}
}
