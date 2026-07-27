package api

import (
	"encoding/json"
	"net/http"

	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleCameraAction(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "camera:snapshot") && !s.permissionChecker.HasPermission(admin, "camera:*") {
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
		Action string `json:"action"` // snapshot, stream, stop
		Camera string `json:"camera"` // front, back
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin)

	var result <-chan *dispatcher.CommandResult
	switch req.Action {
	case "snapshot":
		result = cmdBuilder.CameraSnapshot(deviceID, req.Camera)
	case "stream":
		s.writeJSON(w, http.StatusOK, map[string]string{"message": "Streaming not implemented"})
		return
	case "stop":
		// Handle camera stop
		s.writeJSON(w, http.StatusOK, map[string]string{"message": "Camera stopped"})
		return
	default:
		s.writeError(w, http.StatusBadRequest, "INVALID_ACTION", "Invalid camera action")
		return
	}

	cmdResult := <-result
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction_id": cmdBuilder.Command.TransactionID,
		"status":         cmdResult.Status,
	})
}
