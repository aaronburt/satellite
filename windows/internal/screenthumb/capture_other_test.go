//go:build !windows

package screenthumb

import (
	"errors"
	"testing"
)

func TestCaptureIsUnsupportedOffWindows(t *testing.T) {
	if img, err := Capture(MaxSide); img != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Capture = %v, %v; want ErrUnsupported", img, err)
	}
}
