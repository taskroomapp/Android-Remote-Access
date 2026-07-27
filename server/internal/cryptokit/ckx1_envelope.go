package cryptokit

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// AT1Prefix marks at-rest ChaCha20-Poly1305 envelopes (distinct from session frames).
const AT1Prefix = "AT1:"

// AT1Seal encrypts plaintext with a server master key and record-bound AAD.
// Wire format: AT1: || base64( nonce(12) || ciphertext||tag )
func AT1Seal(key, plaintext, aad []byte) (string, error) {
	if len(key) != CKX1KeySize {
		return "", fmt.Errorf("at-rest key must be 32 bytes")
	}
	nonce := make([]byte, CKX1NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct, err := Ckx1AEADEncrypt(key, nonce, plaintext, aad)
	if err != nil {
		return "", err
	}
	out := append(append([]byte{}, nonce...), ct...)
	return AT1Prefix + base64.StdEncoding.EncodeToString(out), nil
}

// AT1Open decrypts an AT1: envelope.
func AT1Open(key []byte, sealed string, aad []byte) ([]byte, error) {
	if len(sealed) < len(AT1Prefix) || sealed[:len(AT1Prefix)] != AT1Prefix {
		return nil, ErrInvalidEnvelope
	}
	raw, err := base64.StdEncoding.DecodeString(sealed[len(AT1Prefix):])
	if err != nil {
		return nil, err
	}
	if len(raw) < CKX1NonceSize+CKX1TagSize {
		return nil, ErrInvalidEnvelope
	}
	nonce, ct := raw[:CKX1NonceSize], raw[CKX1NonceSize:]
	return Ckx1AEADDecrypt(key, nonce, ct, aad)
}

// AT1SealBytes returns raw nonce||ciphertext (no prefix) for BYTEA columns.
func AT1SealBytes(key, plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, CKX1NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct, err := Ckx1AEADEncrypt(key, nonce, plaintext, aad)
	if err != nil {
		return nil, err
	}
	return append(nonce, ct...), nil
}

// AT1OpenBytes opens raw nonce||ciphertext envelopes.
func AT1OpenBytes(key, envelope, aad []byte) ([]byte, error) {
	if len(envelope) < CKX1NonceSize+CKX1TagSize {
		return nil, ErrInvalidEnvelope
	}
	nonce, ct := envelope[:CKX1NonceSize], envelope[CKX1NonceSize:]
	return Ckx1AEADDecrypt(key, nonce, ct, aad)
}

// FrameAAD builds length-prefixed CKX1 frame AAD.
func FrameAAD(sessionID, deviceID, direction string, seq uint64, txn string) []byte {
	if txn == "" {
		txn = "-"
	}
	return CanonicalEncodeStrings(
		CKX1FrameLabel,
		sessionID,
		deviceID,
		direction,
		fmt.Sprintf("%d", seq),
		txn,
	)
}

// HandshakeTranscript builds the signed CKX1 handshake transcript.
func HandshakeTranscript(
	sessionID, deviceID,
	serverX25519B64, serverEd25519B64,
	deviceX25519B64, deviceEd25519B64,
	serverNonce, deviceNonce string,
) []byte {
	return CanonicalEncodeStrings(
		CKX1HandshakeLabel,
		sessionID,
		deviceID,
		serverX25519B64,
		serverEd25519B64,
		deviceX25519B64,
		deviceEd25519B64,
		serverNonce,
		deviceNonce,
		CKX1Algorithm,
	)
}
