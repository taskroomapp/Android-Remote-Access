package cryptokit

import (
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

// HKDFSHA256 derives length bytes using HKDF-SHA256.
func HKDFSHA256(ikm, salt, info []byte, length int) ([]byte, error) {
	if length <= 0 || length > 255*32 {
		return nil, errors.New("invalid HKDF length")
	}
	r := hkdf.New(sha256.New, ikm, salt, info)
	out := make([]byte, length)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeriveDirectionalKeys returns (clientToServer, serverToClient) 32-byte keys.
func DeriveDirectionalKeys(sharedSecret, transcriptHash []byte) (c2s, s2c []byte, err error) {
	c2s, err = HKDFSHA256(sharedSecret, transcriptHash, []byte(CKX1InfoC2S), CKX1KeySize)
	if err != nil {
		return nil, nil, err
	}
	s2c, err = HKDFSHA256(sharedSecret, transcriptHash, []byte(CKX1InfoS2C), CKX1KeySize)
	if err != nil {
		return nil, nil, err
	}
	return c2s, s2c, nil
}
