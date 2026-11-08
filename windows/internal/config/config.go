package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"satellite/internal/screenthumb"
)

// Version is the app version, MAJOR.MINOR. Release builds set it to the
// released version with -ldflags "-X satellite/internal/config.Version=...".
// The value here is used by local builds and is the floor for the next
// release: raise it to jump to a new version (see CONTRIBUTING.md).
var Version = "0.28"

type MQTTConfig struct {
	Broker      string `json:"broker"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	ClientID    string `json:"client_id"`
	TopicPrefix string `json:"topic_prefix"`
	InsecureTLS bool   `json:"insecure_tls"`
}

type MediaPermissions struct {
	PlayPause  bool `json:"play_pause"`
	Stop       bool `json:"stop"`
	Next       bool `json:"next"`
	Previous   bool `json:"previous"`
	Mute       bool `json:"mute"`
	VolumeUp   bool `json:"volume_up"`
	VolumeDown bool `json:"volume_down"`
}

type ExposeConfig struct {
	DeviceSummary    bool              `json:"device_summary"`
	MediaCommands    *bool             `json:"media_commands,omitempty"`
	MediaPermissions *MediaPermissions `json:"media_permissions,omitempty"`
	CPU              bool              `json:"cpu"`
	Memory           bool              `json:"memory"`
	Storage          bool              `json:"storage"`
	Network          bool              `json:"network"`
	Uptime           bool              `json:"uptime"`
	UserPresence     bool              `json:"user_presence"`
	SessionLock      bool              `json:"session_lock"`
	Microphone       bool              `json:"microphone"`
	Fullscreen       bool              `json:"fullscreen"`
	SystemTheme      bool              `json:"system_theme"`
	LocalIP          bool              `json:"local_ip"`
	Battery          bool              `json:"battery"`
	ActiveWindow     bool              `json:"active_window"`
	WindowTitle      bool              `json:"window_title"`
	UpdatePending    bool              `json:"update_pending"`
	MediaControl     bool              `json:"media_control"`
	RemoteLock       bool              `json:"remote_lock"`
	Webcam           bool              `json:"webcam"`
	AudioOutput      bool              `json:"audio_output"`
	ICUERGB          bool              `json:"icue_rgb"`
	Wifi             bool              `json:"wifi"`
	DisplayState     bool              `json:"display_state"`
	Notifications    bool              `json:"notifications"`
	GPU              bool              `json:"gpu"`
	NvidiaNVML       bool              `json:"nvidia_nvml"`
	ScreenThumbnail  bool              `json:"screen_thumbnail"`
}

type WebhookConfig struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`
	MinLevel string `json:"min_level,omitempty"`
	Secret   string `json:"secret,omitempty"`
}

// TelemetryWebhookConfig sends each telemetry snapshot as JSON to any HTTP
// endpoint. It runs alongside MQTT and does not replace it.
type TelemetryWebhookConfig struct {
	Enabled        bool   `json:"enabled"`
	URL            string `json:"url"`
	BearerToken    string `json:"bearer_token,omitempty"`
	MinIntervalSec int    `json:"min_interval_sec,omitempty"`
}

type Config struct {
	SensorThresholds SensorThresholds `json:"sensor_thresholds"`
	LockCooldownSec  *int             `json:"lock_cooldown_sec,omitempty"`
	NodeID           string           `json:"node_id"`
	MQTT             MQTTConfig       `json:"mqtt"`
	Expose           ExposeConfig     `json:"expose"`
	IntervalSec      int              `json:"interval_sec"`
	// ScreenThumbnailSize is the longest edge in pixels of the screen thumbnail.
	ScreenThumbnailSize     int   `json:"screen_thumbnail_size,omitempty"`
	ScreenThumbnailDisplays []int `json:"screen_thumbnail_displays,omitempty"`
	// ScreenThumbnailHiddenApps lists program names (such as chrome.exe) whose
	// windows are blacked out of the screen thumbnail.
	ScreenThumbnailHiddenApps []string      `json:"screen_thumbnail_hidden_apps"`
	HeartbeatSec              int           `json:"heartbeat_sec,omitempty"`
	Port                      int           `json:"port,omitempty"`
	WebUIPort                 int           `json:"webui_port"`
	BindAddress               string        `json:"bind_address,omitempty"`
	JSONEnabled               bool          `json:"json_enabled"`
	APIKey                    string        `json:"api_key"`
	Webhook                   WebhookConfig `json:"webhook"`
	CheckUpdates              bool          `json:"check_updates"`
	AutoUpdate                bool          `json:"auto_update"`
	UpdateRepo                string        `json:"update_repo,omitempty"`

	TelemetryWebhook TelemetryWebhookConfig `json:"telemetry_webhook"`
}

