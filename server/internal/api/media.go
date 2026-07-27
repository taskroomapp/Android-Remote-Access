package api

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "file:read") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["device_id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	mediaFiles, err := s.db.GetMediaFilesByDevice(r.Context(), deviceID, 50)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch media")
		return
	}

	s.writeJSON(w, http.StatusOK, mediaFiles)
}

func (s *Server) handleGetMediaFile(w http.ResponseWriter, r *http.Request) {
	s.handleDownloadFile(w, r)
}
