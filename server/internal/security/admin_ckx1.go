package security

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
	"github.com/google/uuid"
)

// AdminCKX1Session holds directional keys for an authenticated operator channel.
type AdminCKX1Session struct {
	mu        sync.Mutex
	ID        string
	AdminID   uuid.UUID
	SessionID string
	C2S       []byte
	S2C       []byte
	SendSeq   uint64
	RecvLast  uint64
	seen      map[uint64]struct{}
	CreatedAt time.Time
}

// AdminCKX1Store is an in-process session map.
type AdminCKX1Store struct {
	mu   sync.RWMutex
	byID map[string]*AdminCKX1Session
}

func NewAdminCKX1Store() *AdminCKX1Store {
	return &AdminCKX1Store{byID: make(map[string]*AdminCKX1Session)}
}

func (s *AdminCKX1Store) Put(sess *AdminCKX1Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[sess.ID] = sess
}

func (s *AdminCKX1Store) Get(id string) *AdminCKX1Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[id]
}

func (s *AdminCKX1Store) DeleteForAdmin(adminID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.byID {
		if sess.AdminID == adminID {
			delete(s.byID, id)
		}
	}
}

// Ready reports whether directional session keys are present.
func (s *AdminCKX1Session) Ready() bool {
	return s != nil && len(s.C2S) == cryptokit.CKX1KeySize && len(s.S2C) == cryptokit.CKX1KeySize
}

// SealAdmin encrypts server→admin JSON (uses server-to-client key / DirServerToDevice).
func (s *AdminCKX1Session) SealAdmin(plain []byte) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.S2C) != cryptokit.CKX1KeySize {
		return nil, errors.New("session not ready")
	}
	s.SendSeq++
	seq := s.SendSeq
	aad := cryptokit.FrameAAD(s.SessionID, s.AdminID.String(), cryptokit.DirServerToDevice, seq, "-")
	nonce := make([]byte, cryptokit.CKX1NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct, err := cryptokit.Ckx1AEADEncrypt(s.S2C, nonce, plain, aad)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"type":       "enc",
		"protocol":   cryptokit.CKX1Protocol,
		"version":    cryptokit.CKX1Version,
		"session_id": s.SessionID,
		"seq":        seq,
		"dir":        cryptokit.DirServerToDevice,
		"txn":        "-",
		"nonce":      base64.StdEncoding.EncodeToString(nonce),
		"ciphertext": base64.StdEncoding.EncodeToString(ct),
	}, nil
}

// OpenAdmin decrypts admin→server JSON.
func (s *AdminCKX1Session) OpenAdmin(frame map[string]interface{}) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.C2S) != cryptokit.CKX1KeySize {
		return nil, errors.New("session not ready")
	}
	proto, _ := frame["protocol"].(string)
	if proto != cryptokit.CKX1Protocol {
		return nil, errors.New("bad protocol")
	}
	sid, _ := frame["session_id"].(string)
	if sid != s.SessionID {
		return nil, errors.New("session_id mismatch")
	}
	dir, _ := frame["dir"].(string)
	if dir != cryptokit.DirDeviceToServer {
		return nil, fmt.Errorf("incorrect direction %q", dir)
	}
	seq, err := frameSeq(frame["seq"])
	if err != nil {
		return nil, err
	}
	if seq < 1 {
		return nil, errors.New("bad seq")
	}
	if s.seen == nil {
		s.seen = make(map[uint64]struct{})
	}
	if _, dup := s.seen[seq]; dup {
		return nil, errors.New("duplicate sequence")
	}
	// Allow a small out-of-order window for concurrent HTTP arrivals.
	const maxSkew = uint64(64)
	if seq+maxSkew < s.RecvLast {
		return nil, errors.New("replayed or older sequence")
	}
	s.seen[seq] = struct{}{}
	if seq > s.RecvLast {
		s.RecvLast = seq
	}
	if len(s.seen) > 4096 {
		s.seen = map[uint64]struct{}{seq: {}}
	}
	nonce, err := base64.StdEncoding.DecodeString(fmt.Sprint(frame["nonce"]))
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(fmt.Sprint(frame["ciphertext"]))
	if err != nil {
		return nil, err
	}
	txn, _ := frame["txn"].(string)
	if txn == "" {
		txn = "-"
	}
	aad := cryptokit.FrameAAD(s.SessionID, s.AdminID.String(), dir, seq, txn)
	return cryptokit.Ckx1AEADDecrypt(s.C2S, nonce, ct, aad)
}

func frameSeq(v interface{}) (uint64, error) {
	switch n := v.(type) {
	case float64:
		if n < 1 || n != float64(uint64(n)) {
			return 0, errors.New("bad seq")
		}
		return uint64(n), nil
	case json.Number:
		u, err := strconv.ParseUint(string(n), 10, 64)
		if err != nil || u < 1 {
			return 0, errors.New("bad seq")
		}
		return u, nil
	case int:
		if n < 1 {
			return 0, errors.New("bad seq")
		}
		return uint64(n), nil
	case int64:
		if n < 1 {
			return 0, errors.New("bad seq")
		}
		return uint64(n), nil
	case uint64:
		if n < 1 {
			return 0, errors.New("bad seq")
		}
		return n, nil
	default:
		return 0, errors.New("bad seq")
	}
}
