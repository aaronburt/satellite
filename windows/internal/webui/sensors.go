package webui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/telemetry"
)

// Sensor states. The page maps these to colours.
const (
	StateOK      = "ok"      // a normal reading
	StateActive  = "active"  // something good or "on" (playing, user active)
	StateWarn    = "warn"    // worth noticing (microphone in use, session locked)
	StateIdle    = "idle"    // a reading that says nothing is happening
	StateWaiting = "waiting" // enabled, but no reading yet
)

// Sensor groups, in the order the page shows them.
const (
	GroupSystem  = "System"
	GroupNetwork = "Network"
	GroupSession = "Session"
	GroupDevices = "Media & devices"
	GroupScreen  = "Screen"
)

// Gauge levels, for colouring a bar.
const (
	LevelMid  = "mid"
	LevelHigh = "high"
)

// Sensor is one enabled sensor and its latest reading, ready to show.
type Sensor struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Label string `json:"label"`
	// Value is the main reading. Detail is a smaller second line.
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
	State  string `json:"state"`
	// Percent draws a gauge bar. Level colours it.
	Percent *float64 `json:"percent,omitempty"`
	Level   string   `json:"level,omitempty"`
	// Image marks a sensor shown as a picture. FrameTS is when the latest
	// frame was captured (unix seconds); 0 means there is none yet.
	Image   bool  `json:"image,omitempty"`
	FrameTS int64 `json:"frame_ts,omitempty"`
}

// Frame describes the latest screen thumbnail, if any.
type Frame struct {
	Have bool
	At   time.Time
}

