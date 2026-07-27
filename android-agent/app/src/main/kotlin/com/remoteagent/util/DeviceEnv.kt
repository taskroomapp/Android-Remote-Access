package com.remoteagent.util

import android.os.Build

object DeviceEnv {
    fun isEmulator(): Boolean {
        val fingerprint = Build.FINGERPRINT ?: ""
        val model = Build.MODEL ?: ""
        val manufacturer = Build.MANUFACTURER ?: ""
        val brand = Build.BRAND ?: ""
        val device = Build.DEVICE ?: ""
        val product = Build.PRODUCT ?: ""

        return fingerprint.startsWith("generic") ||
                fingerprint.contains("emulator", ignoreCase = true) ||
                model.contains("Emulator") ||
                model.contains("Android SDK built for") ||
                manufacturer.contains("Genymotion") ||
                product.contains("sdk") ||
                (brand.startsWith("generic") && device.startsWith("generic"))
    }

    /** Human-readable Android version for enrollment / control panel. */
    fun androidVersionLabel(): String {
        val release = Build.VERSION.RELEASE?.takeIf { it.isNotBlank() } ?: "?"
        return "Android $release (API ${Build.VERSION.SDK_INT})"
    }

    fun hardwareLabel(): String {
        val manufacturer = Build.MANUFACTURER?.trim().orEmpty()
        val model = Build.MODEL?.trim().orEmpty()
        return when {
            manufacturer.isNotEmpty() && model.isNotEmpty() &&
                !model.startsWith(manufacturer, ignoreCase = true) -> "$manufacturer $model"
            model.isNotEmpty() -> model
            manufacturer.isNotEmpty() -> manufacturer
            else -> "Unknown device"
        }
    }

    fun friendlyName(): String = Build.MODEL?.takeIf { it.isNotBlank() } ?: hardwareLabel()

    /** Default API base URL for local dev. */
    fun defaultLocalServerUrl(): String {
        return if (isEmulator()) {
            // Requires: adb reverse tcp:8443 tcp:8443 (see scripts/emulator-port-forward.ps1)
            "http://192.168.23.12:8443"
        } else {
            "http://10.0.2.2:8443"
        }
    }
}
