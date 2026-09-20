package telemetry

import (
	"math"
	"sync"
	"time"

	"satellite/internal/config"
)

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
	WindowsTheme      string     `json:"windows_theme,omitempty"`
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

type Collector struct {
	mu           sync.Mutex
	lastIdle     uint64
	lastKernel   uint64
	lastUser     uint64
	lastRx       uint64
	lastTx       uint64
	lastNetTime  time.Time
	lastSnapshot Snapshot
}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Collect(expose config.ExposeConfig) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := Snapshot{
		Timestamp: time.Now().Unix(),
	}

	if expose.CPU {
		idle, kernel, user, err := GetCPUTimes()
		if err == nil {
			if c.lastKernel > 0 || c.lastUser > 0 {
				idleDelta := idle - c.lastIdle
				kernelDelta := kernel - c.lastKernel
				userDelta := user - c.lastUser
				totalDelta := kernelDelta + userDelta
				if totalDelta > 0 && totalDelta >= idleDelta {
					cpuPct := math.Round((float64(totalDelta-idleDelta)/float64(totalDelta))*1000) / 10
					snap.CPUPercent = &cpuPct
				}
			}
			c.lastIdle = idle
			c.lastKernel = kernel
			c.lastUser = user
		}
	}

	if expose.Memory {
		used, total, pct, err := GetRAMUsage()
		if err == nil {
			snap.MemoryUsedGB = &used
			snap.MemoryTotalGB = &total
			snap.MemoryPercent = &pct
		}
	}

	if expose.Storage {
		snap.Drives = GetAllDrives()
	}

	if expose.Network {
		rx, tx, err := GetNetworkOctets()
		if err == nil {
			now := time.Now()
			if !c.lastNetTime.IsZero() {
				elapsed := now.Sub(c.lastNetTime).Seconds()
				if elapsed > 0 && rx >= c.lastRx && tx >= c.lastTx {
					rxKBs := math.Round((float64(rx-c.lastRx)/elapsed/1024)*10) / 10
					txKBs := math.Round((float64(tx-c.lastTx)/elapsed/1024)*10) / 10
					snap.NetworkRxKBs = &rxKBs
					snap.NetworkTxKBs = &txKBs
				}
			}
			c.lastRx = rx
			c.lastTx = tx
			c.lastNetTime = now
		}
	}

	if expose.Uptime {
		uptime := GetSystemUptimeSeconds()
		snap.UptimeSeconds = &uptime
	}

	if expose.UserPresence {
		idleSec, ok := GetUserIdleSeconds()
		if ok {
			snap.UserIdleSeconds = &idleSec
			active := idleSec < 300
			snap.UserActive = &active
		}
	}

	if expose.SessionLock {
		locked := IsSessionLocked()
		snap.SessionLocked = &locked
	}

	if expose.Microphone {
		micInUse := IsMicrophoneInUse()
		snap.MicrophoneInUse = &micInUse
	}

	if expose.Fullscreen {
		fs := IsFullscreenActive()
		snap.FullscreenActive = &fs
	}

	if expose.WindowsTheme {
		snap.WindowsTheme = GetWindowsTheme()
	}

	if expose.LocalIP {
		snap.LocalIP = GetLocalIPv4()
	}

	hasBattery, batteryPct, acPlugged := GetBatteryAndPower()
	snap.HasBattery = hasBattery
	if expose.Battery && hasBattery {
		snap.BatteryPercent = &batteryPct
		snap.PowerPlugged = &acPlugged
	}

	if expose.ActiveWindow || expose.WindowTitle {
		procName, winTitle := GetActiveWindowAndProcess()
		if expose.ActiveWindow {
			snap.ActiveProcess = procName
		}
		if expose.WindowTitle {
			snap.ActiveWindowTitle = winTitle
		}
	}

	if expose.UpdatePending {
		pending := IsRebootPending()
		snap.UpdatePending = &pending
	}

	if expose.MediaControl {
		media := GetActiveMediaInfo()
		snap.Media = &media
	}

	if expose.Webcam {
		inUse, app := GetWebcamStatus()
		snap.WebcamInUse = &inUse
		snap.WebcamActiveApp = app
	}

	if expose.AudioOutput {
		snap.AudioOutputName = GetActiveAudioOutputDevice()
	}

	if expose.Wifi {
		ssid, sig := GetWifiStatus()
		snap.WifiSSID = ssid
		snap.WifiSignalPercent = sig
	}

	if expose.DisplayState {
		powered := IsDisplayPowered()
		snap.DisplayPowered = &powered
	}

	if expose.GPU {
		snap.GPUs = GetGPUInfo()
	}

	c.lastSnapshot = snap
	return snap
}

func (c *Collector) Last() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastSnapshot
}

