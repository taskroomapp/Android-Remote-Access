package api

import (
	"net/http"

	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleLocationAction(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "location:read") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["device_id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin)
	result := <-cmdBuilder.GetLocation(deviceID)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction_id": cmdBuilder.Command.TransactionID,
		"status":         result.Status,
		"data":           dispatcher.EncodeCommandData(result.Data),
		"error":          result.Error,
	})
}

func (s *Server) handleForegroundAppAction(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "device:command") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["device_id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin)
	result := <-cmdBuilder.GetForegroundApp(deviceID)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction_id": cmdBuilder.Command.TransactionID,
		"status":         result.Status,
		"data":           dispatcher.EncodeCommandData(result.Data),
		"error":          result.Error,
	})
}
