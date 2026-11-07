package screenthumb

import (
	"sync"
	"time"

	"satellite/internal/privacy"
)

type Store struct {
	mu     sync.RWMutex
	now    func() time.Time
	frames map[int]storedFrame
}

type storedFrame struct {
	jpeg       []byte
	at         time.Time
	generation uint64
}

func NewStore() *Store {
	return &Store{now: time.Now}
}

func (s *Store) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Set replaces the stored frame with a copy of jpeg.
func (s *Store) Set(jpeg []byte) {
	s.SetDisplay(0, jpeg)
}

func (s *Store) SetDisplay(display int, jpeg []byte) {
	status := privacy.Default.Status()
	s.setDisplay(display, jpeg, status.Generation)
}

func (s *Store) setDisplay(display int, jpeg []byte, generation uint64) {
	if !privacy.Default.Allows(generation) {
		return
	}
	cp := append([]byte(nil), jpeg...)
	s.mu.Lock()
	if s.frames == nil {
		s.frames = make(map[int]storedFrame)
	}
	s.frames[display] = storedFrame{jpeg: cp, at: s.clock(), generation: generation}
	s.mu.Unlock()
}

// Clear forgets the stored frame.
func (s *Store) Clear() {
	s.ClearDisplay(0)
}

func (s *Store) ClearDisplay(display int) {
	s.mu.Lock()
	delete(s.frames, display)
	s.mu.Unlock()
}

func (s *Store) KeepDisplays(ids []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.frames {
		keep := false
		for _, selected := range ids {
			keep = keep || selected == id
		}
		if !keep {
			delete(s.frames, id)
		}
	}
}

// Latest returns the stored frame and when it was captured. ok is false when
// there is no frame. The returned slice must not be modified.
func (s *Store) Latest() (jpeg []byte, at time.Time, ok bool) {
	return s.LatestDisplay(0)
}

func (s *Store) LatestDisplay(display int) (jpeg []byte, at time.Time, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	frame := s.frames[display]
	if len(frame.jpeg) == 0 || !privacy.Default.Allows(frame.generation) {
		return nil, time.Time{}, false
	}
	return frame.jpeg, frame.at, true
}
