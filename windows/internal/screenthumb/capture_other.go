//go:build !windows

package screenthumb

import "image"

func Capture(maxSide int) (image.Image, error) {
	return nil, ErrUnsupported
}
