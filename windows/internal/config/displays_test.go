package config

import (
	"os"
	"slices"
	"testing"

	"satellite/internal/testutil"
)

func TestDisplaySelectionLoadsAndSurvivesExportImport(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	cfg := DefaultConfig()
	cfg.ScreenThumbnailDisplays = []int{3, 1, 3}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(loaded.ScreenThumbnailDisplays, []int{1, 3}) {
		t.Fatalf("loaded displays = %v", loaded.ScreenThumbnailDisplays)
	}
	data, err := ExportConfig(loaded, "")
	if err != nil {
		t.Fatal(err)
	}
	imported, err := ImportConfig(data, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(imported.Config.ScreenThumbnailDisplays, []int{1, 3}) {
		t.Fatalf("imported displays = %v", imported.Config.ScreenThumbnailDisplays)
	}
}

func TestInvalidDisplaySelectionDoesNotFallBackToPrimary(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigFilePath(), []byte(`{"expose":{"screen_thumbnail":true},"screen_thumbnail_displays":[999]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("invalid selection silently fell back to primary")
	}
	cfg := DefaultConfig()
	cfg.ScreenThumbnailDisplays = []int{-1}
	data, err := ExportConfig(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportConfig(data, ImportOptions{}); err == nil {
		t.Fatal("import accepted an invalid selection")
	}
}
