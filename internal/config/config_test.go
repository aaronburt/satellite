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
	if !cfg.Expose.CPU {
		t.Errorf("expected CPU to be enabled by default")
	}
	if !cfg.Expose.Memory {
		t.Errorf("expected Memory to be enabled by default")
	}
	if !cfg.Expose.Network {
		t.Errorf("expected Network to be enabled by default")
	}
	if cfg.Expose.Webcam {
		t.Errorf("expected Webcam to be disabled by default")
	}
	if cfg.Expose.AudioOutput {
		t.Errorf("expected AudioOutput to be disabled by default")
	}
	if cfg.Expose.RemoteLock {
		t.Errorf("expected RemoteLock to be disabled by default")
	}
	if cfg.Expose.Notifications {
		t.Errorf("expected Notifications to be disabled by default")
	}
	if cfg.JSONEnabled {
		t.Errorf("expected JSONEnabled to be disabled by default")
	}
	if cfg.CheckUpdates {
		t.Errorf("expected CheckUpdates to be disabled by default")
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

	savedFile := filepath.Join(ConfigDir(), "config.json")
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
	if loaded.Expose.CPU != true {
		t.Errorf("expected CPU to be true, got %v", loaded.Expose.CPU)
	}

	currentCfg := Get()
	if currentCfg.NodeID != "test-node" {
		t.Errorf("expected Get() to return updated config")
	}
}

func TestGetPort(t *testing.T) {
	c1 := Config{Port: 8080}
	if c1.GetPort() != 8080 {
		t.Errorf("expected 8080, got %d", c1.GetPort())
	}

	c2 := Config{WebUIPort: 9090}
	if c2.GetPort() != 9090 {
		t.Errorf("expected 9090, got %d", c2.GetPort())
	}

	c3 := Config{}
	if c3.GetPort() != 0 {
		t.Errorf("expected 0, got %d", c3.GetPort())
	}
}

func TestGenerateAPIKey(t *testing.T) {
	k1 := GenerateAPIKey()
	k2 := GenerateAPIKey()
	if len(k1) != 32 {
		t.Errorf("expected 32 char hex key, got %d", len(k1))
	}
	if k1 == k2 {
		t.Errorf("expected random keys, got duplicates")
	}
}

func TestLoadCorruptConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", origAppData)
	os.Setenv("APPDATA", tmpDir)

	filePath := ConfigFilePath()
	_ = os.MkdirAll(filepath.Dir(filePath), 0755)
	_ = os.WriteFile(filePath, []byte("{invalid json"), 0644)

	_, err := Load()
	if err == nil {
		t.Errorf("expected error when loading corrupt json")
	}
}

func TestLoadNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	origAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", origAppData)
	os.Setenv("APPDATA", tmpDir)

	partial := Config{
		NodeID:    "norm-node",
		Port:      8000,
		WebUIPort: 0,
	}
	_ = Save(partial)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if loaded.WebUIPort != 8000 {
		t.Errorf("expected WebUIPort normalized to 8000, got %d", loaded.WebUIPort)
	}
	if loaded.BindAddress != "127.0.0.1" {
		t.Errorf("expected BindAddress normalized to 127.0.0.1, got %s", loaded.BindAddress)
	}
	if loaded.Webhook.MinLevel != "info" {
		t.Errorf("expected Webhook.MinLevel normalized to info, got %s", loaded.Webhook.MinLevel)
	}

	partial2 := Config{
		NodeID:      "norm-node-2",
		WebUIPort:   9000,
		Port:        0,
		BindAddress: "0.0.0.0",
		Webhook:     WebhookConfig{MinLevel: "warn"},
	}
	_ = Save(partial2)
	loaded2, err := Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if loaded2.Port != 9000 {
		t.Errorf("expected Port normalized to 9000, got %d", loaded2.Port)
	}
	if loaded2.BindAddress != "0.0.0.0" {
		t.Errorf("expected BindAddress preserved as 0.0.0.0")
	}
	if loaded2.Webhook.MinLevel != "warn" {
		t.Errorf("expected Webhook.MinLevel preserved as warn")
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", origAppData)
	os.Setenv("APPDATA", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for non-existent config: %v", err)
	}
	if cfg.NodeID == "" {
		t.Errorf("expected default config returned")
	}
}
