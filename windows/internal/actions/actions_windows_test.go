//go:build windows

package actions

import (
	"reflect"
	"testing"
)

type keyEvent struct {
	vk    byte
	flags uintptr
}

func TestMediaAndVolumeActionsSendKeyDownThenUp(t *testing.T) {
	orig := keybdEvent
	t.Cleanup(func() { keybdEvent = orig })

	cases := map[string]struct {
		fn func() error
		vk byte
	}{
		"play_pause": {TogglePlayPause, VK_MEDIA_PLAY_PAUSE},
		"next":       {NextTrack, VK_MEDIA_NEXT_TRACK},
		"prev":       {PreviousTrack, VK_MEDIA_PREV_TRACK},
		"stop":       {Stop, VK_MEDIA_STOP},
		"mute":       {ToggleMute, VK_VOLUME_MUTE},
		"up":         {VolumeUp, VK_VOLUME_UP},
		"down":       {VolumeDown, VK_VOLUME_DOWN},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []keyEvent
			keybdEvent = func(vk byte, flags uintptr) { got = append(got, keyEvent{vk, flags}) }
			if err := tc.fn(); err != nil {
				t.Fatal(err)
			}
			want := []keyEvent{{tc.vk, 0}, {tc.vk, KEYEVENTF_KEYUP}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("events = %v, want %v", got, want)
			}
		})
	}
}

// Guards against someone "tidying" the constants: these are the documented
// Win32 virtual-key codes, and a wrong one presses a different key.
func TestVirtualKeyCodesMatchWin32(t *testing.T) {
	want := map[string]byte{
		"VK_VOLUME_MUTE": 0xAD, "VK_VOLUME_DOWN": 0xAE, "VK_VOLUME_UP": 0xAF,
		"VK_MEDIA_NEXT_TRACK": 0xB0, "VK_MEDIA_PREV_TRACK": 0xB1,
		"VK_MEDIA_STOP": 0xB2, "VK_MEDIA_PLAY_PAUSE": 0xB3,
	}
	got := map[string]byte{
		"VK_VOLUME_MUTE": VK_VOLUME_MUTE, "VK_VOLUME_DOWN": VK_VOLUME_DOWN, "VK_VOLUME_UP": VK_VOLUME_UP,
		"VK_MEDIA_NEXT_TRACK": VK_MEDIA_NEXT_TRACK, "VK_MEDIA_PREV_TRACK": VK_MEDIA_PREV_TRACK,
		"VK_MEDIA_STOP": VK_MEDIA_STOP, "VK_MEDIA_PLAY_PAUSE": VK_MEDIA_PLAY_PAUSE,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("virtual key codes = %v, want %v", got, want)
	}
}
