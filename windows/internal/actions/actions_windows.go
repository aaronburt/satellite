//go:build windows

package actions

import (
	"fmt"
	"syscall"
)

const (
	VK_VOLUME_MUTE      = 0xAD
	VK_VOLUME_DOWN      = 0xAE
	VK_VOLUME_UP        = 0xAF
	VK_MEDIA_NEXT_TRACK = 0xB0
	VK_MEDIA_PREV_TRACK = 0xB1
	VK_MEDIA_STOP       = 0xB2
	VK_MEDIA_PLAY_PAUSE = 0xB3
	KEYEVENTF_KEYUP     = 0x0002
)

var (
	modUser32           = syscall.NewLazyDLL("user32.dll")
	procLockWorkStation = modUser32.NewProc("LockWorkStation")
	procKeybdEvent      = modUser32.NewProc("keybd_event")
)

func sendVirtualKey(vk byte) error {
	procKeybdEvent.Call(uintptr(vk), 0, 0, 0)
	procKeybdEvent.Call(uintptr(vk), 0, KEYEVENTF_KEYUP, 0)
	return nil
}

func LockWorkstation() error {
	ret, _, err := procLockWorkStation.Call()
	if ret == 0 {
		return fmt.Errorf("LockWorkStation failed: %w", err)
	}
	return nil
}

func TogglePlayPause() error {
	return sendVirtualKey(VK_MEDIA_PLAY_PAUSE)
}

func NextTrack() error {
	return sendVirtualKey(VK_MEDIA_NEXT_TRACK)
}

func PreviousTrack() error {
	return sendVirtualKey(VK_MEDIA_PREV_TRACK)
}

func Stop() error {
	return sendVirtualKey(VK_MEDIA_STOP)
}

func ToggleMute() error {
	return sendVirtualKey(VK_VOLUME_MUTE)
}

func VolumeUp() error {
	return sendVirtualKey(VK_VOLUME_UP)
}

func VolumeDown() error {
	return sendVirtualKey(VK_VOLUME_DOWN)
}
