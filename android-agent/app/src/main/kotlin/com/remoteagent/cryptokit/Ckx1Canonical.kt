package com.remoteagent.cryptokit

import java.io.ByteArrayOutputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.nio.charset.StandardCharsets
import java.security.MessageDigest

/**
 * Length-prefixed canonical encoding (uint32 BE length + UTF-8 / raw bytes).
 * Avoids ambiguous string concatenation for transcripts and AAD.
 */
object Ckx1Canonical {
    fun encode(vararg parts: ByteArray): ByteArray {
        val out = ByteArrayOutputStream()
        for (p in parts) {
            val len = ByteBuffer.allocate(4).order(ByteOrder.BIG_ENDIAN).putInt(p.size).array()
            out.write(len)
            out.write(p)
        }
        return out.toByteArray()
    }

    fun encodeStrings(vararg values: String): ByteArray {
        return encode(*values.map { it.toByteArray(StandardCharsets.UTF_8) }.toTypedArray())
    }

    fun sha256(data: ByteArray): ByteArray =
        MessageDigest.getInstance("SHA-256").digest(data)

    fun fingerprintSha256Hex(vararg parts: ByteArray): String {
        val md = MessageDigest.getInstance("SHA-256")
        for (p in parts) md.update(p)
        return "sha256:" + md.digest().joinToString("") { "%02x".format(it) }
    }

    fun fingerprintLooseEqual(a: String, b: String): Boolean {
        fun norm(s: String) = s.lowercase()
            .removePrefix("sha256:")
            .replace(":", "")
            .replace(" ", "")
        return norm(a) == norm(b)
    }
}
