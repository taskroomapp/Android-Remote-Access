package websocket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/database"
	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 30 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer (camera JPEG + CKX1 envelope)
	maxMessageSize = 8 * 1024 * 1024 // 8MB

	// Max pending messages in buffer
	maxBufferSize = 256

	// MaxBufferSize is exported for handlers creating client send channels.
	MaxBufferSize = maxBufferSize
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered devices
	devices map[uuid.UUID]*Client

	// Device connections for quick lookup
	deviceConns map[uuid.UUID]*websocket.Conn

	// Admin sessions watching device updates
	adminSessions map[*AdminSession]bool

	// Register requests from device clients
	register chan *Client

	// Unregister requests from device clients
	unregister chan *Client

	// Broadcast to all admin sessions
	broadcast chan *DeviceUpdate

	// Mutex for thread-safe access
	mu sync.RWMutex

	// Database for persistence
	db *database.PostgresDB

	// Redis cache
	cache *database.RedisCache

	// Command response handlers
	responseHandlers map[uuid.UUID]chan *models.AgentResponse

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// CKX1 long-term server identity (X25519 + Ed25519)
	identity *ServerIdentity
}

// DeviceUpdate represents an update about device status
type DeviceUpdate struct {
	Type      string      `json:"type"`
	DeviceID  uuid.UUID   `json:"device_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// AdminSession represents an admin's WebSocket connection
type AdminSession struct {
	ID           string
	AdminID      uuid.UUID
	DeviceFilter []uuid.UUID // If set, only receive updates for these devices
	Conn         *websocket.Conn
	Send         chan []byte
	Hub          *Hub
	// EncryptOutbound optionally wraps plaintext JSON as a CKX1 enc frame.
	EncryptOutbound func([]byte) ([]byte, error)
	// DecryptInbound optionally opens a CKX1 enc frame to plaintext JSON.
	DecryptInbound func([]byte) ([]byte, error)
}

// Client represents a device WebSocket client
type Client struct {
	ID           uuid.UUID
	DeviceUUID   string
	FriendlyName string
	Conn         *websocket.Conn
	Send         chan []byte
	Hub          *Hub

	sessionMu        sync.RWMutex
	ckx1             *ckx1DeviceSession
	offerSessionID   string
	offerNonce       string
	offerFingerprint string
}

// SetCKX1Identity configures the server CKX1 identity keys.
func (h *Hub) SetCKX1Identity(id *ServerIdentity) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.identity = id
}

// OnlineDeviceSnapshot summarizes a connected device for API listings.
type OnlineDeviceSnapshot struct {
	ID           uuid.UUID
	DeviceUUID   string
	FriendlyName string
}

// NewHub creates a new Hub instance
func NewHub(db *database.PostgresDB, cache *database.RedisCache) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		devices:          make(map[uuid.UUID]*Client),
		deviceConns:      make(map[uuid.UUID]*websocket.Conn),
		adminSessions:    make(map[*AdminSession]bool),
		register:         make(chan *Client),
		unregister:       make(chan *Client),
		broadcast:        make(chan *DeviceUpdate, 256),
		db:               db,
		cache:            cache,
		responseHandlers: make(map[uuid.UUID]chan *models.AgentResponse),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case update := <-h.broadcast:
			h.broadcastUpdate(update)

		case <-h.ctx.Done():
			h.shutdown()
			return
		}
	}
}

// shutdown gracefully closes all connections
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all device connections
	for _, client := range h.devices {
		close(client.Send)
		client.Conn.Close()
	}

	// Close all admin sessions
	for session := range h.adminSessions {
		close(session.Send)
		session.Conn.Close()
	}
}

// Register enqueues a device client for registration on the hub loop.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// registerClient registers a new device client
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	h.devices[client.ID] = client
	h.deviceConns[client.ID] = client.Conn
	total := len(h.devices)
	h.mu.Unlock()

	if h.db != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			h.db.UpdateDeviceStatus(ctx, client.ID, "online", 0)
		}()
	}

	if h.cache != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			h.cache.SetDeviceOnline(ctx, &models.Device{
				ID:          client.ID,
				Status:      "online",
				LastCheckIn: time.Now(),
			})
		}()
	}

	// Must not send on h.broadcast while handling register — the hub loop is blocked here.
	// Presence is announced only after session crypto is ready (see notifySessionReady).
	log.Printf("Device registered: %s (total: %d); awaiting session key exchange", client.ID, total)

	if err := client.sendKeyOffer(); err != nil {
		log.Printf("Failed to send key_offer to device %s: %v", client.ID, err)
	}
}

// unregisterClient removes a device client
func (h *Hub) unregisterClient(client *Client) {
	var remaining int
	var shouldNotify bool

	h.mu.Lock()
	if _, ok := h.devices[client.ID]; ok {
		delete(h.devices, client.ID)
		delete(h.deviceConns, client.ID)
		close(client.Send)
		remaining = len(h.devices)
		shouldNotify = true
	}
	h.mu.Unlock()

	if !shouldNotify {
		return
	}

	if h.db != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			h.db.UpdateDeviceStatus(ctx, client.ID, "offline", 0)
		}()
	}

	if h.cache != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			h.cache.SetDeviceOffline(ctx, client.ID)
		}()
	}

	h.broadcastUpdate(&DeviceUpdate{
		Type:      "device_offline",
		DeviceID:  client.ID,
		Timestamp: time.Now(),
	})

	log.Printf("Device unregistered: %s (remaining: %d)", client.ID, remaining)
}

// IsDeviceOnline checks if a device is currently connected (socket registered).
func (h *Hub) IsDeviceOnline(deviceID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.devices[deviceID]
	return ok
}

// IsDeviceSessionReady reports whether the device has completed session key exchange.
func (h *Hub) IsDeviceSessionReady(deviceID uuid.UUID) bool {
	h.mu.RLock()
	client, ok := h.devices[deviceID]
	h.mu.RUnlock()
	if !ok || client == nil {
		return false
	}
	return client.hasSession()
}

// GetDeviceConnection returns the WebSocket connection for a device
func (h *Hub) GetDeviceConnection(deviceID uuid.UUID) *websocket.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.deviceConns[deviceID]
}

// SendCommandToDevice sends a command to a specific device
func (h *Hub) SendCommandToDevice(deviceID uuid.UUID, cmd *models.DeviceCommand) (*models.AgentResponse, error) {
	h.mu.RLock()
	client, ok := h.devices[deviceID]
	_, connOk := h.deviceConns[deviceID]
	h.mu.RUnlock()

	if !ok || !connOk {
		return nil, errors.New("device not connected")
	}

	// Create response channel
	responseChan := make(chan *models.AgentResponse, 1)
	h.mu.Lock()
	h.responseHandlers[cmd.TransactionID] = responseChan
	h.mu.Unlock()

	// Send command to device (encrypted once session is ready)
	cmdData, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	if !client.hasSession() {
		h.mu.Lock()
		delete(h.responseHandlers, cmd.TransactionID)
		h.mu.Unlock()
		return nil, errors.New("device session not established")
	}
	wire, err := client.sealApplicationJSON(cmdData)
	if err != nil {
		h.mu.Lock()
		delete(h.responseHandlers, cmd.TransactionID)
		h.mu.Unlock()
		return nil, fmt.Errorf("failed to encrypt command: %w", err)
	}

	select {
	case client.Send <- wire:
	default:
		h.mu.Lock()
		delete(h.responseHandlers, cmd.TransactionID)
		h.mu.Unlock()
		return nil, errors.New("device send buffer full")
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		h.mu.Lock()
		delete(h.responseHandlers, cmd.TransactionID)
		h.mu.Unlock()
		return response, nil
	case <-time.After(time.Duration(cmd.TimeoutSeconds) * time.Second):
		h.mu.Lock()
		delete(h.responseHandlers, cmd.TransactionID)
		h.mu.Unlock()
		return nil, errors.New("command timeout")
	}
}

// HandleDeviceResponse processes a response from a device
func (h *Hub) HandleDeviceResponse(response *models.AgentResponse) {
	h.mu.RLock()
	ch, ok := h.responseHandlers[response.TransactionID]
	h.mu.RUnlock()

	if ok {
		select {
		case ch <- response:
		default:
			log.Printf("Response channel full for transaction %s", response.TransactionID)
		}
	}
}

// broadcastUpdate sends an update to all matching admin sessions
func (h *Hub) broadcastUpdate(update *DeviceUpdate) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("Failed to marshal update: %v", err)
		return
	}

	for session := range h.adminSessions {
		// Check if session is watching this device
		if len(session.DeviceFilter) > 0 {
			found := false
			for _, id := range session.DeviceFilter {
				if id == update.DeviceID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		payload := data
		if session.EncryptOutbound != nil {
			enc, err := session.EncryptOutbound(data)
			if err != nil {
				log.Printf("Admin session %s CKX1 seal failed: %v", session.ID, err)
				continue
			}
			payload = enc
		}

		select {
		case session.Send <- payload:
		default:
			log.Printf("Admin session %s send buffer full", session.ID)
		}
	}
}

// broadcastToAdmins broadcasts a message to all admin sessions
func (h *Hub) broadcastToAdmins(update *DeviceUpdate) {
	h.broadcast <- update
}

// RegisterAdminSession registers an admin WebSocket session
func (h *Hub) RegisterAdminSession(session *AdminSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	session.Hub = h
	h.adminSessions[session] = true
}

// UnregisterAdminSession unregisters an admin WebSocket session
func (h *Hub) UnregisterAdminSession(session *AdminSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.adminSessions[session]; ok {
		delete(h.adminSessions, session)
		close(session.Send)
	}
}

// GetOnlineDeviceCount returns the number of online devices
func (h *Hub) GetOnlineDeviceCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.devices)
}

// GetOnlineDeviceIDs returns IDs of all online devices
func (h *Hub) GetOnlineDeviceIDs() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uuid.UUID, 0, len(h.devices))
	for id := range h.devices {
		ids = append(ids, id)
	}
	return ids
}

// OnlineDeviceSnapshots returns connected devices for merging into REST device lists.
func (h *Hub) OnlineDeviceSnapshots() []OnlineDeviceSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]OnlineDeviceSnapshot, 0, len(h.devices))
	for id, c := range h.devices {
		name := c.FriendlyName
		if name == "" {
			name = c.DeviceUUID
			if len(name) > 12 {
				name = "Device " + name[:8]
			}
		}
		out = append(out, OnlineDeviceSnapshot{
			ID:           id,
			DeviceUUID:   c.DeviceUUID,
			FriendlyName: name,
		})
	}
	return out
}

func (h *Hub) reassignClientID(c *Client, newID uuid.UUID) {
	if newID == c.ID {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.devices[c.ID]; ok {
		delete(h.devices, c.ID)
		delete(h.deviceConns, c.ID)
	}
	c.ID = newID
	h.devices[c.ID] = c
	h.deviceConns[c.ID] = c.Conn
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Device %s read error: %v", c.ID, err)
			}
			break
		}

		// Handle incoming message from device
		c.handleMessage(message)
	}
}

// handleMessage processes incoming device messages
func (c *Client) handleMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Invalid message from device %s: %v", c.ID, err)
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "key_exchange":
		if err := c.handleKeyExchange(msg); err != nil {
			log.Printf("key_exchange failed for device %s: %v", c.ID, err)
			_ = c.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "key_exchange failed"))
			_ = c.Conn.Close()
		}
		return
	case "enc":
		if !c.hasSession() {
			log.Printf("Encrypted message before session from device %s", c.ID)
			return
		}
		plain, err := c.openEncryptedEnvelope(msg)
		if err != nil {
			log.Printf("Failed to decrypt message from device %s: %v", c.ID, err)
			return
		}
		c.handleApplicationMessage(plain)
		return
	default:
		if c.hasSession() {
			log.Printf("Rejecting plaintext %s after session from device %s", msgType, c.ID)
			return
		}
		log.Printf("Rejecting plaintext %s before session from device %s", msgType, c.ID)
	}
}

func (c *Client) handleApplicationMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Invalid application message from device %s: %v", c.ID, err)
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "response":
		response := &models.AgentResponse{
			TransactionID: uuid.MustParse(msg["transaction_id"].(string)),
			DeviceID:      c.ID,
			CommandType:   models.CommandType(msg["command_type"].(string)),
			Status:        models.ResponseStatus(msg["status"].(string)),
			ReceivedAt:    time.Now(),
		}
		if data, ok := msg["data"].(string); ok {
			if decoded, err := base64.StdEncoding.DecodeString(data); err == nil {
				response.Data = decoded
			} else {
				response.Data = []byte(data)
			}
		}
		if errMsg, ok := msg["error"].(string); ok {
			response.ErrorMessage = errMsg
		}
		c.Hub.HandleDeviceResponse(response)

	case "heartbeat":
		batteryLevel := 0
		if bl, ok := msg["battery_level"].(float64); ok {
			batteryLevel = int(bl)
		}
		if c.Hub.cache != nil {
			go c.Hub.cache.UpdateDeviceHeartbeat(context.Background(), c.ID, batteryLevel)
		}
		if c.Hub.db != nil {
			go c.Hub.db.UpdateDeviceStatus(context.Background(), c.ID, "online", batteryLevel)
		}

	case "status_update":
		c.Hub.broadcastToAdmins(&DeviceUpdate{
			Type:      "status_update",
			DeviceID:  c.ID,
			Timestamp: time.Now(),
			Data:      msg,
		})

	case "enrollment":
		log.Printf("Device %s sent enrollment info: %v", c.ID, msg)
		c.syncEnrollment(msg)

	default:
		log.Printf("Unknown message type from device %s: %s", c.ID, msgType)
	}
}

func stringField(msg map[string]interface{}, key string) string {
	v, _ := msg[key].(string)
	return v
}

func (c *Client) syncEnrollment(msg map[string]interface{}) {
	if c.Hub == nil || c.Hub.db == nil {
		return
	}

	deviceUUID := stringField(msg, "device_uuid")
	if deviceUUID == "" {
		deviceUUID = c.DeviceUUID
	}
	friendlyName := stringField(msg, "friendly_name")
	if friendlyName == "" {
		friendlyName = "Android device"
	}
	c.FriendlyName = friendlyName

	osVersion := stringField(msg, "os_version")
	hardwareModel := stringField(msg, "hardware_model")
	if hardwareModel == "" {
		hardwareModel = stringField(msg, "manufacturer")
	}
	certHash := stringField(msg, "certificate_hash")
	x25519Pub := stringField(msg, "x25519_public_key")
	ed25519Pub := stringField(msg, "ed25519_public_key")
	keyFP := stringField(msg, "key_fingerprint")
	if keyFP == "" {
		keyFP = certHash
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var canonicalID uuid.UUID
		found := false

		if existing, err := c.Hub.db.GetDeviceByUUID(ctx, deviceUUID); err == nil && existing != nil {
			canonicalID = existing.ID
			found = true
			_ = c.Hub.db.UpdateDeviceStatus(ctx, existing.ID, "online", 0)
			_ = c.Hub.db.UpdateDeviceCKX1Keys(ctx, existing.ID, x25519Pub, ed25519Pub, keyFP)
			_ = c.Hub.db.UpdateDeviceMetadata(ctx, existing.ID, friendlyName, osVersion, hardwareModel)
		} else if existing, err := c.Hub.db.GetDeviceByID(ctx, c.ID); err == nil && existing != nil {
			canonicalID = existing.ID
			found = true
			_ = c.Hub.db.UpdateDeviceStatus(ctx, existing.ID, "online", 0)
			_ = c.Hub.db.UpdateDeviceCKX1Keys(ctx, existing.ID, x25519Pub, ed25519Pub, keyFP)
			_ = c.Hub.db.UpdateDeviceMetadata(ctx, existing.ID, friendlyName, osVersion, hardwareModel)
		}

		if !found {
			device := &models.Device{
				ID:               uuid.New(),
				FriendlyName:     friendlyName,
				Owner:            "auto-enrolled",
				OSVersion:        osVersion,
				HardwareModel:    hardwareModel,
				DeviceUUID:       deviceUUID,
				CertificateHash:  keyFP,
				X25519PublicKey:  x25519Pub,
				Ed25519PublicKey: ed25519Pub,
				KeyFingerprint:   keyFP,
				KeyVersion:       1,
				KeyCreatedAt:     time.Now(),
				Status:           "online",
				EnrolledAt:       time.Now(),
			}
			if err := c.Hub.db.CreateDevice(ctx, device); err != nil {
				log.Printf("Auto-enroll failed for %s: %v", deviceUUID, err)
				return
			}
			canonicalID = device.ID
		}

		c.Hub.reassignClientID(c, canonicalID)
	}()
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				_ = w.Close()
				return
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// AdminWritePump pumps messages for admin sessions
func (s *AdminSession) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		s.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-s.Send:
			s.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				s.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := s.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			s.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// AdminReadPump reads messages from admin WebSocket connection
func (s *AdminSession) ReadPump() {
	defer func() {
		s.Hub.UnregisterAdminSession(s)
		s.Conn.Close()
	}()

	s.Conn.SetReadLimit(maxMessageSize)
	s.Conn.SetReadDeadline(time.Now().Add(pongWait))
	s.Conn.SetPongHandler(func(string) error {
		s.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := s.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Admin session %s read error: %v", s.ID, err)
			}
			break
		}

		plain := message
		if s.DecryptInbound != nil {
			opened, err := s.DecryptInbound(message)
			if err != nil {
				log.Printf("Admin session %s CKX1 open failed: %v", s.ID, err)
				continue
			}
			plain = opened
		}

		// Handle admin messages (e.g., subscribe to device updates)
		var msg map[string]interface{}
		if err := json.Unmarshal(plain, &msg); err != nil {
			continue
		}

		if msgType, ok := msg["type"].(string); ok && msgType == "subscribe" {
			// Handle subscription to specific devices
			if deviceIDs, ok := msg["device_ids"].([]interface{}); ok {
				for _, id := range deviceIDs {
					if idStr, ok := id.(string); ok {
						if parsed, err := uuid.Parse(idStr); err == nil {
							s.DeviceFilter = append(s.DeviceFilter, parsed)
						}
					}
				}
			}
		}
	}
}

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

// UpgradeDeviceConnection upgrades an HTTP connection to WebSocket for device
func UpgradeDeviceConnection(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// UpgradeAdminConnection upgrades an HTTP connection to WebSocket for admin
func UpgradeAdminConnection(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
