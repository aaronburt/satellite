package config

import (
	"os"
	"reflect"
	"testing"

	"satellite/internal/screenthumb"
	"satellite/internal/testutil"
)

func TestDefaultRedactionCoversBrowsersAndMessagingApps(t *testing.T) {
	apps := DefaultConfig().ScreenThumbnailHiddenApps
	if got := screenthumb.NormalizeApps(apps); !reflect.DeepEqual(got, apps) {
		t.Fatal("default programs were truncated or needed normalization")
	}
	for _, name := range []string{"chrome.exe", "msedge.exe", "firefox.exe", "brave.exe", "opera.exe", "vivaldi.exe", "mullvadbrowser.exe", "discord.exe", "whatsapp.exe", "signal.exe", "telegram.exe", "slack.exe", "ms-teams.exe"} {
		if !screenthumb.MatchApp(name, apps) {
			t.Errorf("default redaction misses %s", name)
		}
	}
	apps[0] = "changed.exe"
	if DefaultConfig().ScreenThumbnailHiddenApps[0] == "changed.exe" {
		t.Fatal("configs share a mutable default list")
	}
}

func TestRedactionDefaultsPreserveUserChoices(t *testing.T) {
	for _, test := range []struct {
		name, raw string
		want      []string
	}{
		{"missing", `{"node_id":"old"}`, defaultHiddenApps()},
		{"custom", `{"node_id":"custom","screen_thumbnail_hidden_apps":["private.exe"]}`, []string{"private.exe"}},
		{"cleared", `{"node_id":"cleared","screen_thumbnail_hidden_apps":[]}`, nil},
		{"null", `{"node_id":"cleared","screen_thumbnail_hidden_apps":null}`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			testutil.IsolateConfigDir(t, t.TempDir())
			if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ConfigFilePath(), []byte(test.raw), 0600); err != nil {
				t.Fatal(err)
			}
			for cycle := 0; cycle < 2; cycle++ {
				cfg, err := Load()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cfg.ScreenThumbnailHiddenApps, test.want) {
					t.Fatalf("cycle %d: got %v, want %v", cycle, cfg.ScreenThumbnailHiddenApps, test.want)
				}
				if err := Save(cfg); err != nil {
					t.Fatal(err)
				}
				for _, pass := range []string{"", "test-passphrase"} {
					data, err := ExportConfig(cfg, pass)
					if err != nil {
						t.Fatal(err)
					}
					result, err := ImportConfig(data, ImportOptions{Passphrase: pass})
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(result.Config.ScreenThumbnailHiddenApps, test.want) {
						t.Fatal("export/import changed the redaction choices")
					}
				}
			}
		})
	}
}
