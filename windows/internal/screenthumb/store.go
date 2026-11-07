package screenthumb

import (
	"sync"
	"time"
)

// Store keeps the most recent encoded thumbnail so the web UI can show it.
// Only one small JPEG is ever held, and it is dropped as soon as the sensor
// is turned off.
type Store struct {
	mu   sync.RWMutex
	jpeg []byte
	at   time.Time
	now  func() time.Time
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
	cp := append([]byte(nil), jpeg...)
	s.mu.Lock()
	s.jpeg = cp
	s.at = s.clock()
	s.mu.Unlock()
}

// Clear forgets the stored frame.
func (s *Store) Clear() {
	s.mu.Lock()
	s.jpeg = nil
	s.at = time.Time{}
	s.mu.Unlock()
}

// Latest returns the stored frame and when it was captured. ok is false when
// there is no frame. The returned slice must not be modified.
func (s *Store) Latest() (jpeg []byte, at time.Time, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.jpeg) == 0 {
		return nil, time.Time{}, false
	}
	return s.jpeg, s.at, true
}
