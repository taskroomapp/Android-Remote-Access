package com.remoteagent.cryptokit

import org.bouncycastle.asn1.edec.EdECObjectIdentifiers
import org.bouncycastle.asn1.x509.AlgorithmIdentifier
import org.bouncycastle.asn1.x509.SubjectPublicKeyInfo
import java.security.KeyFactory
import java.security.KeyPair
import java.security.KeyPairGenerator
import java.security.PrivateKey
import java.security.PublicKey
import java.security.Signature
import java.security.spec.PKCS8EncodedKeySpec
import java.security.spec.X509EncodedKeySpec
import java.util.Base64

object Ed25519Identity {
    init {
        BcProvider.ensureInstalled()
    }

    fun generate(): KeyPair =
        KeyPairGenerator.getInstance("Ed25519", "BC").generateKeyPair()

    fun encodePublicRaw(publicKey: PublicKey): ByteArray {
        val spki = SubjectPublicKeyInfo.getInstance(publicKey.encoded)
        val raw = spki.publicKeyData.bytes
        require(raw.size == Ckx1Algorithm.PUBLIC_KEY_SIZE) { "Ed25519 public key must be 32 bytes" }
        return raw
    }

    fun encodePublicRawBase64(publicKey: PublicKey): String =
        Base64.getEncoder().encodeToString(encodePublicRaw(publicKey))

    fun decodePublicRawBase64(base64: String): PublicKey {
        val raw = Base64.getDecoder().decode(base64)
        require(raw.size == Ckx1Algorithm.PUBLIC_KEY_SIZE) { "Ed25519 public key must be 32 bytes" }
        val spki = SubjectPublicKeyInfo(
            AlgorithmIdentifier(EdECObjectIdentifiers.id_Ed25519),
            raw
        )
        return KeyFactory.getInstance("Ed25519", "BC")
            .generatePublic(X509EncodedKeySpec(spki.encoded))
    }

    fun encodePrivatePkcs8Base64(privateKey: PrivateKey): String =
        Base64.getEncoder().encodeToString(privateKey.encoded)

    fun decodePrivatePkcs8Base64(base64: String): PrivateKey =
        KeyFactory.getInstance("Ed25519", "BC")
            .generatePrivate(PKCS8EncodedKeySpec(Base64.getDecoder().decode(base64)))

    fun sign(message: ByteArray, privateKey: PrivateKey): ByteArray {
        val sig = Signature.getInstance("Ed25519", "BC")
        sig.initSign(privateKey)
        sig.update(message)
        return sig.sign()
    }

    fun verify(message: ByteArray, signature: ByteArray, publicKey: PublicKey): Boolean {
        val sig = Signature.getInstance("Ed25519", "BC")
        sig.initVerify(publicKey)
        sig.update(message)
        return sig.verify(signature)
    }
}
