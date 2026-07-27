package api

import (
	"encoding/json"
	"net/http"

	"github.com/enterprise/android-remote-access/server/internal/models"
)

func (s *Server) handleSearchAuditLogs(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "audit:read") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	var req models.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.PageSize == 0 {
		req.PageSize = 50
	}

	logs, total, err := s.db.SearchAuditLogs(r.Context(), req)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to search audit logs")
		return
	}

	var entries []models.AuditLogEntry
	for _, l := range logs {
		entries = append(entries, models.AuditLogEntry{
			ID:                l.ID.String(),
			TransactionID:     l.TransactionID.String(),
			AdministratorID:   l.AdministratorID.String(),
			AdministratorName: l.AdministratorName,
			DeviceID:          l.DeviceID.String(),
			DeviceName:        l.DeviceName,
			CommandType:       l.CommandType,
			CommandPayload:    l.CommandPayload,
			Status:            l.ResponseStatus,
			IPAddress:         l.IPAddress,
			Timestamp:         l.Timestamp,
		})
	}

	totalPages := int(total) / req.PageSize
	if int(total)%req.PageSize > 0 {
		totalPages++
	}

	s.writeJSON(w, http.StatusOK, models.SearchResponse{
		Results:    entries,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "audit:read") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Get single audit log"})
}
