package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleExecuteCommand(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "device:command") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	var req models.CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	if !s.validator.ValidateCommandType(req.CommandType) {
		s.writeError(w, http.StatusBadRequest, "INVALID_COMMAND", "Invalid command type")
		return
	}

	timeout := req.TimeoutSeconds
	if timeout == 0 {
		timeout = 60
	}

	priority := req.Priority
	if priority == 0 {
		priority = 5
	}

	cmd := &dispatcher.CommandTask{
		Command: &models.DeviceCommand{
			TransactionID:  uuid.New(),
			CommandType:    models.CommandType(req.CommandType),
			TargetDeviceID: deviceID,
			Payload:        req.Payload,
			TimeoutSeconds: timeout,
			Priority:       priority,
			IssuedBy:       admin.ID,
			IssuedAt:       time.Now(),
		},
		Admin:     admin,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
	}

	result := <-s.dispatcher.ExecuteCommand(cmd)

	resp := map[string]interface{}{
		"transaction_id": cmd.Command.TransactionID.String(),
		"status":         result.Status,
		"queued":         result.Status == "queued",
		"created_at":     time.Now(),
	}
	if len(result.Data) > 0 {
		resp["data"] = dispatcher.EncodeCommandData(result.Data)
	}
	if result.Error != "" {
		resp["error"] = result.Error
		resp["message"] = result.Error
	}

	statusCode := http.StatusOK
	if result.Status == "queued" {
		statusCode = http.StatusAccepted
	}
	s.writeJSON(w, statusCode, resp)
}

func (s *Server) handleGetCommandStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transactionID, err := uuid.Parse(vars["transaction_id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid transaction ID")
		return
	}

	result, err := s.dispatcher.GetCommandStatus(transactionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "NOT_FOUND", "Command status not found")
		return
	}

	s.writeJSON(w, http.StatusOK, result.ForJSON())
}

func (s *Server) handleCancelCommand(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Command cancellation not implemented"})
}
