//go:build !windows

package screenthumb

import "image"

func Capture(maxSide int) (image.Image, error) {
	return nil, ErrUnsupported
}

func CaptureDisplay(display Display, maxSide int) (image.Image, error) {
	return nil, ErrUnsupported
}

func platformDisplays() ([]Display, error) {
	return []Display{}, nil
}
