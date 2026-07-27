package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/enterprise/android-remote-access/server/internal/security"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/xuri/excelize/v2"
)

func (s *Server) canAccessArtifacts(admin *models.Administrator) bool {
	return s.permissionChecker.HasPermission(admin, "device:command") ||
		s.permissionChecker.HasPermission(admin, "location:read") ||
		s.permissionChecker.HasPermission(admin, "file:list") ||
		s.permissionChecker.HasPermission(admin, "camera:snapshot") ||
		s.permissionChecker.HasPermission(admin, "mic:record")
}

func (s *Server) handleSaveDeviceArtifacts(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.canAccessArtifacts(admin) {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}
	if s.db == nil {
		s.writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database is not available")
		return
	}

	deviceID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	var req models.ArtifactsSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	result := models.ArtifactsSaveResult{}
	if len(req.Locations) > 0 {
		if err := s.sealLocationMaps(deviceID, req.Locations); err != nil {
			s.writeError(w, http.StatusInternalServerError, "ENCRYPT_ERROR", err.Error())
			return
		}
		n, err := s.db.UpsertDeviceLocations(r.Context(), deviceID, req.Locations)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save locations: "+err.Error())
			return
		}
		result.LocationsSaved = n
	}
	if len(req.Files) > 0 {
		if err := s.sealFileMaps(deviceID, req.Files); err != nil {
			s.writeError(w, http.StatusInternalServerError, "ENCRYPT_ERROR", err.Error())
			return
		}
		n, err := s.db.UpsertDeviceFileEntries(r.Context(), deviceID, req.Files)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save file entries: "+err.Error())
			return
		}
		result.FilesSaved = n
	}
	if len(req.Media) > 0 {
		n, err := s.saveMediaItems(r, deviceID, req.Media)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save media: "+err.Error())
			return
		}
		result.MediaSaved = n
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"saved":  result,
	})
}

func (s *Server) saveMediaItems(r *http.Request, deviceID uuid.UUID, items []models.MediaSaveItem) (int, error) {
	saved := 0
	for _, item := range items {
		raw, mime, err := decodeMediaPayload(item)
		if err != nil || len(raw) == 0 {
			continue
		}
		fileType := strings.TrimSpace(item.FileType)
		if fileType == "" {
			if strings.HasPrefix(mime, "audio/") {
				fileType = "audio"
			} else if strings.HasPrefix(mime, "image/") {
				fileType = "image"
			} else {
				fileType = "document"
			}
		}
		fileName := strings.TrimSpace(item.FileName)
		if fileName == "" {
			ext := ".bin"
			switch fileType {
			case "image":
				ext = ".jpg"
			case "audio":
				ext = ".m4a"
			}
			fileName = fmt.Sprintf("%s_%s_%d%s", fileType, deviceID.String()[:8], time.Now().Unix(), ext)
		}
		source := item.Source
		if source == "" {
			source = "panel"
		}
		mf := &models.MediaFile{
			ID:            uuid.New(),
			DeviceID:      deviceID,
			FileName:      fileName,
			FileType:      fileType,
			FileSize:      int64(len(raw)),
			EncryptedData: nil,
			Checksum:      security.GenerateChecksum(raw),
			Source:        source,
			Camera:        item.Camera,
			MimeType:      mime,
			Latitude:      item.Latitude,
			Longitude:     item.Longitude,
			DataEntryDate: time.Now().UTC(),
			CreatedAt:     time.Now().UTC(),
		}
		payload, err := security.MustEncrypt(s.encryptor, raw, []byte(security.AADMediaRecord(mf.ID)))
		if err != nil {
			return saved, err
		}
		mf.EncryptedData = payload
		if err := s.db.CreateMediaFile(r.Context(), mf); err != nil {
			return saved, err
		}
		saved++
	}
	return saved, nil
}

