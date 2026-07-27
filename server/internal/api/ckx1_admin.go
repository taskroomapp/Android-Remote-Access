package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
	"github.com/enterprise/android-remote-access/server/internal/security"
	"github.com/google/uuid"
)

type pendingAdminOffer struct {
	SessionID string
	Nonce     string
	AdminID   uuid.UUID
	Created   time.Time
}

func (s *Server) handleAdminCKX1Offer(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	id := s.hub.GetCKX1Identity()
	if id == nil {
		s.writeError(w, http.StatusServiceUnavailable, "CKX1_UNAVAILABLE", "CKX1 identity not configured")
		return
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		s.writeError(w, http.StatusInternalServerError, "RNG_ERROR", "Failed to generate nonce")
		return
	}
	sessionID := uuid.New().String()
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)
	s.adminOfferMu.Lock()
	s.adminOffers[admin.ID.String()] = pendingAdminOffer{
		SessionID: sessionID,
		Nonce:     nonceB64,
		AdminID:   admin.ID,
		Created:   time.Now(),
	}
	s.adminOfferMu.Unlock()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"type":                      "key_offer",
		"protocol":                  cryptokit.CKX1Protocol,
		"version":                   cryptokit.CKX1Version,
		"session_id":                sessionID,
		"server_x25519_public_key":  id.X25519B64,
		"server_ed25519_public_key": id.Ed25519B64,
		"server_fingerprint":        id.Fingerprint,
		"server_nonce":              nonceB64,
		"algorithm":                 cryptokit.CKX1Algorithm,
		"channel":                   "admin",
	})
}

func (s *Server) handleAdminCKX1Exchange(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	id := s.hub.GetCKX1Identity()
	if id == nil || s.adminCKX1 == nil {
		s.writeError(w, http.StatusServiceUnavailable, "CKX1_UNAVAILABLE", "CKX1 not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid body")
		return
	}
	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON")
		return
	}

	s.adminOfferMu.Lock()
	offer, ok := s.adminOffers[admin.ID.String()]
	delete(s.adminOffers, admin.ID.String())
	s.adminOfferMu.Unlock()
	if !ok || time.Since(offer.Created) > 2*time.Minute {
		s.writeError(w, http.StatusBadRequest, "OFFER_EXPIRED", "No valid key_offer; request a new offer")
		return
	}

	sessionID, _ := msg["session_id"].(string)
	deviceID, _ := msg["device_id"].(string) // admin uses admin UUID as device_id in transcript
	clientX, _ := msg["device_x25519_public_key"].(string)
	clientEd, _ := msg["device_ed25519_public_key"].(string)
	clientNonce, _ := msg["device_nonce"].(string)
	sigB64, _ := msg["signature"].(string)

	if sessionID != offer.SessionID {
		s.writeError(w, http.StatusBadRequest, "SESSION_MISMATCH", "session_id mismatch")
		return
	}
	if deviceID == "" {
		deviceID = admin.ID.String()
	}
	if deviceID != admin.ID.String() {
		s.writeError(w, http.StatusBadRequest, "DEVICE_MISMATCH", "device_id must equal admin id")
		return
	}

	transcript := cryptokit.HandshakeTranscript(
		sessionID, deviceID,
		id.X25519B64, id.Ed25519B64,
		clientX, clientEd,
		offer.Nonce, clientNonce,
	)
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "BAD_SIGNATURE", "Invalid signature encoding")
		return
	}
	pub, err := cryptokit.ParseEd25519PublicRawBase64(clientEd)
	if err != nil || !cryptokit.VerifyEd25519(pub, transcript, sig) {
		s.writeError(w, http.StatusUnauthorized, "BAD_SIGNATURE", "Handshake signature invalid")
		return
	}
	peerX, err := cryptokit.ParseX25519PublicRawBase64(clientX)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "BAD_KEY", "Invalid X25519 public key")
		return
	}
	shared, err := cryptokit.X25519SharedSecret(id.X25519, peerX)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "ECDH_FAILED", "Key agreement failed")
		return
	}
	th := cryptokit.SHA256Sum(transcript)
	c2s, s2c, err := cryptokit.DeriveDirectionalKeys(shared, th)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "KDF_FAILED", "Key derivation failed")
		return
	}

	sessID := uuid.New().String()
	sess := &security.AdminCKX1Session{
		ID:        sessID,
		AdminID:   admin.ID,
		SessionID: sessionID,
		C2S:       c2s,
		S2C:       s2c,
		CreatedAt: time.Now(),
	}
	s.adminCKX1.Put(sess)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"type":           "session_ready",
		"protocol":       cryptokit.CKX1Protocol,
		"version":        cryptokit.CKX1Version,
		"session_id":     sessionID,
		"ckx1_session":   sessID,
		"algorithm":      cryptokit.CKX1Algorithm,
		"channel":        "admin",
	})
}
