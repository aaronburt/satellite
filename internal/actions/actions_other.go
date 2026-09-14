//go:build !windows

package actions

import (
	"errors"
)

var errActionNotSupported = errors.New("action not supported on this platform")

func LockWorkstation() error {
	return errActionNotSupported
}

func TogglePlayPause() error {
	return errActionNotSupported
}

func NextTrack() error {
	return errActionNotSupported
}

func PreviousTrack() error {
	return errActionNotSupported
}

func Stop() error {
	return errActionNotSupported
}

func ToggleMute() error {
	return errActionNotSupported
}

func VolumeUp() error {
	return errActionNotSupported
}

func VolumeDown() error {
	return errActionNotSupported
}
