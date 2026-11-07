//go:build !windows

package screenthumb

import "image"

// Capture is only implemented on Windows.
func Capture(maxSide int) (image.Image, error) {
	return nil, ErrUnsupported
}
