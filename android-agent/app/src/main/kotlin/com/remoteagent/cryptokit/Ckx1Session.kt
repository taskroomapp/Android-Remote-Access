package com.remoteagent.cryptokit

import org.json.JSONObject
import java.security.SecureRandom
import java.util.Base64
import java.util.concurrent.atomic.AtomicLong

/**
 * Post-handshake CKX1 session: directional ChaCha20-Poly1305 frames.
 */
class Ckx1Session(
    private val deviceId: String,
    private val sessionId: String,
    private val clientToServerKey: ByteArray,
    private val serverToClientKey: ByteArray
) {
    private val sendSeq = AtomicLong(0)
    private val recvGuard = ReplayGuard()
    private val random = SecureRandom()

    fun ready(): Boolean =
        clientToServerKey.size == Ckx1Algorithm.KEY_SIZE &&
            serverToClientKey.size == Ckx1Algorithm.KEY_SIZE &&
            sessionId.isNotEmpty()

    fun seal(plaintextJson: String): String {
        val seq = sendSeq.incrementAndGet()
        val txn = extractTxn(plaintextJson).ifEmpty { "-" }
        val meta = Ckx1FrameMeta(
            sessionId = sessionId,
            deviceId = deviceId,
            direction = Ckx1Algorithm.DIR_DEVICE_TO_SERVER,
            sequence = seq,
            transactionId = txn
        )
        val nonce = ByteArray(Ckx1Algorithm.NONCE_SIZE).also { random.nextBytes(it) }
        val ct = Ckx1Aead.encrypt(
            clientToServerKey,
            nonce,
            plaintextJson.toByteArray(Charsets.UTF_8),
            meta.frameAad()
        )
        return JSONObject().apply {
            put("type", "enc")
            put("protocol", Ckx1Algorithm.PROTOCOL)
            put("version", Ckx1Algorithm.VERSION)
            put("session_id", sessionId)
            put("seq", seq)
            put("dir", Ckx1Algorithm.DIR_DEVICE_TO_SERVER)
            put("txn", txn)
            put("nonce", Base64.getEncoder().encodeToString(nonce))
            put("ciphertext", Base64.getEncoder().encodeToString(ct))
        }.toString()
    }

    fun open(encMessage: JSONObject): String {
        require(encMessage.optString("protocol") == Ckx1Algorithm.PROTOCOL) { "bad protocol" }
        require(encMessage.optInt("version", 0) == Ckx1Algorithm.VERSION) { "bad version" }
        require(encMessage.getString("session_id") == sessionId) { "session_id mismatch" }
        val dir = encMessage.getString("dir")
        require(dir == Ckx1Algorithm.DIR_SERVER_TO_DEVICE) { "incorrect direction: $dir" }
        val seq = encMessage.getLong("seq")
        recvGuard.accept(seq)
        val txn = encMessage.optString("txn", "-").ifEmpty { "-" }
        val meta = Ckx1FrameMeta(sessionId, deviceId, dir, seq, txn)
        val nonce = Base64.getDecoder().decode(encMessage.getString("nonce"))
        val ct = Base64.getDecoder().decode(encMessage.getString("ciphertext"))
        val plain = Ckx1Aead.decrypt(serverToClientKey, nonce, ct, meta.frameAad())
        return String(plain, Charsets.UTF_8)
    }

    fun clear() {
        sendSeq.set(0)
        recvGuard.reset()
        clientToServerKey.fill(0)
        serverToClientKey.fill(0)
    }

    private fun extractTxn(json: String): String = try {
        JSONObject(json).optString("transaction_id", "")
    } catch (_: Exception) {
        ""
    }
}
