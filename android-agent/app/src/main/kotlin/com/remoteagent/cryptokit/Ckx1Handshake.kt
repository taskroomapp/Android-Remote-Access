package com.remoteagent.cryptokit

import org.json.JSONObject
import java.security.PrivateKey
import java.security.PublicKey
import java.security.SecureRandom
import java.util.Base64

/**
 * CKX1 handshake: key_offer → key_exchange → session_ready.
 */
class Ckx1Handshake(
    private val deviceId: String,
    private val deviceX25519PublicB64: String,
    private val deviceEd25519PublicB64: String,
    private val deviceEd25519Private: PrivateKey,
    private val deviceX25519Private: PrivateKey
) {
    var sessionId: String = ""
        private set
    var serverNonce: String = ""
        private set
    var serverFingerprint: String = ""
        private set
    var serverX25519PublicB64: String = ""
        private set
    var serverEd25519PublicB64: String = ""
        private set
    var deviceNonce: String = ""
        private set
    var transcriptBytes: ByteArray? = null
        private set
    var transcriptHash: ByteArray? = null
        private set

    fun clear() {
        sessionId = ""
        serverNonce = ""
        serverFingerprint = ""
        serverX25519PublicB64 = ""
        serverEd25519PublicB64 = ""
        deviceNonce = ""
        transcriptBytes = null
        transcriptHash = null
    }

    fun acceptKeyOffer(json: JSONObject, pinnedFingerprint: String?) {
        require(json.getString("protocol") == Ckx1Algorithm.PROTOCOL) { "unsupported protocol" }
        require(json.optInt("version", 0) == Ckx1Algorithm.VERSION) { "unsupported version" }
        val alg = json.getString("algorithm")
        require(alg == Ckx1Algorithm.ALGORITHM) { "unsupported algorithm $alg" }

        sessionId = json.getString("session_id")
        serverX25519PublicB64 = json.getString("server_x25519_public_key")
        serverEd25519PublicB64 = json.getString("server_ed25519_public_key")
        serverFingerprint = json.getString("server_fingerprint")
        serverNonce = json.getString("server_nonce")

        val computed = Ckx1Canonical.fingerprintSha256Hex(
            Base64.getDecoder().decode(serverX25519PublicB64),
            Base64.getDecoder().decode(serverEd25519PublicB64)
        )
        require(Ckx1Canonical.fingerprintLooseEqual(computed, serverFingerprint)) {
            "server fingerprint mismatch with offered keys"
        }
        if (pinnedFingerprint != null) {
            require(Ckx1Canonical.fingerprintLooseEqual(pinnedFingerprint, serverFingerprint)) {
                "pinned server fingerprint mismatch — possible MITM"
            }
        }
    }

    fun buildKeyExchange(): JSONObject {
        require(sessionId.isNotEmpty() && serverNonce.isNotEmpty()) { "key_offer not accepted" }
        val nonce = ByteArray(16).also { SecureRandom().nextBytes(it) }
        deviceNonce = Base64.getEncoder().encodeToString(nonce)

        val transcript = Ckx1Canonical.encodeStrings(
            Ckx1Algorithm.HANDSHAKE_LABEL,
            sessionId,
            deviceId,
            serverX25519PublicB64,
            serverEd25519PublicB64,
            deviceX25519PublicB64,
            deviceEd25519PublicB64,
            serverNonce,
            deviceNonce,
            Ckx1Algorithm.ALGORITHM
        )
        transcriptBytes = transcript
        transcriptHash = Ckx1Canonical.sha256(transcript)
        val signature = Ed25519Identity.sign(transcript, deviceEd25519Private)

        return JSONObject().apply {
            put("type", "key_exchange")
            put("protocol", Ckx1Algorithm.PROTOCOL)
            put("version", Ckx1Algorithm.VERSION)
            put("session_id", sessionId)
            put("device_id", deviceId)
            put("device_x25519_public_key", deviceX25519PublicB64)
            put("device_ed25519_public_key", deviceEd25519PublicB64)
            put("device_nonce", deviceNonce)
            put("signature", Base64.getEncoder().encodeToString(signature))
        }
    }

    fun deriveSessionKeys(): Pair<ByteArray, ByteArray> {
        val hash = transcriptHash ?: throw IllegalStateException("transcript not built")
        val peerPub = X25519Keys.decodePublicRawBase64(serverX25519PublicB64)
        val shared = X25519Keys.sharedSecret(deviceX25519Private, peerPub)
        return Ckx1Kdf.deriveDirectionalKeys(shared, hash)
    }

    fun confirmSessionReady(json: JSONObject) {
        require(json.getString("protocol") == Ckx1Algorithm.PROTOCOL)
        require(json.optInt("version", 0) == Ckx1Algorithm.VERSION)
        require(json.getString("session_id") == sessionId) { "session_id mismatch" }
        require(json.getString("algorithm") == Ckx1Algorithm.ALGORITHM)
    }

    companion object {
        fun serverPublicKeys(x25519B64: String, ed25519B64: String): Pair<PublicKey, PublicKey> =
            X25519Keys.decodePublicRawBase64(x25519B64) to Ed25519Identity.decodePublicRawBase64(ed25519B64)
    }
}
