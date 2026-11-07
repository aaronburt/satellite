package config

import "testing"

func TestPrivacyMaskKeepsHardwareAndControlChoices(t *testing.T) {
	e := ExposeConfig{
		CPU: true, Memory: true, Network: true, GPU: true, NvidiaNVML: true,
		LocalIP: true, Wifi: true, Fullscreen: true, AudioOutput: true, DisplayState: true,
		RemoteLock: true, Notifications: true,
		UserPresence: true, SessionLock: true, MediaControl: true, ActiveWindow: true,
		WindowTitle: true, Microphone: true, Webcam: true, ScreenThumbnail: true,
	}
	want := ExposeConfig{
		CPU: true, Memory: true, Network: true, GPU: true, NvidiaNVML: true,
		LocalIP: true, Wifi: true, Fullscreen: true, AudioOutput: true, DisplayState: true,
		RemoteLock: true, Notifications: true,
	}
	if got := e.WithoutSensitive(); got != want {
		t.Fatalf("masked choices = %+v, want %+v", got, want)
	}
	if !e.ScreenThumbnail || !e.MediaControl || !e.UserPresence {
		t.Fatal("mask changed the original choices")
	}
}
