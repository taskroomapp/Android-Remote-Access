package com.remoteagent.cryptokit

import javax.crypto.Cipher
import javax.crypto.spec.IvParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * ChaCha20-Poly1305 AEAD only. Nonce must be 12 bytes; key 32 bytes.
 */
object Ckx1Aead {
    init {
        BcProvider.ensureInstalled()
    }

    fun encrypt(key: ByteArray, nonce: ByteArray, plaintext: ByteArray, aad: ByteArray): ByteArray {
        require(key.size == Ckx1Algorithm.KEY_SIZE) { "key must be 32 bytes" }
        require(nonce.size == Ckx1Algorithm.NONCE_SIZE) { "nonce must be 12 bytes" }
        val cipher = Cipher.getInstance("ChaCha20-Poly1305", "BC")
        cipher.init(Cipher.ENCRYPT_MODE, SecretKeySpec(key, "ChaCha20"), IvParameterSpec(nonce))
        if (aad.isNotEmpty()) cipher.updateAAD(aad)
        return cipher.doFinal(plaintext)
    }

    fun decrypt(key: ByteArray, nonce: ByteArray, ciphertext: ByteArray, aad: ByteArray): ByteArray {
        require(key.size == Ckx1Algorithm.KEY_SIZE) { "key must be 32 bytes" }
        require(nonce.size == Ckx1Algorithm.NONCE_SIZE) { "nonce must be 12 bytes" }
        require(ciphertext.size >= Ckx1Algorithm.TAG_SIZE) { "ciphertext too short" }
        val cipher = Cipher.getInstance("ChaCha20-Poly1305", "BC")
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(key, "ChaCha20"), IvParameterSpec(nonce))
        if (aad.isNotEmpty()) cipher.updateAAD(aad)
        return cipher.doFinal(ciphertext)
    }
}
