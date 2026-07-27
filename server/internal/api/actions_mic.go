package api

import (
	"encoding/json"
	"net/http"

	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleMicAction(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "mic:record") && !s.permissionChecker.HasPermission(admin, "mic:*") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["device_id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	var req struct {
		Action   string `json:"action"`   // start, stop, stream
		Duration int    `json:"duration"` // seconds for start
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin)

	switch req.Action {
	case "start":
		duration := req.Duration
		if duration == 0 {
			duration = 60
		}
		result := <-cmdBuilder.MicStart(deviceID, duration)
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"transaction_id": cmdBuilder.Command.TransactionID,
			"status":         result.Status,
		})
	case "stop":
		s.writeJSON(w, http.StatusOK, map[string]string{"message": "Microphone stopped"})
	default:
		s.writeError(w, http.StatusBadRequest, "INVALID_ACTION", "Invalid microphone action")
	}
}
