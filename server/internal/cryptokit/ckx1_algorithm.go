package cryptokit

// CKX1 — X25519 + HKDF-SHA256 + ChaCha20-Poly1305 + Ed25519.
// Replaces RSA-OAEP / AES-256-GCM / CKR1 for device WebSocket sessions.
const (
	CKX1Protocol  = "CKX1"
	CKX1Version   = 1
	CKX1Algorithm = "X25519-HKDF-SHA256-CHACHA20-POLY1305"

	CKX1HandshakeLabel = "CKX1-HANDSHAKE-V1"
	CKX1FrameLabel     = "CKX1-FRAME-V1"
	CKX1InfoC2S        = "CKX1/client-to-server"
	CKX1InfoS2C        = "CKX1/server-to-client"
	CKX1AtRestContext  = "CKX1-ATREST"

	CKX1KeySize        = 32
	CKX1NonceSize      = 12
	CKX1TagSize        = 16
	CKX1PublicKeySize  = 32
	DirDeviceToServer  = "device-to-server"
	DirServerToDevice  = "server-to-device"
)
