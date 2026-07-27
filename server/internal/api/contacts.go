package api

import (
	"net/http"

	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleGetContacts(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "contacts:read") {
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
	result := <-cmdBuilder.GetContacts(deviceID)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction_id": cmdBuilder.Command.TransactionID.String(),
		"status":         result.Status,
		"data":           dispatcher.EncodeCommandData(result.Data),
		"error":          result.Error,
	})
}
