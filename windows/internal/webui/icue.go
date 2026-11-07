package webui

import (
	"net/http"

	"satellite/internal/icue"
)

func (s *Server) handleICUESDK(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	check := s.checkICUESDK
	if check == nil {
		check = icue.CheckSDK
	}
	writeJSON(w, http.StatusOK, check())
}
