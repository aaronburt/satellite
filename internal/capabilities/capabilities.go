package capabilities

type PlatformCapabilities struct {
	OS            string `json:"os"`
	CPU           bool   `json:"cpu"`
	Memory        bool   `json:"memory"`
	Storage       bool   `json:"storage"`
	Network       bool   `json:"network"`
	Uptime        bool   `json:"uptime"`
	LocalIP       bool   `json:"local_ip"`
	Battery       bool   `json:"battery"`
	GPU           bool   `json:"gpu"`
	SessionLock   bool   `json:"session_lock"`
	UserPresence  bool   `json:"user_presence"`
	MediaControl  bool   `json:"media_control"`
	RemoteLock    bool   `json:"remote_lock"`
	Webcam        bool   `json:"webcam"`
	Microphone    bool   `json:"microphone"`
	DisplayState  bool   `json:"display_state"`
	AudioOutput   bool   `json:"audio_output"`
	Wifi          bool   `json:"wifi"`
	Notifications bool   `json:"notifications"`
	SystemTheme   bool   `json:"system_theme"`
	Fullscreen    bool   `json:"fullscreen"`
	ActiveWindow  bool   `json:"active_window"`
	UpdatePending bool   `json:"update_pending"`
}

func Detect() PlatformCapabilities {
	return detectPlatformCapabilities()
}

func (p PlatformCapabilities) IsSupported(feature string) bool {
	switch feature {
	case "cpu":
		return p.CPU
	case "memory":
		return p.Memory
	case "storage":
		return p.Storage
	case "network":
		return p.Network
	case "uptime":
		return p.Uptime
	case "local_ip":
		return p.LocalIP
	case "battery":
		return p.Battery
	case "gpu":
		return p.GPU
	case "session_lock":
		return p.SessionLock
	case "user_presence":
		return p.UserPresence
	case "media_control":
		return p.MediaControl
	case "remote_lock":
		return p.RemoteLock
	case "webcam":
		return p.Webcam
	case "microphone":
		return p.Microphone
	case "display_state":
		return p.DisplayState
	case "audio_output":
		return p.AudioOutput
	case "wifi":
		return p.Wifi
	case "notifications":
		return p.Notifications
	case "system_theme":
		return p.SystemTheme
	case "fullscreen":
		return p.Fullscreen
	case "active_window":
		return p.ActiveWindow
	case "update_pending":
		return p.UpdatePending
	default:
		return false
	}
}
