package screenthumb

import (
	"fmt"
	"image"
	"sync"
	"time"
)

type Backend string

const (
	BackendGDI Backend = "gdi"
	BackendGPU Backend = "gpu"
)

const (
	gpuRetryMin = 30 * time.Second
	gpuRetryMax = 10 * time.Minute
)

type gpuCapturer interface {
	Capture(maxSide int) (image.Image, error)
	Close()
}

type Source struct {
	UseGPU     func() bool
	GDI        func(maxSide int) (image.Image, error)
	NewGPU     func() (gpuCapturer, error)
	OnBackend  func(Backend)
	OnFallback func(error)
	Now        func() time.Time

	mu       sync.Mutex
	gpu      gpuCapturer
	retryAt  time.Time
	failures int
	last     Backend
}

func NewSource(useGPU func() bool) *Source {
	return &Source{
		UseGPU: useGPU,
		GDI:    Capture,
		NewGPU: platformNewGPU,
	}
}

func (s *Source) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Source) Backend() Backend {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func (s *Source) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeGPU()
}

func (s *Source) Capture(maxSide int) (image.Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.UseGPU == nil || !s.UseGPU() || s.NewGPU == nil {
		s.closeGPU()
		s.failures = 0
		s.retryAt = time.Time{}
		return s.useGDI(maxSide)
	}

	if s.gpu == nil && !s.now().Before(s.retryAt) {
		g, err := s.openGPU()
		if err != nil {
			s.fail(fmt.Errorf("GPU capture unavailable: %w", err))
		} else {
			s.gpu = g
		}
	}

	if s.gpu != nil {
		img, err := s.captureGPU(maxSide)
		if err == nil {
			s.failures = 0
			s.setBackend(BackendGPU)
			return img, nil
		}
		s.closeGPU()
		s.fail(fmt.Errorf("GPU capture failed: %w", err))
	}
	return s.useGDI(maxSide)
}

func (s *Source) openGPU() (g gpuCapturer, err error) {
	defer func() {
		if r := recover(); r != nil {
			g, err = nil, fmt.Errorf("panic: %v", r)
		}
	}()
	return s.NewGPU()
}

func (s *Source) captureGPU(maxSide int) (img image.Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			img, err = nil, fmt.Errorf("panic: %v", r)
		}
	}()
	return s.gpu.Capture(maxSide)
}

func (s *Source) useGDI(maxSide int) (image.Image, error) {
	img, err := s.GDI(maxSide)
	if err == nil {
		s.setBackend(BackendGDI)
	}
	return img, err
}

func (s *Source) closeGPU() {
	if s.gpu != nil {
		s.gpu.Close()
		s.gpu = nil
	}
}

func (s *Source) fail(err error) {
	s.failures++
	delay := gpuRetryMin
	for i := 1; i < s.failures && delay < gpuRetryMax; i++ {
		delay *= 2
	}
	delay = min(delay, gpuRetryMax)
	s.retryAt = s.now().Add(delay)
	if s.OnFallback != nil {
		s.OnFallback(err)
	}
}

func (s *Source) setBackend(b Backend) {
	if s.last == b {
		return
	}
	s.last = b
	if s.OnBackend != nil {
		s.OnBackend(b)
	}
}
