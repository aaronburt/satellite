package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"

	"satellite/internal/config"
	"satellite/internal/telemetry"
)

type DeviceInfo struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer"`
	SWVersion    string   `json:"sw_version,omitempty"`
}

type EntityDiscoveryPayload struct {
	Component              string     `json:"-"`
	Name                   string     `json:"name"`
	StateTopic             string     `json:"state_topic,omitempty"`
	ValueTemplate          string     `json:"value_template,omitempty"`
	UniqueID               string     `json:"unique_id"`
	Device                 DeviceInfo `json:"device"`
	AvailabilityTopic      string     `json:"availability_topic"`
	UnitOfMeasurement      string     `json:"unit_of_measurement,omitempty"`
	DeviceClass            string     `json:"device_class,omitempty"`
	StateClass             string     `json:"state_class,omitempty"`
	PayloadOn              string     `json:"payload_on,omitempty"`
	PayloadOff             string     `json:"payload_off,omitempty"`
	CommandTopic           string     `json:"command_topic,omitempty"`
	PayloadPress           string     `json:"payload_press,omitempty"`
	PayloadPlay            string     `json:"payload_play,omitempty"`
	PayloadPause           string     `json:"payload_pause,omitempty"`
	PayloadStop            string     `json:"payload_stop,omitempty"`
	PayloadNext            string     `json:"payload_next,omitempty"`
	PayloadPrevious        string     `json:"payload_previous,omitempty"`
	JSONAttributesTopic    string     `json:"json_attributes_topic,omitempty"`
	JSONAttributesTemplate string     `json:"json_attributes_template,omitempty"`
	Icon                   string     `json:"icon,omitempty"`
}

type DiscoveryItem struct {
	Key       string
	Topic     string
	Payload   []byte
	ShouldRun bool
}

