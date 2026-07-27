package com.remoteagent.cryptokit

import org.json.JSONObject

/**
 * Ephemeral admin-channel CKX1 session used by the agent during REST enrollment.
 * Matches web-panel establishAdminCkx1 / AdminCkx1Session semantics.
 */
class AdminCkx1Session private constructor(
    val adminId: String,
    val handshakeSessionId: String,
    /** Token for the X-CKX1-Session HTTP header (server's ckx1_session). */
    val protocolSessionToken: String,
    private val crypto: Ckx1Session
) {
    fun seal(plaintextJson: String): String = crypto.seal(plaintextJson)

    fun open(encFrame: JSONObject): String = crypto.open(encFrame)

    fun openIfEncrypted(rawBody: String): String {
        val trimmed = rawBody.trim()
        if (trimmed.isEmpty() || !trimmed.startsWith("{")) return rawBody
        val json = JSONObject(trimmed)
        return if (json.optString("type") == "enc") open(json) else rawBody
    }

    companion object {
        /**
         * Completes offer → exchange against authenticated admin endpoints.
         * [postJson] must send Authorization and return the response body string.
         */
        fun establish(
            adminId: String,
            pinnedServerFingerprint: String?,
            postJson: (path: String, body: String) -> String
        ): AdminCkx1Session {
            val xPair = X25519Keys.generate()
            val edPair = Ed25519Identity.generate()
            val xPub = X25519Keys.encodePublicRawBase64(xPair.public)
            val edPub = Ed25519Identity.encodePublicRawBase64(edPair.public)

            val offerRaw = postJson("/auth/ckx1/offer", "{}")
            val offer = JSONObject(offerRaw)
            val hs = Ckx1Handshake(
                deviceId = adminId,
                deviceX25519PublicB64 = xPub,
                deviceEd25519PublicB64 = edPub,
                deviceEd25519Private = edPair.private,
                deviceX25519Private = xPair.private
            )
            hs.acceptKeyOffer(offer, pinnedServerFingerprint)

            val exchange = hs.buildKeyExchange()
            val readyRaw = postJson("/auth/ckx1/exchange", exchange.toString())
            val ready = JSONObject(readyRaw)
            hs.confirmSessionReady(ready)
            val token = ready.optString("ckx1_session", "")
            require(token.isNotEmpty()) { "session_ready missing ckx1_session" }

            val (c2s, s2c) = hs.deriveSessionKeys()
            return AdminCkx1Session(
                adminId = adminId,
                handshakeSessionId = hs.sessionId,
                protocolSessionToken = token,
                crypto = Ckx1Session(adminId, hs.sessionId, c2s, s2c)
            )
        }
    }
}
