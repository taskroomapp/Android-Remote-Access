package com.remoteagent.network

import android.util.Log
import com.remoteagent.cryptokit.AdminCkx1Session
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.io.IOException

class EnrollmentClient(
    private val httpClient: OkHttpClient,
    private val baseUrl: String,
    private val pinnedServerFingerprint: String? = null
) {
    companion object {
        private const val TAG = "EnrollmentClient"
        private val JSON = "application/json; charset=utf-8".toMediaType()
    }

    data class LoginResult(
        val accessToken: String,
        val adminId: String
    )

    private val normalizedBaseUrl = normalize(baseUrl)

    @Throws(IOException::class, EnrollmentException::class)
    fun login(username: String, password: String): LoginResult {
        verifyServerReachable()

        val body = JSONObject().apply {
            put("username", username)
            put("password", password)
        }

        val request = Request.Builder()
            .url("$normalizedBaseUrl/api/v1/auth/login")
            .post(body.toString().toRequestBody(JSON))
            .build()

        httpClient.newCall(request).execute().use { response ->
            val responseBody = response.body?.string() ?: ""
            if (!response.isSuccessful) {
                throw EnrollmentException("Login failed (${response.code}): $responseBody")
            }
            val json = JSONObject(responseBody)
            if (!json.has("access_token")) {
                throw EnrollmentException("Login response missing access_token")
            }
            val admin = json.optJSONObject("admin")
                ?: throw EnrollmentException("Login response missing admin")
            val adminId = admin.optString("id", "")
            if (adminId.isEmpty()) {
                throw EnrollmentException("Login response missing admin.id")
            }
            return LoginResult(json.getString("access_token"), adminId)
        }
    }

    /**
     * Establishes an admin CKX1 channel (required for POST /devices and other protected APIs).
     */
    @Throws(IOException::class, EnrollmentException::class)
    fun establishAdminCkx1(accessToken: String, adminId: String): AdminCkx1Session {
        return try {
            AdminCkx1Session.establish(adminId, pinnedServerFingerprint) { path, body ->
                authedPost("/api/v1$path", accessToken, body, ckx1 = null)
            }
        } catch (e: EnrollmentException) {
            throw e
        } catch (e: Exception) {
            throw EnrollmentException("CKX1 handshake failed: ${e.message}", e)
        }
    }

    @Throws(IOException::class, EnrollmentException::class)
    fun enrollDevice(
        accessToken: String,
        ckx1: AdminCkx1Session,
        deviceUuid: String,
        friendlyName: String,
        osVersion: String,
        hardwareModel: String,
        certificateHash: String,
        x25519PublicKey: String = "",
        ed25519PublicKey: String = "",
        keyFingerprint: String = certificateHash
    ): String {
        val body = JSONObject().apply {
            put("device_uuid", deviceUuid)
            put("friendly_name", friendlyName)
            put("owner", "field-agent")
            put("os_version", osVersion)
            put("hardware_model", hardwareModel)
            put("certificate_hash", certificateHash)
            put("x25519_public_key", x25519PublicKey)
            put("ed25519_public_key", ed25519PublicKey)
            put("key_fingerprint", keyFingerprint)
        }

        // Body may be plaintext; middleware requires X-CKX1-Session. Seal for parity with the panel.
        val sealed = ckx1.seal(body.toString())
        val responseBody = authedPost("/api/v1/devices", accessToken, sealed, ckx1)
        val json = JSONObject(ckx1.openIfEncrypted(responseBody))
        if (!json.has("device_id")) {
            throw EnrollmentException("Enrollment response missing device_id: $responseBody")
        }
        return json.getString("device_id")
    }

    @Throws(IOException::class, EnrollmentException::class)
    private fun authedPost(
        path: String,
        accessToken: String,
        body: String,
        ckx1: AdminCkx1Session?
    ): String {
        val builder = Request.Builder()
            .url("$normalizedBaseUrl$path")
            .addHeader("Authorization", "Bearer $accessToken")
            .post(body.toRequestBody(JSON))
        if (ckx1 != null) {
            builder.addHeader("X-CKX1-Session", ckx1.protocolSessionToken)
        }
        httpClient.newCall(builder.build()).execute().use { response ->
            val responseBody = response.body?.string() ?: ""
            val decoded = try {
                ckx1?.openIfEncrypted(responseBody) ?: responseBody
            } catch (e: Exception) {
                Log.w(TAG, "Failed to decrypt response", e)
                responseBody
            }
            if (!response.isSuccessful) {
                throw EnrollmentException("Request failed (${response.code}) $path: $decoded")
            }
            return decoded
        }
    }

    @Throws(IOException::class, EnrollmentException::class)
    private fun verifyServerReachable() {
        val request = Request.Builder()
            .url("$normalizedBaseUrl/health")
            .get()
            .build()
        try {
            httpClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    throw EnrollmentException("Server health check failed (${response.code})")
                }
            }
        } catch (e: EnrollmentException) {
            throw e
        } catch (e: IOException) {
            throw EnrollmentException(
                "Cannot reach server at $normalizedBaseUrl. On Windows emulator run: scripts/emulator-port-forward.ps1 then use http://127.0.0.1:8443",
                e
            )
        }
    }

    class EnrollmentException(message: String, cause: Throwable? = null) : Exception(message, cause) {
        override val message: String?
            get() {
                val causeMsg = cause?.message
                return if (causeMsg != null && causeMsg.isNotEmpty()) {
                    super.message + ": $causeMsg"
                } else super.message
            }
    }

    private fun normalize(url: String): String {
        return com.remoteagent.config.AgentPreferences.normalizeBaseUrl(url)
    }
}
