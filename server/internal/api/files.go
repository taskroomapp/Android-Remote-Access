package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/enterprise/android-remote-access/server/internal/security"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleFileList(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "file:list") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	deviceIDStr := r.URL.Query().Get("device_id")
	path := r.URL.Query().Get("path")

	if deviceIDStr == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PARAM", "device_id is required")
		return
	}

	deviceID, _ := uuid.Parse(deviceIDStr)

	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin).WithContext(getClientIP(r), r.UserAgent())
	result := <-cmdBuilder.FileList(deviceID, path)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction_id": cmdBuilder.Command.TransactionID,
		"status":         result.Status,
		"data":           dispatcher.EncodeCommandData(result.Data),
		"error":          result.Error,
	})
}

func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "file:read") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	deviceIDStr := r.URL.Query().Get("device_id")
	path := r.URL.Query().Get("path")

	if deviceIDStr == "" || path == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PARAM", "device_id and path are required")
		return
	}

	deviceID, _ := uuid.Parse(deviceIDStr)

	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin)
	result := <-cmdBuilder.FileRead(deviceID, path)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction_id": cmdBuilder.Command.TransactionID,
		"status":         result.Status,
		"data":           dispatcher.EncodeCommandData(result.Data),
		"error":          result.Error,
	})
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "file:*") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	deviceIDStr := r.URL.Query().Get("device_id")
	path := r.URL.Query().Get("path")

	if deviceIDStr == "" || path == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PARAM", "device_id and path are required")
		return
	}

	deviceID, _ := uuid.Parse(deviceIDStr)

	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin)
	result := <-cmdBuilder.FileDelete(deviceID, path)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction_id": cmdBuilder.Command.TransactionID,
		"status":         result.Status,
	})
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "file:read") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	fileID, err := uuid.Parse(vars["file_id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid file ID")
		return
	}

	mediaFile, err := s.db.GetMediaFileByID(r.Context(), fileID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}

	plaintext := mediaFile.EncryptedData
	if len(mediaFile.EncryptedData) == 0 {
		s.writeError(w, http.StatusNotFound, "NOT_FOUND", "File has no data")
		return
	}
	if s.encryptor == nil {
		s.writeError(w, http.StatusInternalServerError, "ENCRYPTOR_REQUIRED", "Media decryptor not configured")
		return
	}
	decrypted, err := s.encryptor.DecryptWithAAD(mediaFile.EncryptedData, []byte(security.AADMediaRecord(mediaFile.ID)))
	if err != nil {
		log.Printf("Failed to decrypt media file %s: %v", fileID, err)
		s.writeError(w, http.StatusInternalServerError, "DECRYPT_ERROR", "Failed to decrypt media file")
		return
	}
	plaintext = decrypted

	contentType := mediaContentType(mediaFile.FileType, mediaFile.FileName)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename="+mediaFile.FileName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(plaintext)))
	w.Write(plaintext)
}
