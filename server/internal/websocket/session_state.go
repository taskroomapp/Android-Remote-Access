package websocket

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"sync"

	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
)

// ServerIdentity holds long-term CKX1 server keys.
type ServerIdentity struct {
	X25519   *ecdh.PrivateKey
	Ed25519  ed25519.PrivateKey
	EdPub    ed25519.PublicKey
	X25519B64 string
	Ed25519B64 string
	Fingerprint string
}

func NewServerIdentity(x25519 *ecdh.PrivateKey, edPub ed25519.PublicKey, edPriv ed25519.PrivateKey) *ServerIdentity {
	xB64 := cryptokit.X25519PublicRawBase64(x25519)
	eB64 := cryptokit.Ed25519PublicRawBase64(edPub)
	fp := cryptokit.FingerprintSHA256Hex(
		cryptokit.X25519PublicRaw(x25519),
		[]byte(edPub),
	)
	return &ServerIdentity{
		X25519:      x25519,
		Ed25519:     edPriv,
		EdPub:       edPub,
		X25519B64:   xB64,
		Ed25519B64:  eB64,
		Fingerprint: fp,
	}
}

// GetCKX1Identity returns the configured server identity (may be nil).
func (h *Hub) GetCKX1Identity() *ServerIdentity {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.identity
}

// DeviceSession is the hub abstraction for sealing/opening application JSON.
type DeviceSession interface {
	Ready() bool
	SealJSON(message []byte, txn string) ([]byte, error)
	OpenJSON(frame map[string]interface{}) ([]byte, error)
	Clear()
}

type ckx1DeviceSession struct {
	mu           sync.RWMutex
	sessionID    string
	deviceID     string
	c2s          []byte
	s2c          []byte
	sendSeq      uint64
	recv         *ReplayGuard
}

func newCKX1DeviceSession(sessionID, deviceID string, c2s, s2c []byte) *ckx1DeviceSession {
	return &ckx1DeviceSession{
		sessionID: sessionID,
		deviceID:  deviceID,
		c2s:       append([]byte(nil), c2s...),
		s2c:       append([]byte(nil), s2c...),
		recv:      NewReplayGuard(),
	}
}

func (s *ckx1DeviceSession) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.c2s) == cryptokit.CKX1KeySize && len(s.s2c) == cryptokit.CKX1KeySize && s.sessionID != ""
}

func (s *ckx1DeviceSession) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.c2s {
		s.c2s[i] = 0
	}
	for i := range s.s2c {
		s.s2c[i] = 0
	}
	s.c2s = nil
	s.s2c = nil
	s.sessionID = ""
	s.sendSeq = 0
	if s.recv != nil {
		s.recv.Reset()
	}
}
