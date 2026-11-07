//go:build !windows

package screenthumb

func PlatformHiddenRects(apps []string) ([]Rect, int, int, error) {
	return nil, 0, 0, ErrUnsupported
}
