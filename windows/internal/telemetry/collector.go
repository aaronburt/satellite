package telemetry

import (
	"math"
	"sync"
	"time"

	"satellite/internal/config"
)

type Collector struct {
	mu           sync.Mutex
	lastIdle     uint64
	lastKernel   uint64
	lastUser     uint64
	lastRx       uint64
	lastTx       uint64
	lastNetTime  time.Time
	lastSnapshot Snapshot
	now          func() time.Time
}

func NewCollector() *Collector {
	return &Collector{now: time.Now}
}

func (c *Collector) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// cpuPercent converts two cumulative CPU time samples into a busy percentage
// (one decimal place). kernel includes idle time, matching GetSystemTimes on
// Windows and the Linux reader. ok is false when no time has elapsed or the
// counters went backwards.
func cpuPercent(prevIdle, prevKernel, prevUser, idle, kernel, user uint64) (float64, bool) {
	if idle < prevIdle || kernel < prevKernel || user < prevUser {
		return 0, false
	}
	idleDelta := idle - prevIdle
	totalDelta := (kernel - prevKernel) + (user - prevUser)
	if totalDelta == 0 || totalDelta < idleDelta {
		return 0, false
	}
	return math.Round((float64(totalDelta-idleDelta)/float64(totalDelta))*1000) / 10, true
}

// kbPerSecond converts two cumulative byte counters into kB/s (one decimal).
func kbPerSecond(prev, curr uint64, elapsed time.Duration) (float64, bool) {
	secs := elapsed.Seconds()
	if secs <= 0 || curr < prev {
		return 0, false
	}
	return math.Round((float64(curr-prev)/secs/1024)*10) / 10, true
}

func (c *Collector) Collect(expose config.ExposeConfig) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	collectedAt := c.clock()
	snap := Snapshot{
		Timestamp: collectedAt.Unix(),
	}

	if expose.CPU {
		idle, kernel, user, err := GetCPUTimes()
		if err == nil {
			if c.lastKernel > 0 || c.lastUser > 0 {
				if cpuPct, ok := cpuPercent(c.lastIdle, c.lastKernel, c.lastUser, idle, kernel, user); ok {
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
			if !c.lastNetTime.IsZero() {
				elapsed := collectedAt.Sub(c.lastNetTime)
				rxKBs, rxOK := kbPerSecond(c.lastRx, rx, elapsed)
				txKBs, txOK := kbPerSecond(c.lastTx, tx, elapsed)
				if rxOK && txOK {
					snap.NetworkRxKBs = &rxKBs
					snap.NetworkTxKBs = &txKBs
				}
			}
			c.lastRx = rx
			c.lastTx = tx
			c.lastNetTime = collectedAt
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

	if expose.SystemTheme {
		snap.SystemTheme = GetSystemTheme()
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
		snap.GPUs = GetGPUInfo(expose.NvidiaNVML)
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
	if prev.SystemTheme != curr.SystemTheme {
		return true
	}
	if prev.LocalIP != curr.LocalIP {
		return true
	}
	if prev.HasBattery != curr.HasBattery {
		return true
	}
	if (prev.BatteryPercent == nil) != (curr.BatteryPercent == nil) {
		return true
	}
	if (prev.BatteryPercent != nil && curr.BatteryPercent != nil) && *prev.BatteryPercent != *curr.BatteryPercent {
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
	if (prev.WifiSignalPercent == nil) != (curr.WifiSignalPercent == nil) {
		return true
	}
	if (prev.WifiSignalPercent != nil && curr.WifiSignalPercent != nil) && math.Abs(float64(*prev.WifiSignalPercent-*curr.WifiSignalPercent)) >= 5.0 {
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
			prev.Media.Album != curr.Media.Album ||
			prev.Media.AppID != curr.Media.AppID {
			return true
		}
	}
	if len(prev.Drives) != len(curr.Drives) {
		return true
	}
	for i := range curr.Drives {
		if prev.Drives[i].Mount != curr.Drives[i].Mount {
			return true
		}
		if math.Abs(prev.Drives[i].UsedPercent-curr.Drives[i].UsedPercent) >= 1.0 {
			return true
		}
	}
	if len(prev.GPUs) != len(curr.GPUs) {
		return true
	}
	for i := range curr.GPUs {
		prevG := prev.GPUs[i]
		currG := curr.GPUs[i]
		if prevG.Index != currG.Index || prevG.Name != currG.Name {
			return true
		}
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
		if (prevG.PowerWatts == nil) != (currG.PowerWatts == nil) {
			return true
		}
		if prevG.PowerWatts != nil && currG.PowerWatts != nil && math.Abs(*prevG.PowerWatts-*currG.PowerWatts) >= 5.0 {
			return true
		}
		if (prevG.FanSpeedPercent == nil) != (currG.FanSpeedPercent == nil) {
			return true
		}
		if prevG.FanSpeedPercent != nil && currG.FanSpeedPercent != nil && math.Abs(*prevG.FanSpeedPercent-*currG.FanSpeedPercent) >= 2.0 {
			return true
		}
	}
	return false
}
