package websocket

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
)

func (s *ckx1DeviceSession) nextSendSeq() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendSeq++
	return s.sendSeq
}

func (s *ckx1DeviceSession) SealJSON(message []byte, txn string) ([]byte, error) {
	if !s.Ready() {
		return nil, errors.New("session not established")
	}
	if txn == "" {
		txn = extractTxnID(message)
		if txn == "" {
			txn = "-"
		}
	}
	seq := s.nextSendSeq()
	s.mu.RLock()
	sid := s.sessionID
	did := s.deviceID
	key := append([]byte(nil), s.s2c...)
	s.mu.RUnlock()

	aad := cryptokit.FrameAAD(sid, did, cryptokit.DirServerToDevice, seq, txn)
	nonce := make([]byte, cryptokit.CKX1NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct, err := cryptokit.Ckx1AEADEncrypt(key, nonce, message, aad)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]interface{}{
		"type":       "enc",
		"protocol":   cryptokit.CKX1Protocol,
		"version":    cryptokit.CKX1Version,
		"session_id": sid,
		"seq":        seq,
		"dir":        cryptokit.DirServerToDevice,
		"txn":        txn,
		"nonce":      base64.StdEncoding.EncodeToString(nonce),
		"ciphertext": base64.StdEncoding.EncodeToString(ct),
	})
}

func (s *ckx1DeviceSession) OpenJSON(outer map[string]interface{}) ([]byte, error) {
	if !s.Ready() {
		return nil, errors.New("session not established")
	}
	proto, _ := outer["protocol"].(string)
	if proto != cryptokit.CKX1Protocol {
		return nil, errors.New("bad protocol")
	}
	verF, _ := outer["version"].(float64)
	if int(verF) != cryptokit.CKX1Version {
		return nil, errors.New("bad version")
	}
	sid, _ := outer["session_id"].(string)
	s.mu.RLock()
	expectSID := s.sessionID
	did := s.deviceID
	key := append([]byte(nil), s.c2s...)
	s.mu.RUnlock()
	if sid != expectSID {
		return nil, errors.New("session_id mismatch")
	}
	dir, _ := outer["dir"].(string)
	if dir != cryptokit.DirDeviceToServer {
		return nil, fmt.Errorf("incorrect direction: %q", dir)
	}
	seqF, ok := outer["seq"].(float64)
	if !ok || seqF < 1 {
		return nil, errors.New("missing or invalid seq")
	}
	seq := uint64(seqF)
	s.mu.Lock()
	err := s.recv.Accept(seq)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	txn, _ := outer["txn"].(string)
	if txn == "" {
		txn = "-"
	}
	nonceB64, _ := outer["nonce"].(string)
	ctB64, _ := outer["ciphertext"].(string)
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(ctB64)
	if err != nil {
		return nil, err
	}
	aad := cryptokit.FrameAAD(sid, did, dir, seq, txn)
	return cryptokit.Ckx1AEADDecrypt(key, nonce, ct, aad)
}

func extractTxnID(plain []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(plain, &m); err != nil {
		return ""
	}
	if v, ok := m["transaction_id"].(string); ok {
		return v
	}
	return ""
}
