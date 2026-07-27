package com.remoteagent.config

import android.content.Context
import android.content.SharedPreferences

class AgentPreferences(context: Context) {
    private val prefs: SharedPreferences = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    companion object {
        /**
         * Built-in management server base URL.
         * Change this value when building the agent for a specific deployment.
         * Examples: "http://127.0.0.1:8443", "https://remote.example.com"
         */
        const val BUILT_IN_SERVER_URL = "http://142.168.23.12:8443"

        private const val PREFS = "agent_config"
        private const val KEY_SERVER_DEVICE_ID = "server_device_id"
        private const val KEY_AUTO_START = "auto_start"

        fun normalizeBaseUrl(url: String?): String {
            if (url == null) return ""
            var trimmed = url.trim()
            while (trimmed.endsWith("/")) {
                trimmed = trimmed.substring(0, trimmed.length - 1)
            }
            return trimmed
        }

        fun buildWebSocketUrl(baseUrl: String, serverDeviceId: String): String {
            val base = normalizeBaseUrl(baseUrl)
            val wsBase = when {
                base.startsWith("https://") -> "wss://" + base.substring("https://".length)
                base.startsWith("http://") -> "ws://" + base.substring("http://".length)
                else -> "wss://$base"
            }
            return "$wsBase/ws/devices/$serverDeviceId"
        }
    }

    /** Always returns the compile-time server URL (not user-editable). */
    fun getServerBaseUrl(): String = normalizeBaseUrl(BUILT_IN_SERVER_URL)

    fun getServerDeviceId(): String? = prefs.getString(KEY_SERVER_DEVICE_ID, null)

    fun setServerDeviceId(deviceId: String) {
        prefs.edit().putString(KEY_SERVER_DEVICE_ID, deviceId).apply()
    }

    fun isAutoStartEnabled(): Boolean = prefs.getBoolean(KEY_AUTO_START, true)

    fun setAutoStartEnabled(enabled: Boolean) {
        prefs.edit().putBoolean(KEY_AUTO_START, enabled).apply()
    }
}