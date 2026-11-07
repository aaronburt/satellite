//go:build !windows

package screenthumb

func platformNewGPU() (gpuCapturer, error) {
	return nil, ErrUnsupported
}
