package telemetry

type DiskInfo struct {
	Mount       string  `json:"mount"`
	UsedGB      float64 `json:"used_gb"`
	TotalGB     float64 `json:"total_gb"`
	UsedPercent float64 `json:"used_percent"`
}

type MediaInfo struct {
	Status string `json:"status"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	AppID  string `json:"app_id"`
}

type GPUInfo struct {
	Index            int      `json:"index"`
	Name             string   `json:"name"`
	Vendor           string   `json:"vendor"`
	CoreUsagePercent *float64 `json:"core_usage_percent,omitempty"`
	MemoryUsedMB     *float64 `json:"memory_used_mb,omitempty"`
	MemoryTotalMB    *float64 `json:"memory_total_mb,omitempty"`
	MemoryPercent    *float64 `json:"memory_percent,omitempty"`
	TemperatureC     *float64 `json:"temperature_c,omitempty"`
	PowerWatts       *float64 `json:"power_watts,omitempty"`
	FanSpeedPercent  *float64 `json:"fan_speed_percent,omitempty"`
}

type Snapshot struct {
	CPUPercent        *float64   `json:"cpu_percent,omitempty"`
	MemoryUsedGB      *float64   `json:"memory_used_gb,omitempty"`
	MemoryTotalGB     *float64   `json:"memory_total_gb,omitempty"`
	MemoryPercent     *float64   `json:"memory_percent,omitempty"`
	Drives            []DiskInfo `json:"drives,omitempty"`
	NetworkRxKBs      *float64   `json:"network_rx_kbs,omitempty"`
	NetworkTxKBs      *float64   `json:"network_tx_kbs,omitempty"`
	UptimeSeconds     *uint64    `json:"uptime_seconds,omitempty"`
	UserIdleSeconds   *uint64    `json:"user_idle_seconds,omitempty"`
	UserActive        *bool      `json:"user_active,omitempty"`
	SessionLocked     *bool      `json:"session_locked,omitempty"`
	MicrophoneInUse   *bool      `json:"microphone_in_use,omitempty"`
	FullscreenActive  *bool      `json:"fullscreen_active,omitempty"`
	SystemTheme       string     `json:"system_theme,omitempty"`
	LocalIP           string     `json:"local_ip,omitempty"`
	HasBattery        bool       `json:"has_battery"`
	BatteryPercent    *int       `json:"battery_percent,omitempty"`
	PowerPlugged      *bool      `json:"power_plugged,omitempty"`
	ActiveProcess     string     `json:"active_process,omitempty"`
	ActiveWindowTitle string     `json:"active_window_title,omitempty"`
	UpdatePending     *bool      `json:"update_pending,omitempty"`
	Media             *MediaInfo `json:"media,omitempty"`
	WebcamInUse       *bool      `json:"webcam_in_use,omitempty"`
	WebcamActiveApp   string     `json:"webcam_active_app,omitempty"`
	AudioOutputName   string     `json:"audio_output_name,omitempty"`
	WifiSSID          string     `json:"wifi_ssid,omitempty"`
	WifiSignalPercent *int       `json:"wifi_signal_percent,omitempty"`
	DisplayPowered    *bool      `json:"display_powered,omitempty"`
	GPUs              []GPUInfo  `json:"gpus,omitempty"`
	Timestamp         int64      `json:"timestamp"`
}
