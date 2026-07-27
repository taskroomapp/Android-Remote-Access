package cryptokit

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

// GenerateEd25519 returns a new Ed25519 key pair (public, private).
func GenerateEd25519() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// Ed25519PublicRawBase64 encodes the 32-byte public key.
func Ed25519PublicRawBase64(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString([]byte(pub))
}

// ParseEd25519PublicRawBase64 parses a raw 32-byte Ed25519 public key.
func ParseEd25519PublicRawBase64(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("Ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

// SignEd25519 signs message with the private key.
func SignEd25519(priv ed25519.PrivateKey, message []byte) []byte {
	return ed25519.Sign(priv, message)
}

// VerifyEd25519 verifies a signature.
func VerifyEd25519(pub ed25519.PublicKey, message, sig []byte) bool {
	return ed25519.Verify(pub, message, sig)
}

// LoadOrGenerateEd25519Key loads a 64-byte seed private key or creates one.
// File stores the 64-byte private key (seed||pub) in raw form.
func LoadOrGenerateEd25519Key(path string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if path == "" {
		return nil, nil, errors.New("Ed25519 key path is empty")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		raw := decodeKeyFile(data)
		if len(raw) == ed25519.SeedSize {
			priv := ed25519.NewKeyFromSeed(raw)
			return priv.Public().(ed25519.PublicKey), priv, nil
		}
		if len(raw) == ed25519.PrivateKeySize {
			priv := ed25519.PrivateKey(raw)
			return priv.Public().(ed25519.PublicKey), priv, nil
		}
		return nil, nil, fmt.Errorf("Ed25519 private key invalid length %d", len(raw))
	}
	if !os.IsNotExist(err) {
		return nil, nil, err
	}
	pub, priv, err := GenerateEd25519()
	if err != nil {
		return nil, nil, err
	}
	if err := writeKeyFile(path, priv.Seed()); err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}
