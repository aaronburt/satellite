package screenthumb

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

func TestStoreStartsEmpty(t *testing.T) {
	if _, _, ok := NewStore().Latest(); ok {
		t.Fatal("a new store must have no frame")
	}
}

func TestStoreKeepsLatestFrameAndTime(t *testing.T) {
	s := NewStore()
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	s.Set([]byte{1, 2, 3})
	now = now.Add(5 * time.Second)
	s.Set([]byte{4, 5})

	got, at, ok := s.Latest()
	if !ok || !bytes.Equal(got, []byte{4, 5}) {
		t.Fatalf("Latest = %v, %v; want the second frame", got, ok)
	}
	if !at.Equal(now) {
		t.Errorf("frame time = %v, want %v", at, now)
	}
}

func TestStoreCopiesTheFrame(t *testing.T) {
	s := NewStore()
	buf := []byte{1, 2, 3}
	s.Set(buf)
	buf[0] = 99
	if got, _, _ := s.Latest(); got[0] != 1 {
		t.Fatalf("store shares the caller's buffer: %v", got)
	}
}

func TestStoreClearForgetsTheFrame(t *testing.T) {
	s := NewStore()
	s.Set([]byte{1})
	s.Clear()
	if _, at, ok := s.Latest(); ok || !at.IsZero() {
		t.Fatal("Clear must drop the frame")
	}
}

func TestStoreEmptyFrameCountsAsNone(t *testing.T) {
	s := NewStore()
	s.Set(nil)
	if _, _, ok := s.Latest(); ok {
		t.Fatal("an empty frame is not a frame")
	}
}

func TestStoreIsSafeForConcurrentUse(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				s.Set([]byte{byte(j)})
				s.Latest()
				if j%50 == 0 {
					s.Clear()
				}
			}
		}()
	}
	wg.Wait()
}

func TestTickStoresTheFrameForTheWebUI(t *testing.T) {
	f := newFakes()
	r := f.runner()
	r.Store = NewStore()
	r.Publish = func([]byte) error { return nil }
	if res, err := r.Tick(); err != nil || res != Published {
		t.Fatalf("Tick = %v, %v", res, err)
	}
	got, _, ok := r.Store.Latest()
	if !ok || !bytes.HasPrefix(got, []byte{0xff, 0xd8}) {
		t.Fatal("the captured JPEG was not stored")
	}
}

func TestTickStoresTheFrameEvenIfPublishFails(t *testing.T) {
	f := newFakes()
	f.publishErr = errBoom
	r := f.runner()
	r.Store = NewStore()
	if res, _ := r.Tick(); res != Failed {
		t.Fatalf("Tick = %v, want Failed", res)
	}
	if _, _, ok := r.Store.Latest(); !ok {
		t.Fatal("a broker problem must not blank the web UI preview")
	}
}

func TestTickDoesNotStoreFailedCaptures(t *testing.T) {
	f := newFakes()
	f.captureErr = errBoom
	r := f.runner()
	r.Store = NewStore()
	r.Tick()
	if _, _, ok := r.Store.Latest(); ok {
		t.Fatal("nothing was captured, so nothing should be stored")
	}
}

func TestTickClearsTheStoredFrameWhenDisabled(t *testing.T) {
	f := newFakes()
	r := f.runner()
	r.Store = NewStore()
	r.Store.Set([]byte{1, 2, 3})
	f.enabled = false
	r.Tick()
	if _, _, ok := r.Store.Latest(); ok {
		t.Fatal("turning the sensor off must drop the last screenshot")
	}
}

func TestTickKeepsTheFrameWhileLocked(t *testing.T) {
	f := newFakes()
	r := f.runner()
	r.Store = NewStore()
	r.Store.Set([]byte{1, 2, 3})
	f.locked = true
	r.Tick()
	if _, _, ok := r.Store.Latest(); !ok {
		t.Fatal("a locked session skips capture but keeps showing the last frame")
	}
}
