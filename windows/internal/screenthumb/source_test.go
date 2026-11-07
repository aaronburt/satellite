package screenthumb

import (
	"errors"
	"fmt"
	"image"
	"testing"
	"time"
)

type fakeGPU struct {
	captures int
	closed   bool
	err      error
	doPanic  bool
}

func (f *fakeGPU) Capture(int) (image.Image, error) {
	f.captures++
	if f.doPanic {
		panic("boom")
	}
	if f.err != nil {
		return nil, f.err
	}
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

func (f *fakeGPU) Close() { f.closed = true }

type sourceHarness struct {
	useGPU    bool
	now       time.Time
	gdiCalls  int
	gdiErr    error
	opened    []*fakeGPU
	openErr   error
	next      func() *fakeGPU
	backends  []Backend
	fallbacks []error
	src       *Source
}

func newHarness() *sourceHarness {
	h := &sourceHarness{now: time.Unix(1000, 0)}
	h.next = func() *fakeGPU { return &fakeGPU{} }
	h.src = &Source{
		UseGPU: func() bool { return h.useGPU },
		GDI: func(int) (image.Image, error) {
			h.gdiCalls++
			if h.gdiErr != nil {
				return nil, h.gdiErr
			}
			return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
		},
		NewGPU: func() (gpuCapturer, error) {
			if h.openErr != nil {
				return nil, h.openErr
			}
			g := h.next()
			h.opened = append(h.opened, g)
			return g, nil
		},
		OnBackend:  func(b Backend) { h.backends = append(h.backends, b) },
		OnFallback: func(err error) { h.fallbacks = append(h.fallbacks, err) },
		Now:        func() time.Time { return h.now },
	}
	return h
}

func (h *sourceHarness) capture(t *testing.T) image.Image {
	t.Helper()
	img, err := h.src.Capture(200)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestSourceUsesGDIWhenGPUIsOff(t *testing.T) {
	h := newHarness()
	h.capture(t)
	h.capture(t)
	if h.gdiCalls != 2 || len(h.opened) != 0 {
		t.Fatalf("gdi=%d gpuOpened=%d; the GPU must not even be opened when off", h.gdiCalls, len(h.opened))
	}
	if h.src.Backend() != BackendGDI {
		t.Fatalf("backend = %s", h.src.Backend())
	}
}

func TestSourceUsesGPUWhenOnAndWorking(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	img := h.capture(t)
	h.capture(t)
	if h.gdiCalls != 0 {
		t.Fatalf("gdi used %d times", h.gdiCalls)
	}
	if len(h.opened) != 1 || h.opened[0].captures != 2 {
		t.Fatalf("the GPU capturer should open once and be reused: %+v", h.opened)
	}
	if img.Bounds().Dx() != 1 {
		t.Fatal("did not return the GPU image")
	}
	if h.src.Backend() != BackendGPU {
		t.Fatalf("backend = %s", h.src.Backend())
	}
	if len(h.backends) != 1 || h.backends[0] != BackendGPU {
		t.Fatalf("backend events = %v, want one change to gpu", h.backends)
	}
}

func TestSourceFallsBackToGDIWhenGPUCannotOpen(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.openErr = errors.New("no gpu")
	img := h.capture(t)
	if img.Bounds().Dx() != 2 || h.gdiCalls != 1 {
		t.Fatal("expected the GDI image on the same tick")
	}
	if len(h.fallbacks) != 1 {
		t.Fatalf("fallback events = %v", h.fallbacks)
	}
	if h.src.Backend() != BackendGDI {
		t.Fatalf("backend = %s", h.src.Backend())
	}
}

func TestSourceFallsBackWhenGPUCaptureFailsAndClosesIt(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.next = func() *fakeGPU { return &fakeGPU{err: errors.New("access lost")} }
	img := h.capture(t)
	if img.Bounds().Dx() != 2 || h.gdiCalls != 1 {
		t.Fatal("a failed GPU capture must still produce a GDI image that tick")
	}
	if !h.opened[0].closed {
		t.Fatal("the broken GPU capturer must be closed")
	}
}

func TestSourceSurvivesGPUPanic(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.next = func() *fakeGPU { return &fakeGPU{doPanic: true} }
	img := h.capture(t)
	if img.Bounds().Dx() != 2 {
		t.Fatal("expected GDI image after a GPU panic")
	}
	if len(h.fallbacks) != 1 {
		t.Fatalf("fallbacks = %v", h.fallbacks)
	}
}

func TestSourceSurvivesGPUOpenPanic(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.src.NewGPU = func() (gpuCapturer, error) { panic("open boom") }
	if img := h.capture(t); img.Bounds().Dx() != 2 {
		t.Fatal("expected GDI image after an open panic")
	}
}

func TestSourceBacksOffBeforeRetryingGPU(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	attempts := 0
	h.src.NewGPU = func() (gpuCapturer, error) {
		attempts++
		return nil, errors.New("no gpu")
	}

	h.capture(t)
	h.now = h.now.Add(10 * time.Second)
	h.capture(t)
	h.capture(t)
	if attempts != 1 {
		t.Fatalf("attempts = %d inside the back-off window, want 1", attempts)
	}

	h.now = h.now.Add(gpuRetryMin)
	h.capture(t)
	if attempts != 2 {
		t.Fatalf("attempts = %d after the back-off, want 2", attempts)
	}

	h.now = h.now.Add(gpuRetryMin)
	h.capture(t)
	if attempts != 2 {
		t.Fatalf("second back-off should be longer; attempts = %d", attempts)
	}
	h.now = h.now.Add(gpuRetryMin)
	h.capture(t)
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestSourceBackOffIsCapped(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.src.NewGPU = func() (gpuCapturer, error) { return nil, errors.New("no gpu") }
	for i := 0; i < 20; i++ {
		h.capture(t)
		h.now = h.now.Add(gpuRetryMax)
	}
	h.capture(t)
	if got := h.src.retryAt.Sub(h.now); got > gpuRetryMax || got <= 0 {
		t.Fatalf("retry delay = %v, want within (0, %v]", got, gpuRetryMax)
	}
}

func TestSourceRecoversAfterBackOff(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.openErr = errors.New("no gpu")
	h.capture(t)
	h.openErr = nil
	h.now = h.now.Add(gpuRetryMin)
	h.capture(t)
	if h.src.Backend() != BackendGPU {
		t.Fatalf("backend = %s, want gpu once the GPU works again", h.src.Backend())
	}
	if want := []Backend{BackendGDI, BackendGPU}; fmt.Sprint(h.backends) != fmt.Sprint(want) {
		t.Fatalf("backend events = %v, want %v", h.backends, want)
	}
}

func TestSourceReleasesGPUWhenSwitchedOff(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.capture(t)
	g := h.opened[0]

	h.useGPU = false
	h.capture(t)
	if !g.closed {
		t.Fatal("turning the GPU option off must release the GPU")
	}
	if h.gdiCalls != 1 {
		t.Fatalf("gdi calls = %d", h.gdiCalls)
	}

	h.useGPU = true
	h.capture(t)
	if len(h.opened) != 2 {
		t.Fatalf("opened %d capturers, want a fresh one after re-enabling", len(h.opened))
	}
}

func TestSourceTurningOffResetsBackOff(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.openErr = errors.New("no gpu")
	h.capture(t)
	h.useGPU = false
	h.capture(t)
	h.useGPU = true
	h.openErr = nil
	h.capture(t)
	if h.src.Backend() != BackendGPU {
		t.Fatalf("backend = %s; re-enabling should try the GPU straight away", h.src.Backend())
	}
}

func TestSourceCloseReleasesGPU(t *testing.T) {
	h := newHarness()
	h.useGPU = true
	h.capture(t)
	h.src.Close()
	if !h.opened[0].closed {
		t.Fatal("Close must release the GPU")
	}
	h.src.Close()
}

func TestSourceReturnsGDIErrors(t *testing.T) {
	h := newHarness()
	h.gdiErr = errors.New("secure desktop")
	if _, err := h.src.Capture(200); err == nil {
		t.Fatal("want the GDI error")
	}
	if h.src.Backend() != "" {
		t.Fatalf("backend = %q after a failed capture", h.src.Backend())
	}
}

func TestSourceWithoutUseGPUHookIsGDI(t *testing.T) {
	h := newHarness()
	h.src.UseGPU = nil
	h.capture(t)
	if h.gdiCalls != 1 {
		t.Fatal("a source with no GPU switch must use GDI")
	}
}

func TestNewSourceDefaultsToGDIOffWindowsGPU(t *testing.T) {
	s := NewSource(func() bool { return false })
	if s.GDI == nil || s.NewGPU == nil {
		t.Fatal("NewSource must wire both backends")
	}
}
