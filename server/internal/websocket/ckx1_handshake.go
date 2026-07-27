package websocket

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func randomNonceB64() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func (c *Client) sendKeyOffer() error {
	id := c.Hub.identity
	if id == nil {
		return errors.New("CKX1 server identity not configured")
	}
	nonce, err := randomNonceB64()
	if err != nil {
		return err
	}
	sessionID := uuid.New().String()
	c.sessionMu.Lock()
	c.offerSessionID = sessionID
	c.offerNonce = nonce
	c.offerFingerprint = id.Fingerprint
	c.sessionMu.Unlock()

	return c.enqueueJSON(map[string]interface{}{
		"type":                      "key_offer",
		"protocol":                  cryptokit.CKX1Protocol,
		"version":                   cryptokit.CKX1Version,
		"session_id":                sessionID,
		"server_x25519_public_key":  id.X25519B64,
		"server_ed25519_public_key": id.Ed25519B64,
		"server_fingerprint":        id.Fingerprint,
		"server_nonce":              nonce,
		"algorithm":                 cryptokit.CKX1Algorithm,
	})
}

func (c *Client) sendSessionReady() error {
	c.sessionMu.RLock()
	sid := c.offerSessionID
	c.sessionMu.RUnlock()
	return c.enqueueJSON(map[string]interface{}{
		"type":       "session_ready",
		"protocol":   cryptokit.CKX1Protocol,
		"version":    cryptokit.CKX1Version,
		"session_id": sid,
		"algorithm":  cryptokit.CKX1Algorithm,
	})
}

func (c *Client) handleKeyExchange(msg map[string]interface{}) error {
	if c.hasSession() {
		return errors.New("session already established")
	}
	id := c.Hub.identity
	if id == nil {
		return errors.New("CKX1 server identity not configured")
	}

	proto, _ := msg["protocol"].(string)
	if proto != cryptokit.CKX1Protocol {
		return errors.New("unsupported protocol")
	}
	verF, _ := msg["version"].(float64)
	if int(verF) != cryptokit.CKX1Version {
		return errors.New("unsupported version")
	}

	sessionID, _ := msg["session_id"].(string)
	deviceID, _ := msg["device_id"].(string)
	deviceX25519B64, _ := msg["device_x25519_public_key"].(string)
	deviceEd25519B64, _ := msg["device_ed25519_public_key"].(string)
	deviceNonce, _ := msg["device_nonce"].(string)
	sigB64, _ := msg["signature"].(string)

	c.sessionMu.RLock()
	offerSID := c.offerSessionID
	offerNonce := c.offerNonce
	c.sessionMu.RUnlock()

	if sessionID == "" || sessionID != offerSID {
		return errors.New("session_id mismatch")
	}
	if deviceID == "" {
		deviceID = c.DeviceUUID
	}
	if deviceID != c.DeviceUUID {
		return errors.New("device_id mismatch")
	}
	if deviceX25519B64 == "" || deviceEd25519B64 == "" || deviceNonce == "" || sigB64 == "" {
		return errors.New("incomplete key_exchange fields")
	}

	// Verify enrolled public keys when present.
	if c.Hub.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		dev, err := c.Hub.db.GetDeviceByUUID(ctx, deviceID)
		cancel()
		if err == nil && dev != nil {
			if dev.X25519PublicKey != "" && dev.X25519PublicKey != deviceX25519B64 {
				return errors.New("device X25519 public key mismatch")
			}
			if dev.Ed25519PublicKey != "" && dev.Ed25519PublicKey != deviceEd25519B64 {
				return errors.New("device Ed25519 public key mismatch")
			}
		}
	}

	transcript := cryptokit.HandshakeTranscript(
		sessionID, deviceID,
		id.X25519B64, id.Ed25519B64,
		deviceX25519B64, deviceEd25519B64,
		offerNonce, deviceNonce,
	)
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	pub, err := cryptokit.ParseEd25519PublicRawBase64(deviceEd25519B64)
	if err != nil {
		return fmt.Errorf("device Ed25519 public key: %w", err)
	}
	if !cryptokit.VerifyEd25519(pub, transcript, sig) {
		return errors.New("handshake signature invalid")
	}

	peerX, err := cryptokit.ParseX25519PublicRawBase64(deviceX25519B64)
	if err != nil {
		return fmt.Errorf("device X25519 public key: %w", err)
	}
	shared, err := cryptokit.X25519SharedSecret(id.X25519, peerX)
	if err != nil {
		return fmt.Errorf("X25519 agreement failed: %w", err)
	}
	th := cryptokit.SHA256Sum(transcript)
	c2s, s2c, err := cryptokit.DeriveDirectionalKeys(shared, th)
	if err != nil {
		return err
	}

	c.sessionMu.Lock()
	c.ckx1 = newCKX1DeviceSession(sessionID, deviceID, c2s, s2c)
	c.sessionMu.Unlock()

	if err := c.sendSessionReady(); err != nil {
		return err
	}
	c.Hub.notifySessionReady(c)
	return nil
}

func (c *Client) hasSession() bool {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.ckx1 != nil && c.ckx1.Ready()
}

func (c *Client) clearSession() {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.ckx1 != nil {
		c.ckx1.Clear()
		c.ckx1 = nil
	}
	c.offerSessionID = ""
	c.offerNonce = ""
	c.offerFingerprint = ""
}

func (c *Client) sealApplicationJSON(plain []byte) ([]byte, error) {
	c.sessionMu.RLock()
	sess := c.ckx1
	c.sessionMu.RUnlock()
	if sess == nil {
		return nil, errors.New("session not established")
	}
	return sess.SealJSON(plain, "")
}

func (c *Client) openEncryptedEnvelope(outer map[string]interface{}) ([]byte, error) {
	c.sessionMu.RLock()
	sess := c.ckx1
	c.sessionMu.RUnlock()
	if sess == nil {
		return nil, errors.New("session not established")
	}
	return sess.OpenJSON(outer)
}

func (h *Hub) notifySessionReady(c *Client) {
	h.broadcastUpdate(&DeviceUpdate{
		Type:      "device_online",
		DeviceID:  c.ID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"device_id":     c.ID.String(),
			"session_ready": true,
			"protocol":      cryptokit.CKX1Protocol,
		},
	})
	h.broadcastUpdate(&DeviceUpdate{
		Type:      "device_session_ready",
		DeviceID:  c.ID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"device_id": c.ID.String(), "protocol": cryptokit.CKX1Protocol},
	})
	log.Printf("Device CKX1 session ready: %s", c.ID)
}

// InvalidateAllDeviceSessions closes every device socket after identity key rotation.
func (h *Hub) InvalidateAllDeviceSessions() {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.devices))
	for _, c := range h.devices {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		c.clearSession()
		_ = c.Conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseServiceRestart, "session key rotated"))
		_ = c.Conn.Close()
	}
	log.Printf("Invalidated %d device sessions after CKX1 key rotation", len(clients))
}

func (c *Client) enqueueJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	select {
	case c.Send <- data:
		return nil
	default:
		return errors.New("device send buffer full")
	}
}
