package com.remoteagent.cryptokit

import android.content.Context
import android.content.SharedPreferences
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import android.util.Log
import java.io.File
import java.security.KeyPair
import java.security.KeyStore
import java.security.PrivateKey
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Device long-term CKX1 identity: X25519 (agreement) + Ed25519 (signing).
 *
 * Private key material is stored in app-private files, optionally wrapped with
 * an AES key held in the Android Keystore when available.
 */
class KeyStoreIdentity(private val context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    private var x25519Pair: KeyPair? = null
    private var ed25519Pair: KeyPair? = null

    init {
        loadOrGenerate()
    }

    fun deviceUuid(): String {
        var id = prefs.getString(KEY_DEVICE_UUID, null)
        if (id == null) {
            id = java.util.UUID.randomUUID().toString()
            prefs.edit().putString(KEY_DEVICE_UUID, id).apply()
        }
        return id
    }

    fun x25519PublicBase64(): String =
        X25519Keys.encodePublicRawBase64(requireNotNull(x25519Pair).public)

    fun ed25519PublicBase64(): String =
        Ed25519Identity.encodePublicRawBase64(requireNotNull(ed25519Pair).public)

    fun x25519Private(): PrivateKey = requireNotNull(x25519Pair).private

    fun ed25519Private(): PrivateKey = requireNotNull(ed25519Pair).private

    fun keyFingerprint(): String = Ckx1Canonical.fingerprintSha256Hex(
        X25519Keys.encodePublicRaw(requireNotNull(x25519Pair).public),
        Ed25519Identity.encodePublicRaw(requireNotNull(ed25519Pair).public)
    )

    fun pinServerFingerprint(fp: String) {
        prefs.edit().putString(KEY_SERVER_FP, fp).apply()
    }

    fun loadPinnedServerFingerprint(): String? = prefs.getString(KEY_SERVER_FP, null)

    /** Compatibility alias used by enrollment UI. */
    fun getCertificateFingerprint(): String = keyFingerprint()

    private fun loadOrGenerate() {
        try {
            val xPriv = readWrapped(FILE_X25519_PRIV)
            val ePriv = readWrapped(FILE_ED25519_PRIV)
            val xPub = readPlain(FILE_X25519_PUB)
            val ePub = readPlain(FILE_ED25519_PUB)
            if (xPriv != null && ePriv != null && xPub != null && ePub != null) {
                x25519Pair = KeyPair(
                    X25519Keys.decodePublicRawBase64(Base64.encodeToString(xPub, Base64.NO_WRAP)),
                    X25519Keys.decodePrivatePkcs8Base64(Base64.encodeToString(xPriv, Base64.NO_WRAP))
                )
                ed25519Pair = KeyPair(
                    Ed25519Identity.decodePublicRawBase64(Base64.encodeToString(ePub, Base64.NO_WRAP)),
                    Ed25519Identity.decodePrivatePkcs8Base64(Base64.encodeToString(ePriv, Base64.NO_WRAP))
                )
                Log.i(TAG, "Loaded CKX1 identity keys")
                return
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to load identity keys", e)
        }
        generateAndPersist()
    }

    private fun generateAndPersist() {
        Log.i(TAG, "Generating CKX1 X25519 + Ed25519 identity keys")
        x25519Pair = X25519Keys.generate()
        ed25519Pair = Ed25519Identity.generate()
        writeWrapped(FILE_X25519_PRIV, requireNotNull(x25519Pair).private.encoded)
        writeWrapped(FILE_ED25519_PRIV, requireNotNull(ed25519Pair).private.encoded)
        writePlain(FILE_X25519_PUB, X25519Keys.encodePublicRaw(requireNotNull(x25519Pair).public))
        writePlain(FILE_ED25519_PUB, Ed25519Identity.encodePublicRaw(requireNotNull(ed25519Pair).public))
        prefs.edit().putInt(KEY_VERSION, 1).apply()
    }

    private fun wrapKey(): SecretKey {
        val ks = KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }
        if (ks.containsAlias(WRAP_ALIAS)) {
            return ks.getKey(WRAP_ALIAS, null) as SecretKey
        }
        val kg = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE)
        kg.init(
            KeyGenParameterSpec.Builder(
                WRAP_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build()
        )
        return kg.generateKey()
    }

    private fun writeWrapped(name: String, plaintext: ByteArray) {
        try {
            val key = wrapKey()
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.ENCRYPT_MODE, key)
            val iv = cipher.iv
            val ct = cipher.doFinal(plaintext)
            val out = ByteArray(iv.size + ct.size)
            System.arraycopy(iv, 0, out, 0, iv.size)
            System.arraycopy(ct, 0, out, iv.size, ct.size)
            File(context.filesDir, name).writeBytes(out)
        } catch (e: Exception) {
            Log.w(TAG, "Keystore wrap unavailable; writing private file", e)
            File(context.filesDir, name).writeBytes(plaintext)
        }
    }

    private fun readWrapped(name: String): ByteArray? {
        val f = File(context.filesDir, name)
        if (!f.exists()) return null
        val data = f.readBytes()
        return try {
            val key = wrapKey()
            val iv = data.copyOfRange(0, 12)
            val ct = data.copyOfRange(12, data.size)
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, iv))
            cipher.doFinal(ct)
        } catch (_: Exception) {
            // Unwrapped private file fallback
            data
        }
    }

    private fun writePlain(name: String, bytes: ByteArray) {
        File(context.filesDir, name).writeBytes(bytes)
    }

    private fun readPlain(name: String): ByteArray? {
        val f = File(context.filesDir, name)
        return if (f.exists()) f.readBytes() else null
    }

    companion object {
        private const val TAG = "KeyStoreIdentity"
        private const val PREFS_NAME = "agent_ckx1_identity"
        private const val KEY_DEVICE_UUID = "device_uuid"
        private const val KEY_SERVER_FP = "server_ckx1_fp"
        private const val KEY_VERSION = "key_version"
        private const val ANDROID_KEYSTORE = "AndroidKeyStore"
        private const val WRAP_ALIAS = "ckx1_wrap_aes"
        private const val FILE_X25519_PRIV = "ckx1_x25519.priv"
        private const val FILE_ED25519_PRIV = "ckx1_ed25519.priv"
        private const val FILE_X25519_PUB = "ckx1_x25519.pub"
        private const val FILE_ED25519_PUB = "ckx1_ed25519.pub"
    }
}
