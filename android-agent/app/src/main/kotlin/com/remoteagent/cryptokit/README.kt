package com.remoteagent.cryptokit

/**
 * CKX1 package layout:
 *
 * - [Ckx1Algorithm] constants
 * - [Ckx1Canonical] length-prefixed encoding + fingerprints
 * - [Ckx1Kdf] HKDF-SHA256
 * - [Ckx1Aead] ChaCha20-Poly1305
 * - [X25519Keys] / [Ed25519Identity]
 * - [Ckx1Handshake] / [Ckx1Session] / [ReplayGuard]
 * - [KeyStoreIdentity] device long-term keys
 * - [Ckx1Envelope] wire helpers
 *
 * RSA/AES/CKR1/CK01 session helpers are removed from this package.
 */
@Suppress("unused")
private object Ckx1PackageDoc
