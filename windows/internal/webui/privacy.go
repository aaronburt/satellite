package webui

import (
	"net/url"
	"reflect"
	"strings"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/privacy"
)

type privacyPermission struct {
	Key       string `json:"key"`
	Enabled   bool   `json:"enabled"`
	Supported bool   `json:"supported"`
	Paused    bool   `json:"paused"`
}

func privacyOverview(cfg config.Config, caps capabilities.PlatformCapabilities, paused, jsonEnabled bool) map[string]interface{} {
	effective := cfg.Expose
	if paused {
		effective = effective.WithoutSensitive()
	}
	saved := reflect.ValueOf(cfg.Expose)
	active := reflect.ValueOf(effective)
	permissions := []privacyPermission{}
	for i := 0; i < saved.NumField(); i++ {
		if saved.Field(i).Kind() != reflect.Bool {
			continue
		}
		key := strings.Split(saved.Type().Field(i).Tag.Get("json"), ",")[0]
		feature := key
		if key == "window_title" {
			feature = "active_window"
		} else if key == "nvidia_nvml" {
			feature = "gpu"
		}
		permissions = append(permissions, privacyPermission{
			Key: key, Enabled: saved.Field(i).Bool(),
			Supported: key == "device_summary" || caps.IsSupported(feature),
			Paused:    saved.Field(i).Bool() && !active.Field(i).Bool(),
		})
	}
	permissions = append(permissions, privacyPermission{Key: "media_commands", Enabled: cfg.Expose.MediaCommandsEnabled(), Supported: caps.MediaControl})
	media := map[string]bool{}
	for _, action := range []string{"media_play_pause", "media_stop", "media_next", "media_prev", "volume_mute", "volume_up", "volume_down"} {
		media[action] = caps.MediaControl && cfg.Expose.MediaActionAllowed(action)
	}
	return map[string]interface{}{
		"permissions":    permissions,
		"media_commands": media,
		"outputs": []map[string]interface{}{
			{"name": "MQTT", "enabled": strings.TrimSpace(cfg.MQTT.Broker) != "", "host": privacyHost(cfg.MQTT.Broker), "data": "Telemetry state, discovery metadata and availability. Enabled screen thumbnails are sent separately as images; RGB can also use separate topics."},
			{"name": "Telemetry webhook", "enabled": cfg.TelemetryWebhook.Enabled && strings.TrimSpace(cfg.TelemetryWebhook.URL) != "", "host": privacyHost(cfg.TelemetryWebhook.URL), "data": "Telemetry in a state envelope with device ID, device name, platform, app version and timestamp."},
			{"name": "JSON API", "enabled": jsonEnabled, "host": "This device", "data": "The snapshot below, returned on request to authenticated /json clients."},
			{"name": "Log webhook", "enabled": cfg.Webhook.Enabled && strings.TrimSpace(cfg.Webhook.URL) != "", "host": privacyHost(cfg.Webhook.URL), "data": "Log events at the configured severity, separately from telemetry. These events are not shown in the snapshot below."},
		},
	}
}

func privacyHost(raw string) string {
	if !strings.Contains(raw, "://") {
		raw = "//" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "Not configured"
	}
	return u.Hostname()
}

func (s *Server) PausePrivacy(duration time.Duration) {
	privacy.Default.Pause(duration)
	s.collector.ClearSensitive()
	s.mu.Lock()
	store := s.thumbs
	s.mu.Unlock()
	if store != nil {
		store.KeepDisplays(nil)
	}
}
