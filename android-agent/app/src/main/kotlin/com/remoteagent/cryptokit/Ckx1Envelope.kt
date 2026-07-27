package com.remoteagent.cryptokit

/**
 * Wire helpers for CKX1 session frames (JSON outer wrapper fields).
 */
data class Ckx1FrameMeta(
    val sessionId: String,
    val deviceId: String,
    val direction: String,
    val sequence: Long,
    val transactionId: String
) {
    fun frameAad(): ByteArray = Ckx1Canonical.encodeStrings(
        Ckx1Algorithm.FRAME_LABEL,
        sessionId,
        deviceId,
        direction,
        sequence.toString(),
        if (transactionId.isEmpty()) "-" else transactionId
    )
}
