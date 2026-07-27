package com.remoteagent.security

import android.content.Context
import java.security.KeyStore
import java.security.SecureRandom
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

/**
 * Builds a default TLS context for WSS. Device identity is CKX1 (X25519/Ed25519),
 * not an RSA client certificate — mTLS is not required by the current server.
 */
class TLSConnectionFactory(
    @Suppress("UNUSED_PARAMETER") context: Context,
    @Suppress("UNUSED_PARAMETER") unused: Any? = null
) {
    val sslContext: SSLContext
    val trustManager: X509TrustManager

    init {
        val tmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        tmf.init(null as KeyStore?)
        val tm = tmf.trustManagers[0]
        if (tm !is X509TrustManager) {
            throw IllegalStateException("Default trust manager is not X509")
        }
        trustManager = tm
        sslContext = SSLContext.getInstance("TLSv1.2").apply {
            init(null, arrayOf(trustManager), SecureRandom())
        }
    }
}
