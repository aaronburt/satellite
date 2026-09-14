package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.NodeID == "" {
		t.Fatalf("expected non-empty NodeID")
	}
	if !cfg.Expose.Webcam {
		t.Errorf("expected Webcam to be enabled by default")
	}
	if !cfg.Expose.AudioOutput {
		t.Errorf("expected AudioOutput to be enabled by default")
	}
	if !cfg.Expose.Wifi {
		t.Errorf("expected Wifi to be enabled by default")
	}
	if !cfg.Expose.DisplayState {
		t.Errorf("expected DisplayState to be enabled by default")
	}
	if cfg.IntervalSec != 5 {
		t.Errorf("expected default IntervalSec to be 5, got %d", cfg.IntervalSec)
	}
}

func TestConfigSerialization(t *testing.T) {
	cfg := DefaultConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	var parsed Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if parsed.NodeID != cfg.NodeID {
		t.Errorf("expected NodeID %q, got %q", cfg.NodeID, parsed.NodeID)
	}
	if parsed.Expose.Webcam != cfg.Expose.Webcam {
		t.Errorf("expected Webcam %v, got %v", cfg.Expose.Webcam, parsed.Expose.Webcam)
	}
	if parsed.Expose.AudioOutput != cfg.Expose.AudioOutput {
		t.Errorf("expected AudioOutput %v, got %v", cfg.Expose.AudioOutput, parsed.Expose.AudioOutput)
	}
	if parsed.Expose.Wifi != cfg.Expose.Wifi {
		t.Errorf("expected Wifi %v, got %v", cfg.Expose.Wifi, parsed.Expose.Wifi)
	}
	if parsed.Expose.DisplayState != cfg.Expose.DisplayState {
		t.Errorf("expected DisplayState %v, got %v", cfg.Expose.DisplayState, parsed.Expose.DisplayState)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", origAppData)
	os.Setenv("APPDATA", tmpDir)

	cfg := DefaultConfig()
	cfg.NodeID = "test-node"
	cfg.Expose.Webcam = false

	if err := Save(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	savedFile := filepath.Join(tmpDir, "satellite", "config.json")
	if _, err := os.Stat(savedFile); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.NodeID != "test-node" {
		t.Errorf("expected NodeID 'test-node', got %q", loaded.NodeID)
	}
	if loaded.Expose.Webcam != false {
		t.Errorf("expected Webcam to be false, got %v", loaded.Expose.Webcam)
	}
	if loaded.Expose.DisplayState != true {
		t.Errorf("expected DisplayState to be true, got %v", loaded.Expose.DisplayState)
	}
}
