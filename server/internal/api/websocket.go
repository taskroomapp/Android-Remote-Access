package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/enterprise/android-remote-access/server/internal/websocket"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) handleDeviceWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pathID, err := uuid.Parse(vars["device_id"])
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	deviceID := pathID
	if s.db != nil {
		if agentUUID := strings.TrimSpace(r.Header.Get("X-Device-UUID")); agentUUID != "" {
			if dev, err := s.db.GetDeviceByUUID(r.Context(), agentUUID); err == nil && dev != nil {
				deviceID = dev.ID
			}
		}
		if dev, err := s.db.GetDeviceByUUID(r.Context(), pathID.String()); err == nil && dev != nil {
			deviceID = dev.ID
		}
	}

	conn, err := websocket.UpgradeDeviceConnection(w, r, deviceID)
	if err != nil {
		log.Printf("Device WebSocket upgrade failed: %v", err)
		return
	}

	agentUUID := strings.TrimSpace(r.Header.Get("X-Device-UUID"))
	if agentUUID == "" {
		agentUUID = pathID.String()
	}

	client := &websocket.Client{
		ID:         deviceID,
		DeviceUUID: agentUUID,
		Conn:       conn,
		Hub:        s.hub,
		Send:       make(chan []byte, websocket.MaxBufferSize),
	}

	s.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) handleAdminWebSocket(w http.ResponseWriter, r *http.Request) {
	admin, err := s.authenticateAdmin(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.UpgradeAdminConnection(w, r)
	if err != nil {
		log.Printf("Admin WebSocket upgrade failed: %v", err)
		return
	}

	session := &websocket.AdminSession{
		ID:      uuid.New().String(),
		AdminID: admin.ID,
		Conn:    conn,
		Send:    make(chan []byte, websocket.MaxBufferSize),
	}

	if ckx1ID := strings.TrimSpace(r.URL.Query().Get("ckx1")); ckx1ID != "" && s.adminCKX1 != nil {
		if ckx := s.adminCKX1.Get(ckx1ID); ckx != nil && ckx.AdminID == admin.ID && ckx.Ready() {
			session.EncryptOutbound = func(plain []byte) ([]byte, error) {
				frame, err := ckx.SealAdmin(plain)
				if err != nil {
					return nil, err
				}
				return json.Marshal(frame)
			}
			session.DecryptInbound = func(raw []byte) ([]byte, error) {
				var frame map[string]interface{}
				if err := json.Unmarshal(raw, &frame); err != nil {
					return nil, err
				}
				if t, _ := frame["type"].(string); t != "enc" {
					return raw, nil
				}
				return ckx.OpenAdmin(frame)
			}
		}
	}

	s.hub.RegisterAdminSession(session)

	go session.WritePump()
	go session.ReadPump()
}
