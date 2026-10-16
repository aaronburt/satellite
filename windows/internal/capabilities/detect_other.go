//go:build !windows && !linux

package capabilities

import "runtime"

func detectPlatformCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		OS:            runtime.GOOS,
		CPU:           true,
		Memory:        true,
		Storage:       true,
		Network:       true,
		Uptime:        true,
		LocalIP:       true,
		Battery:       false,
		GPU:           false,
		SessionLock:   false,
		UserPresence:  false,
		MediaControl:  false,
		RemoteLock:    false,
		Webcam:        false,
		Microphone:    false,
		DisplayState:  false,
		AudioOutput:   false,
		Wifi:          false,
		Notifications: false,
		SystemTheme:   false,
		Fullscreen:    false,
		ActiveWindow:  false,
		UpdatePending: false,
	}
}
