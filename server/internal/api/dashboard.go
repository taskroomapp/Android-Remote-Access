package api

import (
	"net/http"
)

func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "audit:read") &&
		!s.permissionChecker.HasPermission(admin, "device:command") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	stats, err := s.db.GetDashboardStats(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch dashboard stats")
		return
	}

	stats.OnlineDevices = s.hub.GetOnlineDeviceCount()
	if stats.OnlineDevices+stats.OfflineDevices > stats.TotalDevices {
		stats.TotalDevices = stats.OnlineDevices + stats.OfflineDevices
	}

	s.writeJSON(w, http.StatusOK, stats)
}