func decodeMediaPayload(item models.MediaSaveItem) ([]byte, string, error) {
	src := strings.TrimSpace(item.DataURL)
	mime := strings.TrimSpace(item.MimeType)
	if src == "" {
		src = strings.TrimSpace(item.Base64)
	}
	if src == "" {
		return nil, "", fmt.Errorf("empty payload")
	}
	if strings.HasPrefix(src, "data:") {
		parts := strings.SplitN(src, ",", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid data url")
		}
		meta := parts[0]
		if mime == "" {
			if i := strings.Index(meta, ":"); i >= 0 {
				rest := meta[i+1:]
				if j := strings.Index(rest, ";"); j >= 0 {
					mime = rest[:j]
				} else {
					mime = rest
				}
			}
		}
		src = parts[1]
	}
	raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(src, "\n", ""))
	if err != nil {
		// try raw URL encoding variant
		raw, err = base64.RawStdEncoding.DecodeString(src)
		if err != nil {
			return nil, "", err
		}
	}
	if mime == "" {
		mime = "application/octet-stream"
		if len(raw) >= 2 && raw[0] == 0xff && raw[1] == 0xd8 {
			mime = "image/jpeg"
		}
	}
	return raw, mime, nil
}

func (s *Server) handleListDeviceArtifacts(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.canAccessArtifacts(admin) {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}
	if s.db == nil {
		s.writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database is not available")
		return
	}

	deviceID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	artType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if artType == "" {
		artType = "all"
	}
	limit := 5000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	resp := map[string]interface{}{
		"device_id": deviceID.String(),
		"type":      artType,
	}

	switch artType {
	case "location", "locations":
		items, err := s.db.ListDeviceLocations(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list locations")
			return
		}
		s.openLocations(items)
		resp["locations"] = items
		resp["count"] = len(items)
	case "files", "file_entries":
		items, err := s.db.ListDeviceFileEntries(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list file entries")
			return
		}
		s.openFileEntries(items)
		resp["files"] = items
		resp["count"] = len(items)
	case "media", "images", "audio", "image":
		ft := ""
		if artType == "images" || artType == "image" {
			ft = "image"
		} else if artType == "audio" {
			ft = "audio"
		}
		items, err := s.db.ListDeviceMedia(r.Context(), deviceID, ft, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list media")
			return
		}
		resp["media"] = items
		resp["count"] = len(items)
	case "all":
		locations, err := s.db.ListDeviceLocations(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list locations")
			return
		}
		files, err := s.db.ListDeviceFileEntries(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list file entries")
			return
		}
		media, err := s.db.ListDeviceMedia(r.Context(), deviceID, "", limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list media")
			return
		}
		s.openLocations(locations)
		s.openFileEntries(files)
		lc, fc, mc, _ := s.db.CountDeviceArtifacts(r.Context(), deviceID)
		resp["locations"] = locations
		resp["files"] = files
		resp["media"] = media
		resp["counts"] = map[string]int{"locations": lc, "files": fc, "media": mc}
	default:
		s.writeError(w, http.StatusBadRequest, "INVALID_TYPE", "type must be location, files, media, images, audio, or all")
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleExportDeviceArtifacts(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.canAccessArtifacts(admin) {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}
	if s.db == nil {
		s.writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database is not available")
		return
	}

	deviceID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	artType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if artType == "" {
		artType = "all"
	}

	f := excelize.NewFile()
	defer f.Close()
	defaultSheet := f.GetSheetName(0)
	wrote := 0

	writeLocations := func() error {
		items, err := s.db.ListDeviceLocations(r.Context(), deviceID, 100000)
		if err != nil {
			return err
		}
		s.openLocations(items)
		sheet := "Locations"
		if wrote == 0 {
			_ = f.SetSheetName(defaultSheet, sheet)
		} else {
			_, _ = f.NewSheet(sheet)
		}
		headers := []string{"Latitude", "Longitude", "Altitude", "Accuracy", "Provider", "Stale", "FixTime", "DataEntryDate"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, h)
		}
		for i, loc := range items {
			row := i + 2
			fixStr := ""
			if loc.FixTime != nil {
				fixStr = loc.FixTime.Format("1/2/2006 3:04:05 PM")
			}
			alt, acc := "", ""
			if loc.Altitude != nil {
				alt = strconv.FormatFloat(*loc.Altitude, 'f', -1, 64)
			}
			if loc.Accuracy != nil {
				acc = strconv.FormatFloat(*loc.Accuracy, 'f', -1, 64)
			}
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), loc.Latitude)
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), loc.Longitude)
			_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), alt)
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), acc)
			_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), loc.Provider)
			_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), loc.Stale)
			_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), fixStr)
			_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), loc.DataEntryDate.Format("1/2/2006 3:04:05 PM"))
		}
		wrote++
		return nil
	}

	writeFiles := func() error {
		items, err := s.db.ListDeviceFileEntries(r.Context(), deviceID, 200000)
		if err != nil {
			return err
		}
		sheet := "Files"
		if wrote == 0 {
			_ = f.SetSheetName(defaultSheet, sheet)
		} else {
			_, _ = f.NewSheet(sheet)
		}
		headers := []string{"Path", "Name", "IsDirectory", "Size", "Permissions", "ModifiedTime", "DataEntryDate"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, h)
		}
		for i, e := range items {
			row := i + 2
			mod := ""
			if e.ModifiedTime != nil {
				mod = e.ModifiedTime.Format("1/2/2006 3:04:05 PM")
			}
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), e.Path)
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), e.Name)
			_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), e.IsDirectory)
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), e.Size)
			_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), e.Permissions)
			_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), mod)
			_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), e.DataEntryDate.Format("1/2/2006 3:04:05 PM"))
		}
		wrote++
		return nil
	}

	writeMedia := func(fileType string) error {
		items, err := s.db.ListDeviceMedia(r.Context(), deviceID, fileType, 50000)
		if err != nil {
			return err
		}
		sheet := "Media"
		if fileType == "image" {
			sheet = "Images"
		} else if fileType == "audio" {
			sheet = "Audio"
		}
		if wrote == 0 {
			_ = f.SetSheetName(defaultSheet, sheet)
		} else {
			_, _ = f.NewSheet(sheet)
		}
		headers := []string{"Id", "FileName", "FileType", "FileSize", "Source", "Camera", "MimeType", "Latitude", "Longitude", "DataEntryDate", "DownloadPath"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, h)
		}
		for i, m := range items {
			row := i + 2
			lat, lng := "", ""
			if m.Latitude != nil {
				lat = strconv.FormatFloat(*m.Latitude, 'f', -1, 64)
			}
			if m.Longitude != nil {
				lng = strconv.FormatFloat(*m.Longitude, 'f', -1, 64)
			}
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), m.ID.String())
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), m.FileName)
			_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), m.FileType)
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), m.FileSize)
			_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), m.Source)
			_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), m.Camera)
			_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), m.MimeType)
			_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), lat)
			_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", row), lng)
			_ = f.SetCellValue(sheet, fmt.Sprintf("J%d", row), m.DataEntryDate.Format("1/2/2006 3:04:05 PM"))
			_ = f.SetCellValue(sheet, fmt.Sprintf("K%d", row), "/api/v1/media/file/"+m.ID.String())
		}
		wrote++
		return nil
	}

	switch artType {
	case "location", "locations":
		if err := writeLocations(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "files", "file_entries":
		if err := writeFiles(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "media":
		if err := writeMedia(""); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "images", "image":
		if err := writeMedia("image"); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "audio":
		if err := writeMedia("audio"); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "all":
		if err := writeLocations(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if err := writeFiles(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if err := writeMedia(""); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	default:
		s.writeError(w, http.StatusBadRequest, "INVALID_TYPE", "type must be location, files, media, images, audio, or all")
		return
	}

	filename := fmt.Sprintf("device-%s-%s-%s.xlsx", deviceID.String()[:8], artType, time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	if err := f.Write(w); err != nil {
		s.writeError(w, http.StatusInternalServerError, "EXPORT_ERROR", "Failed to write Excel file")
		return
	}
}
