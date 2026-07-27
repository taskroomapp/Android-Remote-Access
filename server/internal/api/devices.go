package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "device:status") && !s.permissionChecker.HasPermission(admin, "device:command") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	onlineSet := make(map[uuid.UUID]struct{})
	for _, id := range s.hub.GetOnlineDeviceIDs() {
		onlineSet[id] = struct{}{}
	}

	seen := make(map[uuid.UUID]struct{})
	var onlineCount, offlineCount int
	deviceList := make([]models.DeviceOverview, 0)

	var devices []models.Device
	if s.db != nil {
		var err error
		devices, err = s.db.GetAllDevices(r.Context())
		if err != nil {
			log.Printf("GetAllDevices failed: %v", err)
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch devices")
			return
		}
	}

	for _, d := range devices {
		seen[d.ID] = struct{}{}
		status := d.Status
		if _, live := onlineSet[d.ID]; live {
			status = "online"
		}
		if status == "online" {
			onlineCount++
		} else {
			offlineCount++
		}
		deviceList = append(deviceList, models.DeviceOverview{
			ID:           d.ID.String(),
			FriendlyName: d.FriendlyName,
			Owner:        d.Owner,
			Status:       status,
			BatteryLevel: d.BatteryLevel,
			LastCheckIn:  d.LastCheckIn,
		})
	}

	for _, snap := range s.hub.OnlineDeviceSnapshots() {
		if _, ok := seen[snap.ID]; ok {
			continue
		}
		onlineCount++
		deviceList = append(deviceList, models.DeviceOverview{
			ID:           snap.ID.String(),
			FriendlyName: snap.FriendlyName,
			Owner:        "connected",
			Status:       "online",
			BatteryLevel: 0,
		})
	}

	s.writeJSON(w, http.StatusOK, models.DeviceListResponse{
		Devices:      deviceList,
		Total:        len(deviceList),
		OnlineCount:  onlineCount,
		OfflineCount: offlineCount,
	})
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "device:status") && !s.permissionChecker.HasPermission(admin, "device:command") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	device, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "NOT_FOUND", "Device not found")
		return
	}

	// Add online status from hub
	device.Status = "offline"
	if s.hub.IsDeviceOnline(deviceID) {
		device.Status = "online"
	}

	s.writeJSON(w, http.StatusOK, device)
}

func (s *Server) handleEnrollDevice(w http.ResponseWriter, r *http.Request) {
	var req models.EnrollDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	fp := req.KeyFingerprint
	if fp == "" {
		fp = req.CertificateHash
	}

	// Check if device already enrolled
	existing, _ := s.db.GetDeviceByUUID(r.Context(), req.DeviceUUID)
	if existing != nil {
		_ = s.db.UpdateDeviceCKX1Keys(r.Context(), existing.ID, req.X25519PublicKey, req.Ed25519PublicKey, fp)
		_ = s.db.UpdateDeviceMetadata(r.Context(), existing.ID, req.FriendlyName, req.OSVersion, req.HardwareModel)
		s.db.UpdateDeviceStatus(r.Context(), existing.ID, "online", 0)
		s.writeJSON(w, http.StatusOK, models.EnrollDeviceResponse{
			DeviceID:  existing.ID.String(),
			ServerURL: s.config.ServerHost + ":" + s.config.ServerPort,
			Message:   "Device re-enrolled",
			Status:    "enrolled",
		})
		return
	}

	// Create new device
	device := &models.Device{
		ID:               uuid.New(),
		FriendlyName:     req.FriendlyName,
		Owner:            req.Owner,
		OSVersion:        req.OSVersion,
		HardwareModel:    req.HardwareModel,
		DeviceUUID:       req.DeviceUUID,
		CertificateHash:  fp,
		X25519PublicKey:  req.X25519PublicKey,
		Ed25519PublicKey: req.Ed25519PublicKey,
		KeyFingerprint:   fp,
		KeyVersion:       1,
		KeyCreatedAt:     time.Now(),
		Status:           "online",
		EnrolledAt:       time.Now(),
	}

	if err := s.db.CreateDevice(r.Context(), device); err != nil {
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to enroll device")
		return
	}

	s.writeJSON(w, http.StatusCreated, models.EnrollDeviceResponse{
		DeviceID:  device.ID.String(),
		ServerURL: s.config.ServerHost + ":" + s.config.ServerPort,
		Message:   "Device enrolled successfully",
		Status:    "enrolled",
	})
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "device:revoke") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	if err := s.db.DeleteDevice(r.Context(), deviceID); err != nil {
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to delete device")
		return
	}

	// Disconnect device if online
	if conn := s.hub.GetDeviceConnection(deviceID); conn != nil {
		conn.Close()
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Device deleted"})
}

func (s *Server) handleGetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	device, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "NOT_FOUND", "Device not found")
		return
	}

	// Get real-time status from hub
	online := s.hub.IsDeviceOnline(deviceID)

	var pendingCommands int64
	if s.cache != nil {
		pendingCommands, _ = s.cache.GetQueuedCommandCount(r.Context(), deviceID)
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":        device.ID,
		"status":           map[string]bool{"online": online, "offline": !online}["online"],
		"online":           online,
		"battery_level":    device.BatteryLevel,
		"last_check_in":    device.LastCheckIn,
		"pending_commands": pendingCommands,
	})
}
