//go:build linux

package capabilities

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

func hasBattery() bool {
	matches, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err == nil && len(matches) > 0 {
		return true
	}
	return false
}

func hasGPU() bool {
	matches, err := filepath.Glob("/sys/class/drm/card*")
	if err == nil && len(matches) > 0 {
		return true
	}
	if _, err := os.Stat("/proc/driver/nvidia"); err == nil {
		return true
	}
	return false
}

func hasWebcam() bool {
	matches, err := filepath.Glob("/dev/video*")
	if err == nil && len(matches) > 0 {
		return true
	}
	return false
}

func hasMicrophone() bool {
	if _, err := os.Stat("/dev/snd"); err == nil {
		return true
	}
	return false
}

func hasWifi() bool {
	data, err := os.ReadFile("/proc/net/wireless")
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	return len(lines) > 2
}

func hasUpdatePending() bool {
	_, err := os.Stat("/var/run/reboot-required")
	return err == nil
}

func probeSessionDBus() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func probeSystemDBus() bool {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func detectPlatformCapabilities() PlatformCapabilities {
	sessionBusOk := probeSessionDBus()
	systemBusOk := probeSystemDBus()
	hasDisplay := os.Getenv("DISPLAY") != ""
	hasWayland := os.Getenv("WAYLAND_DISPLAY") != ""

	return PlatformCapabilities{
		OS:            "linux",
		CPU:           true,
		Memory:        true,
		Storage:       true,
		Network:       true,
		Uptime:        true,
		LocalIP:       true,
		Battery:       hasBattery(),
		GPU:           hasGPU(),
		SessionLock:   systemBusOk,
		UserPresence:  sessionBusOk || systemBusOk,
		MediaControl:  sessionBusOk,
		RemoteLock:    systemBusOk,
		Webcam:        hasWebcam(),
		Microphone:    hasMicrophone(),
		DisplayState:  hasDisplay || hasWayland,
		AudioOutput:   sessionBusOk || hasMicrophone(),
		Wifi:          hasWifi(),
		Notifications: sessionBusOk,
		SystemTheme:   sessionBusOk,
		Fullscreen:    hasDisplay,
		ActiveWindow:  hasDisplay,
		UpdatePending: hasUpdatePending(),
	}
}
