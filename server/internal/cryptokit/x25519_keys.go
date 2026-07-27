package cryptokit

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateX25519 returns a new X25519 private key.
func GenerateX25519() (*ecdh.PrivateKey, error) {
	return ecdh.X25519().GenerateKey(rand.Reader)
}

// X25519PublicRaw returns the 32-byte public key.
func X25519PublicRaw(priv *ecdh.PrivateKey) []byte {
	return priv.PublicKey().Bytes()
}

// X25519PublicRawBase64 encodes the raw public key.
func X25519PublicRawBase64(priv *ecdh.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(X25519PublicRaw(priv))
}

// ParseX25519PublicRawBase64 parses a raw 32-byte public key from base64.
func ParseX25519PublicRawBase64(b64 string) (*ecdh.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(raw) != CKX1PublicKeySize {
		return nil, fmt.Errorf("X25519 public key must be 32 bytes, got %d", len(raw))
	}
	return ecdh.X25519().NewPublicKey(raw)
}

// X25519SharedSecret computes the ECDH shared secret.
func X25519SharedSecret(priv *ecdh.PrivateKey, peer *ecdh.PublicKey) ([]byte, error) {
	secret, err := priv.ECDH(peer)
	if err != nil {
		return nil, err
	}
	if len(secret) != CKX1KeySize {
		return nil, errors.New("invalid X25519 shared secret length")
	}
	allZero := true
	for _, b := range secret {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil, errors.New("invalid all-zero X25519 shared secret")
	}
	return secret, nil
}

// LoadOrGenerateX25519Key loads a raw 32-byte seed/private key file, or creates one.
func LoadOrGenerateX25519Key(path string) (*ecdh.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("X25519 key path is empty")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		raw := decodeKeyFile(data)
		if len(raw) != CKX1KeySize {
			return nil, fmt.Errorf("X25519 private key must be 32 bytes, got %d", len(raw))
		}
		return ecdh.X25519().NewPrivateKey(raw)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	priv, err := GenerateX25519()
	if err != nil {
		return nil, err
	}
	if err := writeKeyFile(path, priv.Bytes()); err != nil {
		return nil, err
	}
	return priv, nil
}

func writeKeyFile(path string, raw []byte) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		_ = os.Chmod(dir, 0o700)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func decodeKeyFile(data []byte) []byte {
	// Accept raw 32 bytes or standard base64 of 32 bytes.
	if len(data) == CKX1KeySize {
		return data
	}
	trimmed := make([]byte, 0, len(data))
	for _, b := range data {
		if b != '\n' && b != '\r' && b != ' ' {
			trimmed = append(trimmed, b)
		}
	}
	if dec, err := base64.StdEncoding.DecodeString(string(trimmed)); err == nil && len(dec) == CKX1KeySize {
		return dec
	}
	return data
}