func GetAllDiscoveryItems(cfg config.Config, hasBattery bool) []DiscoveryItem {
	nodeID := cfg.NodeID
	topicPrefix := cfg.MQTT.TopicPrefix
	if topicPrefix == "" {
		topicPrefix = "satellite"
	}

	device := DeviceInfo{
		Identifiers:  []string{"satellite_" + nodeID},
		Name:         "Satellite " + nodeID,
		Model:        "Satellite Agent",
		Manufacturer: "Antigravity",
		SWVersion:    config.Version,
	}

	stateTopic := fmt.Sprintf("%s/%s/state", topicPrefix, nodeID)
	availTopic := fmt.Sprintf("%s/%s/status", topicPrefix, nodeID)
	commandTopic := fmt.Sprintf("%s/%s/command", topicPrefix, nodeID)

	definitions := []struct {
		Key       string
		ShouldRun bool
		Payload   EntityDiscoveryPayload
	}{
		{
			Key:       "cpu",
			ShouldRun: cfg.Expose.CPU,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "CPU Usage",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.cpu_percent }}",
				UniqueID:          nodeID + "_cpu",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "%",
				StateClass:        "measurement",
				Icon:              "mdi:cpu-64-bit",
			},
		},
		{
			Key:       "memory_percent",
			ShouldRun: cfg.Expose.Memory,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Memory Usage",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.memory_percent }}",
				UniqueID:          nodeID + "_memory_percent",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "%",
				StateClass:        "measurement",
				Icon:              "mdi:memory",
			},
		},
		{
			Key:       "memory_used",
			ShouldRun: cfg.Expose.Memory,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Memory Used",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.memory_used_gb }}",
				UniqueID:          nodeID + "_memory_used",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "GB",
				DeviceClass:       "data_size",
				StateClass:        "measurement",
				Icon:              "mdi:memory",
			},
		},
		{
			Key:       "network_rx",
			ShouldRun: cfg.Expose.Network,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Network Download",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.network_rx_kbs }}",
				UniqueID:          nodeID + "_network_rx",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "kB/s",
				DeviceClass:       "data_rate",
				StateClass:        "measurement",
				Icon:              "mdi:download-network",
			},
		},
		{
			Key:       "network_tx",
			ShouldRun: cfg.Expose.Network,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Network Upload",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.network_tx_kbs }}",
				UniqueID:          nodeID + "_network_tx",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "kB/s",
				DeviceClass:       "data_rate",
				StateClass:        "measurement",
				Icon:              "mdi:upload-network",
			},
		},
		{
			Key:       "uptime",
			ShouldRun: cfg.Expose.Uptime,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Uptime",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ (value_json.uptime_seconds / 3600) | round(1) }}",
				UniqueID:          nodeID + "_uptime",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "h",
				DeviceClass:       "duration",
				StateClass:        "measurement",
				Icon:              "mdi:clock-outline",
			},
		},
		{
			Key:       "user_idle",
			ShouldRun: cfg.Expose.UserPresence,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "User Idle Time",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.user_idle_seconds }}",
				UniqueID:          nodeID + "_user_idle",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "s",
				DeviceClass:       "duration",
				StateClass:        "measurement",
				Icon:              "mdi:account-clock",
			},
		},
		{
			Key:       "user_active",
			ShouldRun: cfg.Expose.UserPresence,
			Payload: EntityDiscoveryPayload{
				Component:         "binary_sensor",
				Name:              "User Active",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ 'ON' if value_json.user_active else 'OFF' }}",
				UniqueID:          nodeID + "_user_active",
				Device:            device,
				AvailabilityTopic: availTopic,
				DeviceClass:       "occupancy",
				PayloadOn:         "ON",
				PayloadOff:        "OFF",
				Icon:              "mdi:account-check",
			},
		},
		{
			Key:       "session_locked",
			ShouldRun: cfg.Expose.SessionLock,
			Payload: EntityDiscoveryPayload{
				Component:         "binary_sensor",
				Name:              "Workstation Locked",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ 'ON' if not value_json.session_locked else 'OFF' }}",
				UniqueID:          nodeID + "_session_locked",
				Device:            device,
				AvailabilityTopic: availTopic,
				DeviceClass:       "lock",
				PayloadOn:         "ON",
				PayloadOff:        "OFF",
				Icon:              "mdi:lock",
			},
		},
		{
			Key:       "microphone",
			ShouldRun: cfg.Expose.Microphone,
			Payload: EntityDiscoveryPayload{
				Component:         "binary_sensor",
				Name:              "Microphone Active",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ 'ON' if value_json.microphone_in_use else 'OFF' }}",
				UniqueID:          nodeID + "_mic_active",
				Device:            device,
				AvailabilityTopic: availTopic,
				DeviceClass:       "sound",
				PayloadOn:         "ON",
				PayloadOff:        "OFF",
				Icon:              "mdi:microphone",
			},
		},
		{
			Key:       "fullscreen",
			ShouldRun: cfg.Expose.Fullscreen,
			Payload: EntityDiscoveryPayload{
				Component:         "binary_sensor",
				Name:              "Fullscreen Active",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ 'ON' if value_json.fullscreen_active else 'OFF' }}",
				UniqueID:          nodeID + "_fullscreen",
				Device:            device,
				AvailabilityTopic: availTopic,
				DeviceClass:       "running",
				PayloadOn:         "ON",
				PayloadOff:        "OFF",
				Icon:              "mdi:fullscreen",
			},
		},
		{
			Key:       "windows_theme",
			ShouldRun: cfg.Expose.WindowsTheme,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Windows Theme",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.windows_theme }}",
				UniqueID:          nodeID + "_theme",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:theme-light-dark",
			},
		},
		{
			Key:       "local_ip",
			ShouldRun: cfg.Expose.LocalIP,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Local IP",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.local_ip }}",
				UniqueID:          nodeID + "_local_ip",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:ip-network",
			},
		},
		{
			Key:       "battery",
			ShouldRun: cfg.Expose.Battery && hasBattery,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Battery Level",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.battery_percent }}",
				UniqueID:          nodeID + "_battery",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "%",
				DeviceClass:       "battery",
				StateClass:        "measurement",
			},
		},
		{
			Key:       "power_plugged",
			ShouldRun: cfg.Expose.Battery && hasBattery,
			Payload: EntityDiscoveryPayload{
				Component:         "binary_sensor",
				Name:              "Power Connected",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ 'ON' if value_json.power_plugged else 'OFF' }}",
				UniqueID:          nodeID + "_power_plugged",
				Device:            device,
				AvailabilityTopic: availTopic,
				DeviceClass:       "plug",
				PayloadOn:         "ON",
				PayloadOff:        "OFF",
			},
		},
		{
			Key:       "active_window",
			ShouldRun: cfg.Expose.ActiveWindow,
			Payload: EntityDiscoveryPayload{
				Component:              "sensor",
				Name:                   "Active Window",
				StateTopic:             stateTopic,
				ValueTemplate:          "{{ value_json.active_process }}",
				UniqueID:               nodeID + "_active_window",
				Device:                 device,
				AvailabilityTopic:      availTopic,
				Icon:                   "mdi:application",
			},
		},
		{
			Key:       "window_title",
			ShouldRun: cfg.Expose.WindowTitle,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Window Title",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.active_window_title }}",
				UniqueID:          nodeID + "_window_title",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:card-text-outline",
			},
		},
		{
			Key:       "update_pending",
			ShouldRun: cfg.Expose.UpdatePending,
			Payload: EntityDiscoveryPayload{
				Component:         "binary_sensor",
				Name:              "Reboot Pending",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ 'ON' if value_json.update_pending else 'OFF' }}",
				UniqueID:          nodeID + "_update_pending",
				Device:            device,
				AvailabilityTopic: availTopic,
				DeviceClass:       "problem",
				PayloadOn:         "ON",
				PayloadOff:        "OFF",
				Icon:              "mdi:restart-alert",
			},
		},
		{
			Key:       "media_player",
			ShouldRun: cfg.Expose.MediaControl,
			Payload: EntityDiscoveryPayload{
				Component:              "media_player",
				Name:                   "Media Player",
				StateTopic:             stateTopic,
				ValueTemplate:          "{{ value_json.media.status if value_json.media is defined else 'idle' }}",
				CommandTopic:           commandTopic,
				PayloadPlay:            "media_play",
				PayloadPause:           "media_pause",
				PayloadStop:            "media_stop",
				PayloadNext:            "media_next",
				PayloadPrevious:        "media_prev",
				JSONAttributesTopic:    stateTopic,
				JSONAttributesTemplate: "{{ value_json.media | default({}) | to_json }}",
				UniqueID:               nodeID + "_media_player",
				Device:                 device,
				AvailabilityTopic:      availTopic,
				Icon:                   "mdi:music",
			},
		},
		{
			Key:       "media_play_pause",
			ShouldRun: cfg.Expose.MediaControl,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Media Play / Pause",
				CommandTopic:      commandTopic,
				PayloadPress:      "media_play_pause",
				UniqueID:          nodeID + "_media_play_pause",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:play-pause",
			},
		},
		{
			Key:       "media_next",
			ShouldRun: cfg.Expose.MediaControl,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Media Next Track",
				CommandTopic:      commandTopic,
				PayloadPress:      "media_next",
				UniqueID:          nodeID + "_media_next",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:skip-next",
			},
		},
		{
			Key:       "media_prev",
			ShouldRun: cfg.Expose.MediaControl,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Media Previous Track",
				CommandTopic:      commandTopic,
				PayloadPress:      "media_prev",
				UniqueID:          nodeID + "_media_previous",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:skip-previous",
			},
		},
		{
			Key:       "volume_mute",
			ShouldRun: cfg.Expose.MediaControl,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Media Mute / Unmute",
				CommandTopic:      commandTopic,
				PayloadPress:      "volume_mute",
				UniqueID:          nodeID + "_volume_mute",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:volume-mute",
			},
		},
		{
			Key:       "volume_up",
			ShouldRun: cfg.Expose.MediaControl,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Volume Up",
				CommandTopic:      commandTopic,
				PayloadPress:      "volume_up",
				UniqueID:          nodeID + "_volume_up",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:volume-high",
			},
		},
		{
			Key:       "volume_down",
			ShouldRun: cfg.Expose.MediaControl,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Volume Down",
				CommandTopic:      commandTopic,
				PayloadPress:      "volume_down",
				UniqueID:          nodeID + "_volume_down",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:volume-low",
			},
		},
		{
			Key:       "remote_lock",
			ShouldRun: cfg.Expose.RemoteLock,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Lock Workstation",
				CommandTopic:      commandTopic,
				PayloadPress:      "LOCK",
				UniqueID:          nodeID + "_lock",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:lock",
			},
		},
		{
			Key:       "webcam",
			ShouldRun: cfg.Expose.Webcam,
			Payload: EntityDiscoveryPayload{
				Component:              "binary_sensor",
				Name:                   "Webcam Active",
				StateTopic:             stateTopic,
				ValueTemplate:          "{{ 'ON' if value_json.webcam_in_use else 'OFF' }}",
				UniqueID:               nodeID + "_webcam_active",
				Device:                 device,
				AvailabilityTopic:      availTopic,
				DeviceClass:            "running",
				PayloadOn:              "ON",
				PayloadOff:             "OFF",
				JSONAttributesTopic:    stateTopic,
				JSONAttributesTemplate: "{{ {'active_app': value_json.webcam_active_app} | to_json if value_json.webcam_active_app is defined else '{}' }}",
				Icon:                   "mdi:webcam",
			},
		},
		{
			Key:       "audio_output",
			ShouldRun: cfg.Expose.AudioOutput,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Audio Output Device",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.audio_output_name }}",
				UniqueID:          nodeID + "_audio_output",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:speaker",
			},
		},
		{
			Key:       "wifi_ssid",
			ShouldRun: cfg.Expose.Wifi,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Network SSID",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.wifi_ssid }}",
				UniqueID:          nodeID + "_wifi_ssid",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:wifi",
			},
		},
		{
			Key:       "wifi_signal",
			ShouldRun: cfg.Expose.Wifi,
			Payload: EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              "Wi-Fi Signal Strength",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ value_json.wifi_signal_percent }}",
				UniqueID:          nodeID + "_wifi_signal",
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "%",
				DeviceClass:       "signal_strength",
				StateClass:        "measurement",
				Icon:              "mdi:wifi-strength-4",
			},
		},
		{
			Key:       "display_state",
			ShouldRun: cfg.Expose.DisplayState,
			Payload: EntityDiscoveryPayload{
				Component:         "binary_sensor",
				Name:              "Display Power",
				StateTopic:        stateTopic,
				ValueTemplate:     "{{ 'ON' if value_json.display_powered else 'OFF' }}",
				UniqueID:          nodeID + "_display_power",
				Device:            device,
				AvailabilityTopic: availTopic,
				DeviceClass:       "power",
				PayloadOn:         "ON",
				PayloadOff:        "OFF",
				Icon:              "mdi:monitor",
			},
		},
		{
			Key:       "display_sleep",
			ShouldRun: cfg.Expose.DisplayState,
			Payload: EntityDiscoveryPayload{
				Component:         "button",
				Name:              "Sleep Displays",
				CommandTopic:      commandTopic,
				PayloadPress:      "display_sleep",
				UniqueID:          nodeID + "_sleep_displays",
				Device:            device,
				AvailabilityTopic: availTopic,
				Icon:              "mdi:monitor-off",
			},
		},
	}

	var items []DiscoveryItem
	for _, def := range definitions {
		topic := fmt.Sprintf("homeassistant/%s/%s/%s/config", def.Payload.Component, nodeID, def.Payload.UniqueID)
		var b []byte
		if def.ShouldRun {
			b, _ = json.Marshal(def.Payload)
		}
		items = append(items, DiscoveryItem{
			Key:       def.Key,
			Topic:     topic,
			Payload:   b,
			ShouldRun: def.ShouldRun,
		})
	}

	if cfg.Expose.Storage {
		drives := telemetry.GetAllDrives()
		for _, d := range drives {
			cleanMount := strings.ToLower(strings.Trim(d.Mount, ":"))
			dTopic := fmt.Sprintf("homeassistant/sensor/%s/%s_drive_%s_pct/config", nodeID, nodeID, cleanMount)
			payload := EntityDiscoveryPayload{
				Component:         "sensor",
				Name:              fmt.Sprintf("Disk %s Usage", d.Mount),
				StateTopic:        stateTopic,
				ValueTemplate:     fmt.Sprintf("{{ (value_json.drives | selectattr('mount', 'equalto', '%s') | map(attribute='used_percent') | first) if value_json.drives is defined else None }}", d.Mount),
				UniqueID:          fmt.Sprintf("%s_drive_%s_pct", nodeID, cleanMount),
				Device:            device,
				AvailabilityTopic: availTopic,
				UnitOfMeasurement: "%",
				StateClass:        "measurement",
				Icon:              "mdi:harddisk",
			}
			b, _ := json.Marshal(payload)
			items = append(items, DiscoveryItem{
				Key:       "drive_" + cleanMount,
				Topic:     dTopic,
				Payload:   b,
				ShouldRun: true,
			})
		}
	}

	return items
}