func (c Config) LockCooldownSeconds() int {
	if c.LockCooldownSec == nil {
		return 60
	}
	return *c.LockCooldownSec
}

func (c Config) ValidateLockCooldown() error {
	if seconds := c.LockCooldownSeconds(); seconds < 0 || seconds > 2147483647 {
		return fmt.Errorf("lock cooldown must be a whole number from 0 to 2147483647 seconds")
	}
	return nil
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

	listenersLock  sync.Mutex
	listeners      []listener
	nextListenerID int
)

func DefaultExpose() ExposeConfig {
	return ExposeConfig{
		CPU:             true,
		Memory:          true,
		Storage:         false,
		Network:         true,
		Uptime:          false,
		UserPresence:    false,
		SessionLock:     false,
		Microphone:      false,
		Fullscreen:      false,
		SystemTheme:     false,
		LocalIP:         false,
		Battery:         false,
		ActiveWindow:    false,
		WindowTitle:     false,
		UpdatePending:   false,
		MediaControl:    false,
		RemoteLock:      false,
		Webcam:          false,
		AudioOutput:     false,
		Wifi:            false,
		DisplayState:    false,
		Notifications:   false,
		GPU:             false,
		NvidiaNVML:      false,
		ScreenThumbnail: false,
	}
}

func ConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "satellite")
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
		Expose:                    DefaultExpose(),
		IntervalSec:               5,
		ScreenThumbnailSize:       screenthumb.DefaultSide,
		ScreenThumbnailHiddenApps: defaultHiddenApps(),
		HeartbeatSec:              60,
		Port:                      0,
		WebUIPort:                 0,
		BindAddress:               "127.0.0.1",
		JSONEnabled:               false,
		APIKey:                    "",
		Webhook: WebhookConfig{
			Enabled:  false,
			URL:      "",
			MinLevel: "info",
			Secret:   "",
		},
		CheckUpdates: false,
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
	if err := cfg.ValidateLockCooldown(); err != nil {
		return Config{}, err
	}
	if err := cfg.SensorThresholds.Validate(); err != nil {
		return Config{}, err
	}
	if err := screenthumb.ValidateDisplays(cfg.ScreenThumbnailDisplays); err != nil {
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
	cfg.ScreenThumbnailSize = screenthumb.ClampSide(cfg.ScreenThumbnailSize)
	cfg.ScreenThumbnailDisplays = screenthumb.NormalizeDisplays(cfg.ScreenThumbnailDisplays)
	cfg.ScreenThumbnailHiddenApps = screenthumb.NormalizeApps(cfg.ScreenThumbnailHiddenApps)

	current = cfg
	return current, nil
}

// OnChange registers fn to run after every successful Save, with the saved
// config. Listeners run synchronously on the saving goroutine, after the
// config lock is released, so they must be quick and must not call Save.
// The returned func unregisters fn.
func OnChange(fn func(Config)) (remove func()) {
	listenersLock.Lock()
	defer listenersLock.Unlock()
	nextListenerID++
	id := nextListenerID
	listeners = append(listeners, listener{id, fn})
	return func() {
		listenersLock.Lock()
		defer listenersLock.Unlock()
		for i, l := range listeners {
			if l.id == id {
				listeners = append(listeners[:i:i], listeners[i+1:]...)
				return
			}
		}
	}
}

type listener struct {
	id int
	fn func(Config)
}

func Save(cfg Config) error {
	if err := cfg.SensorThresholds.Validate(); err != nil {
		return err
	}
	if err := cfg.ValidateLockCooldown(); err != nil {
		return err
	}
	if err := save(cfg); err != nil {
		return err
	}
	listenersLock.Lock()
	ls := append([]listener(nil), listeners...)
	listenersLock.Unlock()
	for _, l := range ls {
		l.fn(cfg)
	}
	return nil
}

func save(cfg Config) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	filePath := ConfigFilePath()
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return os.WriteFile(filePath, data, 0600)
	}

	current = cfg
	return nil
}

func Get() Config {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return current
}

func (e ExposeConfig) MediaActionAllowed(action string) bool {
	if !e.MediaCommandsEnabled() {
		return false
	}
	p := e.MediaPermissions
	switch action {
	case "media_play_pause", "media_play", "media_pause":
		return p == nil || p.PlayPause
	case "media_stop":
		return p == nil || p.Stop
	case "media_next":
		return p == nil || p.Next
	case "media_prev":
		return p == nil || p.Previous
	case "volume_mute":
		return p == nil || p.Mute
	case "volume_up":
		return p == nil || p.VolumeUp
	case "volume_down":
		return p == nil || p.VolumeDown
	}
	return false
}

func (e ExposeConfig) MediaCommandsEnabled() bool {
	if e.MediaCommands != nil {
		return *e.MediaCommands
	}
	return e.MediaControl
}
