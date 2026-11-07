//go:build !windows

package screenthumb

func platformNewGPU() (gpuCapturer, error) {
	return nil, ErrUnsupported
}

func platformNewDisplayGPU(display Display) (gpuCapturer, error) {
	return nil, ErrUnsupported
}
