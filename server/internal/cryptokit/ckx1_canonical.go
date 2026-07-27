package cryptokit

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
)

// CanonicalEncode length-prefixes each part as uint32 BE + bytes.
func CanonicalEncode(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += 4 + len(p)
	}
	out := make([]byte, 0, n)
	buf := make([]byte, 4)
	for _, p := range parts {
		binary.BigEndian.PutUint32(buf, uint32(len(p)))
		out = append(out, buf...)
		out = append(out, p...)
	}
	return out
}

// CanonicalEncodeStrings encodes UTF-8 strings with length prefixes.
func CanonicalEncodeStrings(values ...string) []byte {
	parts := make([][]byte, len(values))
	for i, v := range values {
		parts[i] = []byte(v)
	}
	return CanonicalEncode(parts...)
}

func SHA256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// FingerprintSHA256Hex returns sha256:<hex> over concatenated parts.
func FingerprintSHA256Hex(parts ...[]byte) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write(p)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func FingerprintLooseEqual(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.TrimPrefix(s, "sha256:")
		s = strings.ReplaceAll(s, ":", "")
		s = strings.ReplaceAll(s, " ", "")
		return s
	}
	return norm(a) == norm(b)
}