// BuildSensors lists every sensor that is switched on in expose and supported
// on this platform, with its reading from snap. A sensor that is on but has no
// reading yet is still listed, as waiting, so the page never silently drops
// one. now is used for ages like "4s ago".
func BuildSensors(expose config.ExposeConfig, caps capabilities.PlatformCapabilities, snap telemetry.Snapshot, frame Frame, now time.Time) []Sensor {
	b := sensorBuilder{expose: expose, caps: caps, snap: snap, haveSnap: snap.Timestamp != 0, out: []Sensor{}}

	if b.on(expose.CPU, "cpu") {
		s := b.base("cpu", GroupSystem, "Processor")
		if snap.CPUPercent != nil {
			b.gauge(&s, *snap.CPUPercent, usageLevel(*snap.CPUPercent))
			s.Value = pct1(*snap.CPUPercent)
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.Memory, "memory") {
		s := b.base("memory", GroupSystem, "Memory")
		if snap.MemoryPercent != nil && snap.MemoryUsedGB != nil && snap.MemoryTotalGB != nil {
			b.gauge(&s, *snap.MemoryPercent, usageLevel(*snap.MemoryPercent))
			s.Value = pct0(*snap.MemoryPercent)
			s.Detail = fmt.Sprintf("%.1f / %.1f GB", *snap.MemoryUsedGB, *snap.MemoryTotalGB)
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.GPU, "gpu") {
		if len(snap.GPUs) == 0 {
			s := b.base("gpu", GroupSystem, "Graphics")
			if b.haveSnap {
				s.Value = "No GPU found"
				s.State = StateIdle
			} else {
				b.waiting(&s)
			}
			b.add(s)
		}
		for i, g := range snap.GPUs {
			label := g.Name
			if label == "" {
				label = fmt.Sprintf("Graphics %d", i+1)
			}
			s := b.base(fmt.Sprintf("gpu%d", g.Index), GroupSystem, label)
			if g.CoreUsagePercent != nil {
				b.gauge(&s, *g.CoreUsagePercent, usageLevel(*g.CoreUsagePercent))
				s.Value = pct1(*g.CoreUsagePercent)
			} else {
				s.Value = "Active"
			}
			s.Detail = gpuDetail(g)
			b.add(s)
		}
	}

	if b.on(expose.Storage, "storage") {
		if len(snap.Drives) == 0 {
			s := b.base("storage", GroupSystem, "Storage")
			if b.haveSnap {
				s.Value = "No drives found"
				s.State = StateIdle
			} else {
				b.waiting(&s)
			}
			b.add(s)
		}
		for _, d := range snap.Drives {
			mount := d.Mount
			if mount == "" {
				mount = "Disk"
			}
			s := b.base("disk:"+mount, GroupSystem, "Disk ("+mount+")")
			b.gauge(&s, d.UsedPercent, usageLevel(d.UsedPercent))
			s.Value = pct0(d.UsedPercent)
			s.Detail = fmt.Sprintf("%.1f / %.1f GB", d.UsedGB, d.TotalGB)
			b.add(s)
		}
	}

	if b.on(expose.Uptime, "uptime") {
		s := b.base("uptime", GroupSystem, "Uptime")
		if snap.UptimeSeconds != nil {
			s.Value = formatDuration(*snap.UptimeSeconds)
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	// Only listed when a battery was found, as in Home Assistant.
	if b.on(expose.Battery, "battery") && (snap.HasBattery || !b.haveSnap) {
		s := b.base("battery", GroupSystem, "Battery")
		if snap.BatteryPercent != nil {
			p := float64(*snap.BatteryPercent)
			b.gauge(&s, p, batteryLevel(p))
			s.Value = pct0(p)
			if snap.PowerPlugged != nil {
				if *snap.PowerPlugged {
					s.Detail = "Plugged in"
					s.State = StateActive
				} else {
					s.Detail = "On battery"
				}
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.UpdatePending, "update_pending") {
		s := b.base("update_pending", GroupSystem, "Windows Update")
		if snap.UpdatePending != nil {
			if *snap.UpdatePending {
				s.Value, s.State = "Restart pending", StateWarn
			} else {
				s.Value, s.State = "Up to date", StateActive
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.Network, "network") {
		s := b.base("network", GroupNetwork, "Network traffic")
		if snap.NetworkRxKBs != nil && snap.NetworkTxKBs != nil {
			s.Value = "Down " + formatRate(*snap.NetworkRxKBs)
			s.Detail = "Up " + formatRate(*snap.NetworkTxKBs)
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.LocalIP, "local_ip") {
		s := b.base("local_ip", GroupNetwork, "IPv4 address")
		if snap.LocalIP != "" {
			s.Value = snap.LocalIP
		} else if b.haveSnap {
			s.Value, s.State = "Not connected", StateIdle
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.Wifi, "wifi") {
		s := b.base("wifi", GroupNetwork, "Wi-Fi")
		switch {
		case snap.WifiSSID != "":
			s.Value = snap.WifiSSID
			if snap.WifiSignalPercent != nil {
				p := float64(*snap.WifiSignalPercent)
				s.Percent = &p
				s.Detail = fmt.Sprintf("Signal %d%%", *snap.WifiSignalPercent)
			}
		case b.haveSnap:
			s.Value, s.State = "Not connected", StateIdle
		default:
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.UserPresence, "user_presence") {
		s := b.base("user_presence", GroupSession, "User")
		if snap.UserActive != nil {
			if *snap.UserActive {
				s.Value, s.State = "Active", StateActive
			} else {
				s.Value, s.State = "Away", StateIdle
			}
			if snap.UserIdleSeconds != nil {
				s.Detail = "Idle for " + formatDuration(*snap.UserIdleSeconds)
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.SessionLock, "session_lock") {
		s := b.base("session_lock", GroupSession, "Session")
		if snap.SessionLocked != nil {
			if *snap.SessionLocked {
				s.Value, s.State = "Locked", StateWarn
			} else {
				s.Value, s.State = "Unlocked", StateActive
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.DisplayState, "display_state") {
		s := b.base("display_state", GroupSession, "Display")
		if snap.DisplayPowered != nil {
			if *snap.DisplayPowered {
				s.Value, s.State = "On", StateActive
			} else {
				s.Value, s.State = "Asleep", StateIdle
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.Fullscreen, "fullscreen") {
		s := b.base("fullscreen", GroupSession, "Fullscreen")
		if snap.FullscreenActive != nil {
			if *snap.FullscreenActive {
				s.Value, s.State = "Fullscreen app", StateActive
			} else {
				s.Value, s.State = "Windowed", StateIdle
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.SystemTheme, "system_theme") {
		s := b.base("system_theme", GroupSession, "Theme")
		if snap.SystemTheme != "" {
			s.Value = titleCase(snap.SystemTheme)
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.ActiveWindow, "active_window") {
		s := b.base("active_window", GroupSession, "Foreground app")
		b.text(&s, snap.ActiveProcess, "None")
		b.add(s)
	}

	// The title comes from the same platform feature as the app name.
	if b.on(expose.WindowTitle, "active_window") {
		s := b.base("window_title", GroupSession, "Window title")
		b.text(&s, snap.ActiveWindowTitle, "None")
		b.add(s)
	}

	if b.on(expose.MediaControl, "media_control") {
		s := b.base("media_control", GroupDevices, "Media player")
		mediaSensor(&s, snap.Media, b.haveSnap)
		b.add(s)
	}

	if b.on(expose.AudioOutput, "audio_output") {
		s := b.base("audio_output", GroupDevices, "Audio output")
		b.text(&s, snap.AudioOutputName, "None")
		b.add(s)
	}

	if b.on(expose.Microphone, "microphone") {
		s := b.base("microphone", GroupDevices, "Microphone")
		if snap.MicrophoneInUse != nil {
			if *snap.MicrophoneInUse {
				s.Value, s.State = "In use", StateWarn
			} else {
				s.Value, s.State = "Not in use", StateIdle
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.Webcam, "webcam") {
		s := b.base("webcam", GroupDevices, "Webcam")
		if snap.WebcamInUse != nil {
			if *snap.WebcamInUse {
				s.Value, s.State = "In use", StateWarn
				s.Detail = snap.WebcamActiveApp
			} else {
				s.Value, s.State = "Not in use", StateIdle
			}
		} else {
			b.waiting(&s)
		}
		b.add(s)
	}

	if b.on(expose.ScreenThumbnail, "screen_thumbnail") {
		s := b.base("screen_thumbnail", GroupScreen, "Screen preview")
		s.Image = true
		if frame.Have {
			s.FrameTS = frame.At.Unix()
			s.Value = "Live"
			s.State = StateActive
			s.Detail = "Updated " + formatAge(now.Sub(frame.At))
		} else {
			s.Value, s.State = "Waiting for the first capture", StateWaiting
		}
		b.add(s)
	}

	return b.out
}

type sensorBuilder struct {
	expose   config.ExposeConfig
	caps     capabilities.PlatformCapabilities
	snap     telemetry.Snapshot
	haveSnap bool
	out      []Sensor
}

// on reports whether a sensor is switched on and supported here.
func (b *sensorBuilder) on(enabled bool, capability string) bool {
	return enabled && b.caps.IsSupported(capability)
}

func (b *sensorBuilder) base(id, group, label string) Sensor {
	return Sensor{ID: id, Group: group, Label: label, State: StateOK}
}

func (b *sensorBuilder) add(s Sensor) { b.out = append(b.out, s) }

// waiting marks a sensor with no reading. Before the first collection that is
// expected; after it, the platform could not provide the value.
func (b *sensorBuilder) waiting(s *Sensor) {
	s.State = StateWaiting
	if b.haveSnap {
		s.Value = "No reading"
	} else {
		s.Value = "Waiting for the first reading"
	}
}

func (b *sensorBuilder) gauge(s *Sensor, percent float64, level string) {
	p := math.Max(0, math.Min(100, percent))
	s.Percent = &p
	s.Level = level
}

// text fills a text sensor. An empty reading after the first collection is a
// real answer (no foreground window), so it shows as none, not as missing.
func (b *sensorBuilder) text(s *Sensor, value, none string) {
	switch {
	case value != "":
		s.Value = value
	case b.haveSnap:
		s.Value, s.State = none, StateIdle
	default:
		b.waiting(s)
	}
}

func mediaSensor(s *Sensor, m *telemetry.MediaInfo, haveSnap bool) {
	if m == nil {
		if haveSnap {
			s.Value, s.State = "Nothing playing", StateIdle
		} else {
			s.Value, s.State = "Waiting for the first reading", StateWaiting
		}
		return
	}
	status := strings.ToLower(m.Status)
	if status != "playing" && status != "paused" {
		s.Value, s.State = "Nothing playing", StateIdle
		return
	}
	title := m.Title
	if title == "" {
		title = "Unknown track"
	}
	s.Value = title
	parts := []string{titleCase(status)}
	if m.Artist != "" {
		parts = append(parts, m.Artist)
	}
	if m.AppID != "" {
		parts = append(parts, m.AppID)
	}
	s.Detail = strings.Join(parts, " · ")
	if status == "playing" {
		s.State = StateActive
	}
}

func gpuDetail(g telemetry.GPUInfo) string {
	var parts []string
	if g.MemoryUsedMB != nil && g.MemoryTotalMB != nil {
		p := fmt.Sprintf("VRAM %.1f / %.1f GB", *g.MemoryUsedMB/1024, *g.MemoryTotalMB/1024)
		if g.MemoryPercent != nil {
			p += fmt.Sprintf(" (%.0f%%)", *g.MemoryPercent)
		}
		parts = append(parts, p)
	}
	if g.TemperatureC != nil {
		parts = append(parts, fmt.Sprintf("%.0f°C", *g.TemperatureC))
	}
	if g.PowerWatts != nil {
		parts = append(parts, fmt.Sprintf("%.0f W", *g.PowerWatts))
	}
	if g.FanSpeedPercent != nil {
		parts = append(parts, fmt.Sprintf("Fan %.0f%%", *g.FanSpeedPercent))
	}
	return strings.Join(parts, " · ")
}

// usageLevel colours a "more is worse" gauge.
func usageLevel(p float64) string {
	switch {
	case p >= 90:
		return LevelHigh
	case p >= 70:
		return LevelMid
	}
	return ""
}

// batteryLevel colours a "less is worse" gauge.
func batteryLevel(p float64) string {
	switch {
	case p <= 15:
		return LevelHigh
	case p <= 30:
		return LevelMid
	}
	return ""
}

func pct0(p float64) string { return fmt.Sprintf("%.0f%%", p) }
func pct1(p float64) string { return fmt.Sprintf("%.1f%%", p) }

// formatRate shows a kB/s reading, switching to MB/s from 1 MB/s up.
func formatRate(kbs float64) string {
	if kbs >= 1024 {
		return fmt.Sprintf("%.1f MB/s", kbs/1024)
	}
	return fmt.Sprintf("%.1f kB/s", kbs)
}

// formatDuration shows the two largest units, e.g. "3d 4h", "5h 12m", "42s".
func formatDuration(secs uint64) string {
	d := secs / 86400
	h := secs % 86400 / 3600
	m := secs % 3600 / 60
	s := secs % 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// formatAge shows how long ago something happened. A clock that is slightly
// behind never produces a negative age.
func formatAge(d time.Duration) string {
	secs := int64(d / time.Second)
	switch {
	case secs < 2:
		return "just now"
	case secs < 120:
		return fmt.Sprintf("%ds ago", secs)
	case secs < 7200:
		return fmt.Sprintf("%dm ago", secs/60)
	}
	return fmt.Sprintf("%dh ago", secs/3600)
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
