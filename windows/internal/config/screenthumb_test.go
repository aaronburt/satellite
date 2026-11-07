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

func TestScreenThumbnailSizeDefaultsTo200(t *testing.T) {
	if got := DefaultConfig().ScreenThumbnailSize; got != 200 {
		t.Fatalf("default size = %d, want 200", got)
	}
}

func TestScreenThumbnailSizeLoad(t *testing.T) {
	for name, tc := range map[string]struct {
		json string
		want int
	}{
		"missing":   {`{"node_id":"x"}`, 200},
		"custom":    {`{"node_id":"x","screen_thumbnail_size":320}`, 320},
		"too big":   {`{"node_id":"x","screen_thumbnail_size":4000}`, 400},
		"too small": {`{"node_id":"x","screen_thumbnail_size":5}`, 64},
		"negative":  {`{"node_id":"x","screen_thumbnail_size":-1}`, 200},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.IsolateConfigDir(t, t.TempDir())
			if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ConfigFilePath(), []byte(tc.json), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ScreenThumbnailSize != tc.want {
				t.Fatalf("size = %d, want %d", cfg.ScreenThumbnailSize, tc.want)
			}
		})
	}
}

func TestScreenThumbnailSizeSurvivesExportImport(t *testing.T) {
	orig := DefaultConfig()
	orig.ScreenThumbnailSize = 288
	data, err := ExportConfig(orig, "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := ImportConfig(data, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Config.ScreenThumbnailSize != 288 {
		t.Fatalf("size = %d, want 288", res.Config.ScreenThumbnailSize)
	}
}

func TestHiddenAppsNormalizedOnLoadAndImport(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		t.Fatal(err)
	}
	raw := `{"node_id":"x","screen_thumbnail_hidden_apps":[" Chrome.EXE ","C:\\Apps\\Edge.exe","chrome.exe",""]}`
	if err := os.WriteFile(ConfigFilePath(), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.ScreenThumbnailHiddenApps; len(got) != 2 || got[0] != "chrome.exe" || got[1] != "edge.exe" {
		t.Fatalf("loaded %v", got)
	}

	orig := DefaultConfig()
	orig.ScreenThumbnailHiddenApps = []string{"chrome.exe", "vlc.exe"}
	data, err := ExportConfig(orig, "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := ImportConfig(data, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Config.ScreenThumbnailHiddenApps; len(got) != 2 || got[1] != "vlc.exe" {
		t.Fatalf("imported %v", got)
	}
}
