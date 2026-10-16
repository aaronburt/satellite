//go:build windows

package capabilities

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

func probeBattery() bool {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemPowerStatus := kernel32.NewProc("GetSystemPowerStatus")
	if procGetSystemPowerStatus.Find() != nil {
		return false
	}

	var sps systemPowerStatus
	r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&sps)))
	if r == 0 {
		return false
	}

	const batteryFlagNoSystemBattery = 128
	return sps.BatteryFlag != batteryFlagNoSystemBattery
}

func detectPlatformCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		OS:            "windows",
		CPU:           true,
		Memory:        true,
		Storage:       true,
		Network:       true,
		Uptime:        true,
		LocalIP:       true,
		Battery:       probeBattery(),
		GPU:           true,
		SessionLock:   true,
		UserPresence:  true,
		MediaControl:  true,
		RemoteLock:    true,
		Webcam:        true,
		Microphone:    true,
		DisplayState:  true,
		AudioOutput:   true,
		Wifi:          true,
		Notifications: true,
		SystemTheme:   true,
		Fullscreen:    true,
		ActiveWindow:  true,
		UpdatePending: true,
	}
}
