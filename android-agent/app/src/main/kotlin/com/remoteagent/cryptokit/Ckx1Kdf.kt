package com.remoteagent.cryptokit

import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

object Ckx1Kdf {
    /**
     * HKDF-SHA256 (Extract + Expand) to [length] bytes.
     */
    fun hkdfSha256(ikm: ByteArray, salt: ByteArray, info: ByteArray, length: Int = Ckx1Algorithm.KEY_SIZE): ByteArray {
        require(length in 1..(255 * 32)) { "invalid HKDF length" }
        val mac = Mac.getInstance("HmacSHA256")
        val effectiveSalt = if (salt.isEmpty()) ByteArray(32) else salt
        mac.init(SecretKeySpec(effectiveSalt, "HmacSHA256"))
        val prk = mac.doFinal(ikm)

        mac.init(SecretKeySpec(prk, "HmacSHA256"))
        val out = ByteArray(length)
        var previous = ByteArray(0)
        var pos = 0
        var counter = 1
        while (pos < length) {
            mac.reset()
            mac.update(previous)
            mac.update(info)
            mac.update(counter.toByte())
            previous = mac.doFinal()
            val n = minOf(previous.size, length - pos)
            System.arraycopy(previous, 0, out, pos, n)
            pos += n
            counter++
        }
        return out
    }

    fun deriveDirectionalKeys(sharedSecret: ByteArray, transcriptHash: ByteArray): Pair<ByteArray, ByteArray> {
        val c2s = hkdfSha256(
            sharedSecret,
            transcriptHash,
            Ckx1Algorithm.INFO_C2S.toByteArray(Charsets.UTF_8)
        )
        val s2c = hkdfSha256(
            sharedSecret,
            transcriptHash,
            Ckx1Algorithm.INFO_S2C.toByteArray(Charsets.UTF_8)
        )
        return c2s to s2c
    }
}
