package com.remoteagent.cryptokit

/**
 * CKX1 constants — X25519 + HKDF-SHA256 + ChaCha20-Poly1305 + Ed25519.
 * This replaces the former RSA-OAEP / AES-256-GCM / CKR1 session path.
 */
object Ckx1Algorithm {
    const val PROTOCOL = "CKX1"
    const val VERSION = 1
    const val ALGORITHM = "X25519-HKDF-SHA256-CHACHA20-POLY1305"
    const val HANDSHAKE_LABEL = "CKX1-HANDSHAKE-V1"
    const val FRAME_LABEL = "CKX1-FRAME-V1"
    const val INFO_C2S = "CKX1/client-to-server"
    const val INFO_S2C = "CKX1/server-to-client"
    const val ATREST_CONTEXT = "CKX1-ATREST"

    const val KEY_SIZE = 32
    const val NONCE_SIZE = 12
    const val TAG_SIZE = 16
    const val PUBLIC_KEY_SIZE = 32

    const val DIR_DEVICE_TO_SERVER = "device-to-server"
    const val DIR_SERVER_TO_DEVICE = "server-to-device"
}
