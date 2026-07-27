package com.remoteagent.cryptokit

import org.bouncycastle.jce.provider.BouncyCastleProvider
import java.security.Security

/**
 * Android ships a truncated "BC" provider that lacks X25519/Ed25519.
 * Replace it with the full BouncyCastle jar before any CKX1 crypto runs.
 */
internal object BcProvider {
    @Volatile
    private var installed = false

    fun ensureInstalled() {
        if (installed) return
        synchronized(this) {
            if (installed) return
            Security.removeProvider(BouncyCastleProvider.PROVIDER_NAME)
            Security.insertProviderAt(BouncyCastleProvider(), 1)
            installed = true
        }
    }
}
