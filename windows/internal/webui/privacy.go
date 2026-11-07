package webui

import (
	"time"

	"satellite/internal/privacy"
)

func (s *Server) PausePrivacy(duration time.Duration) {
	privacy.Default.Pause(duration)
	s.collector.ClearSensitive()
	s.mu.Lock()
	store := s.thumbs
	s.mu.Unlock()
	if store != nil {
		store.KeepDisplays(nil)
	}
}
