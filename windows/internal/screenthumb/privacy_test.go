package screenthumb

import (
	"image"
	"testing"
	"time"

	"satellite/internal/privacy"
)

func TestPauseInvalidatesAllDisplayFramesAfterResume(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	s := NewStore()
	for _, display := range []int{0, 1, 3} {
		s.SetDisplay(display, []byte{1, 2, 3})
	}
	privacy.Default.Pause(time.Minute)
	for _, display := range []int{0, 1, 3} {
		if _, _, ok := s.LatestDisplay(display); ok {
			t.Fatalf("display %d remained visible while paused", display)
		}
		s.SetDisplay(display, []byte{4, 5, 6})
	}
	privacy.Default.Resume()
	for _, display := range []int{0, 1, 3} {
		if _, _, ok := s.LatestDisplay(display); ok {
			t.Fatalf("display %d restored a stale frame", display)
		}
	}
	s.SetDisplay(3, []byte{7})
	if _, _, ok := s.LatestDisplay(3); !ok {
		t.Fatal("fresh frame was rejected after resume")
	}
}

func TestPausedRunnerDoesNotCapture(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	f := newFakes()
	r := f.runner()
	r.Capture = func(int) (image.Image, error) {
		t.Fatal("capture called during privacy pause")
		return nil, nil
	}
	privacy.Default.Pause(time.Minute)
	if result, err := r.Tick(); result != Skipped || err != nil {
		t.Fatalf("Tick = %v, %v", result, err)
	}
}

func TestRunnerDropsCaptureAcrossPauseAndResume(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	f := newFakes()
	r := f.runner()
	r.Store = NewStore()
	r.Capture = func(int) (image.Image, error) {
		privacy.Default.Pause(time.Minute)
		privacy.Default.Resume()
		return image.NewRGBA(image.Rect(0, 0, 4, 4)), nil
	}
	r.Publish = func([]byte) error {
		t.Fatal("pre-pause capture was published after resume")
		return nil
	}
	if result, err := r.Tick(); result != Skipped || err != nil {
		t.Fatalf("Tick = %v, %v", result, err)
	}
	if _, _, ok := r.Store.Latest(); ok {
		t.Fatal("pre-pause capture was stored after resume")
	}
}

func TestStoreRejectsFramesFromEarlierPauseGeneration(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	generation := privacy.Default.Status().Generation
	privacy.Default.Pause(time.Minute)
	privacy.Default.Resume()
	s := NewStore()
	s.setDisplay(1, []byte{1, 2, 3}, generation)
	if _, _, ok := s.LatestDisplay(1); ok {
		t.Fatal("store relabeled an old capture as a fresh frame")
	}
}
