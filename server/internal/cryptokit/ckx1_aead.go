package cryptokit

import (
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

// Ckx1AEADEncrypt seals plaintext with ChaCha20-Poly1305 and AAD. Nonce must be 12 bytes.
func Ckx1AEADEncrypt(key, nonce, plaintext, aad []byte) ([]byte, error) {
	if len(key) != CKX1KeySize {
		return nil, errors.New("key must be 32 bytes")
	}
	if len(nonce) != chacha20poly1305.NonceSize {
		return nil, errors.New("nonce must be 12 bytes")
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, plaintext, aad), nil
}

// Ckx1AEADDecrypt opens ciphertext with ChaCha20-Poly1305 and AAD.
func Ckx1AEADDecrypt(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	if len(key) != CKX1KeySize {
		return nil, errors.New("key must be 32 bytes")
	}
	if len(nonce) != chacha20poly1305.NonceSize {
		return nil, errors.New("nonce must be 12 bytes")
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, aad)
}
