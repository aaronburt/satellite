package webui

import (
	"fmt"
	"net/http"

	"satellite/internal/screenthumb"
)

func (s *Server) getDisplays() ([]screenthumb.Display, error) {
	if s.listDisplays != nil {
		return s.listDisplays()
	}
	return screenthumb.Displays()
}

func (s *Server) handleDisplays(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	displays, err := s.getDisplays()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Could not enumerate connected displays")
		return
	}
	if displays == nil {
		displays = []screenthumb.Display{}
	}
	writeJSON(w, http.StatusOK, displays)
}

func (s *Server) displayFrames(ids []int, store *screenthumb.Store) []DisplayFrame {
	displays, err := s.getDisplays()
	frames := make([]DisplayFrame, 0, len(ids))
	for _, id := range screenthumb.NormalizeDisplays(ids) {
		_, available := screenthumb.ResolveDisplay(displays, id)
		frame := DisplayFrame{ID: id, Name: fmt.Sprintf("Screen thumbnail Display %d", id), Available: available && err == nil}
		if store != nil && frame.Available {
			_, frame.At, frame.Have = store.LatestDisplay(id)
		}
		frames = append(frames, frame)
	}
	return frames
}