func HasSignificantDelta(prev, curr Snapshot) bool {
	if (prev.CPUPercent == nil) != (curr.CPUPercent == nil) {
		return true
	}
	if (prev.CPUPercent != nil && curr.CPUPercent != nil) && math.Abs(*prev.CPUPercent-*curr.CPUPercent) >= 1.0 {
		return true
	}
	if (prev.MemoryPercent == nil) != (curr.MemoryPercent == nil) {
		return true
	}
	if (prev.MemoryPercent != nil && curr.MemoryPercent != nil) && math.Abs(*prev.MemoryPercent-*curr.MemoryPercent) >= 0.5 {
		return true
	}
	if (prev.NetworkRxKBs == nil) != (curr.NetworkRxKBs == nil) {
		return true
	}
	if (prev.NetworkRxKBs != nil && curr.NetworkRxKBs != nil) && math.Abs(*prev.NetworkRxKBs-*curr.NetworkRxKBs) >= 20.0 {
		return true
	}
	if (prev.NetworkTxKBs == nil) != (curr.NetworkTxKBs == nil) {
		return true
	}
	if (prev.NetworkTxKBs != nil && curr.NetworkTxKBs != nil) && math.Abs(*prev.NetworkTxKBs-*curr.NetworkTxKBs) >= 20.0 {
		return true
	}
	if (prev.UserActive == nil) != (curr.UserActive == nil) {
		return true
	}
	if (prev.UserActive != nil && curr.UserActive != nil) && *prev.UserActive != *curr.UserActive {
		return true
	}
	if (prev.SessionLocked == nil) != (curr.SessionLocked == nil) {
		return true
	}
	if (prev.SessionLocked != nil && curr.SessionLocked != nil) && *prev.SessionLocked != *curr.SessionLocked {
		return true
	}
	if (prev.MicrophoneInUse == nil) != (curr.MicrophoneInUse == nil) {
		return true
	}
	if (prev.MicrophoneInUse != nil && curr.MicrophoneInUse != nil) && *prev.MicrophoneInUse != *curr.MicrophoneInUse {
		return true
	}
	if (prev.FullscreenActive == nil) != (curr.FullscreenActive == nil) {
		return true
	}
	if (prev.FullscreenActive != nil && curr.FullscreenActive != nil) && *prev.FullscreenActive != *curr.FullscreenActive {
		return true
	}
	if prev.WindowsTheme != curr.WindowsTheme {
		return true
	}
	if prev.LocalIP != curr.LocalIP {
		return true
	}
	if (prev.PowerPlugged == nil) != (curr.PowerPlugged == nil) {
		return true
	}
	if (prev.PowerPlugged != nil && curr.PowerPlugged != nil) && *prev.PowerPlugged != *curr.PowerPlugged {
		return true
	}
	if prev.ActiveProcess != curr.ActiveProcess {
		return true
	}
	if prev.ActiveWindowTitle != curr.ActiveWindowTitle {
		return true
	}
	if (prev.UpdatePending == nil) != (curr.UpdatePending == nil) {
		return true
	}
	if (prev.UpdatePending != nil && curr.UpdatePending != nil) && *prev.UpdatePending != *curr.UpdatePending {
		return true
	}
	if (prev.WebcamInUse == nil) != (curr.WebcamInUse == nil) {
		return true
	}
	if (prev.WebcamInUse != nil && curr.WebcamInUse != nil) && *prev.WebcamInUse != *curr.WebcamInUse {
		return true
	}
	if prev.WebcamActiveApp != curr.WebcamActiveApp {
		return true
	}
	if prev.AudioOutputName != curr.AudioOutputName {
		return true
	}
	if prev.WifiSSID != curr.WifiSSID {
		return true
	}
	if (prev.DisplayPowered == nil) != (curr.DisplayPowered == nil) {
		return true
	}
	if (prev.DisplayPowered != nil && curr.DisplayPowered != nil) && *prev.DisplayPowered != *curr.DisplayPowered {
		return true
	}
	if (prev.Media == nil) != (curr.Media == nil) {
		return true
	}
	if prev.Media != nil && curr.Media != nil {
		if prev.Media.Status != curr.Media.Status ||
			prev.Media.Title != curr.Media.Title ||
			prev.Media.Artist != curr.Media.Artist ||
			prev.Media.AppID != curr.Media.AppID {
			return true
		}
	}
	if len(prev.GPUs) != len(curr.GPUs) {
		return true
	}
	for i := range curr.GPUs {
		prevG := prev.GPUs[i]
		currG := curr.GPUs[i]
		if (prevG.CoreUsagePercent == nil) != (currG.CoreUsagePercent == nil) {
			return true
		}
		if prevG.CoreUsagePercent != nil && currG.CoreUsagePercent != nil && math.Abs(*prevG.CoreUsagePercent-*currG.CoreUsagePercent) >= 1.0 {
			return true
		}
		if (prevG.MemoryPercent == nil) != (currG.MemoryPercent == nil) {
			return true
		}
		if prevG.MemoryPercent != nil && currG.MemoryPercent != nil && math.Abs(*prevG.MemoryPercent-*currG.MemoryPercent) >= 1.0 {
			return true
		}
		if (prevG.TemperatureC == nil) != (currG.TemperatureC == nil) {
			return true
		}
		if prevG.TemperatureC != nil && currG.TemperatureC != nil && math.Abs(*prevG.TemperatureC-*currG.TemperatureC) >= 1.0 {
			return true
		}
	}
	return false
}
