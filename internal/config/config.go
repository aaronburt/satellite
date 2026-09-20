package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const Version = "0.15.0"

type MQTTConfig struct {
	Broker      string `json:"broker"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	ClientID    string `json:"client_id"`
	TopicPrefix string `json:"topic_prefix"`
	InsecureTLS bool   `json:"insecure_tls"`
}

type ExposeConfig struct {
	CPU           bool `json:"cpu"`
	Memory        bool `json:"memory"`
	Storage       bool `json:"storage"`
	Network       bool `json:"network"`
	Uptime        bool `json:"uptime"`
	UserPresence  bool `json:"user_presence"`
	SessionLock   bool `json:"session_lock"`
	Microphone    bool `json:"microphone"`
	Fullscreen    bool `json:"fullscreen"`
	WindowsTheme  bool `json:"windows_theme"`
	LocalIP       bool `json:"local_ip"`
	Battery       bool `json:"battery"`
	ActiveWindow  bool `json:"active_window"`
	WindowTitle   bool `json:"window_title"`
	UpdatePending bool `json:"update_pending"`
	MediaControl  bool `json:"media_control"`
	RemoteLock    bool `json:"remote_lock"`
	Webcam        bool `json:"webcam"`
	AudioOutput   bool `json:"audio_output"`
	Wifi          bool `json:"wifi"`
	DisplayState  bool `json:"display_state"`
	Notifications bool `json:"notifications"`
	GPU           bool `json:"gpu"`
}

type WebhookConfig struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`
	MinLevel string `json:"min_level,omitempty"`
	Secret   string `json:"secret,omitempty"`
}

type Config struct {
	NodeID      string        `json:"node_id"`
	MQTT        MQTTConfig    `json:"mqtt"`
	Expose      ExposeConfig  `json:"expose"`
	IntervalSec int           `json:"interval_sec"`
	Port        int           `json:"port,omitempty"`
	WebUIPort   int           `json:"webui_port"`
	BindAddress string        `json:"bind_address,omitempty"`
	JSONEnabled  bool          `json:"json_enabled"`
	APIKey       string        `json:"api_key"`
	Webhook      WebhookConfig `json:"webhook"`
	CheckUpdates bool          `json:"check_updates"`
	UpdateRepo   string        `json:"update_repo,omitempty"`
}

func (c Config) GetPort() int {
	if c.Port > 0 {
		return c.Port
	}
	if c.WebUIPort > 0 {
		return c.WebUIPort
	}
	return 0
}

var (
	cfgLock sync.RWMutex
	current Config
)

func DefaultExpose() ExposeConfig {
	return ExposeConfig{
		CPU:           true,
		Memory:        true,
		Storage:       true,
		Network:       true,
		Uptime:        true,
		UserPresence:  true,
		SessionLock:   true,
		Microphone:    true,
		Fullscreen:    true,
		WindowsTheme:  true,
		LocalIP:       true,
		Battery:       true,
		ActiveWindow:  true,
		WindowTitle:   true,
		UpdatePending: true,
		MediaControl:  true,
		RemoteLock:    true,
		Webcam:        true,
		AudioOutput:   true,
		Wifi:          true,
		DisplayState:  true,
		Notifications: true,
		GPU:           true,
	}
}

func ConfigDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	return filepath.Join(appData, "satellite")
}

func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

func DefaultConfig() Config {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "satellite-node"
	}
	hostname = strings.ToLower(strings.ReplaceAll(hostname, " ", "-"))

	return Config{
		NodeID: hostname,
		MQTT: MQTTConfig{
			Broker:      "",
			Username:    "",
			Password:    "",
			ClientID:    "satellite-" + hostname,
			TopicPrefix: "satellite",
		},
		Expose:      DefaultExpose(),
		IntervalSec: 5,
		Port:        0,
		WebUIPort:   0,
		BindAddress: "127.0.0.1",
		JSONEnabled: false,
		APIKey:      "",
		Webhook: WebhookConfig{
			Enabled:  false,
			URL:      "",
			MinLevel: "info",
			Secret:   "",
		},
		CheckUpdates: true,
		UpdateRepo:   "aaronburt/satellite",
	}
}

func GenerateAPIKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Load() (Config, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	filePath := ConfigFilePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			current = DefaultConfig()
			return current, nil
		}
		return Config{}, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.BindAddress == "" {
		cfg.BindAddress = "127.0.0.1"
	}
	if cfg.Port > 0 && cfg.WebUIPort == 0 {
		cfg.WebUIPort = cfg.Port
	} else if cfg.WebUIPort > 0 && cfg.Port == 0 {
		cfg.Port = cfg.WebUIPort
	}
	if cfg.Webhook.MinLevel == "" {
		cfg.Webhook.MinLevel = "info"
	}

	current = cfg
	return current, nil
}

func Save(cfg Config) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	filePath := ConfigFilePath()
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return os.WriteFile(filePath, data, 0644)
	}

	current = cfg
	return nil
}

func Get() Config {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return current
}
