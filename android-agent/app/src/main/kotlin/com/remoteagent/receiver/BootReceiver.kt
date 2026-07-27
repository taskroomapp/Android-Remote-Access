package com.remoteagent.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import androidx.core.content.ContextCompat
import com.remoteagent.config.AgentPreferences
import com.remoteagent.service.AgentService

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (intent == null || intent.action == null) return
        val action = intent.action
        if (action != Intent.ACTION_BOOT_COMPLETED && action != "android.intent.action.QUICKBOOT_POWERON") {
            return
        }

        val preferences = AgentPreferences(context)
        if (!preferences.isAutoStartEnabled()) return
        val deviceId = preferences.getServerDeviceId()
        if (deviceId.isNullOrEmpty()) return

        val serviceIntent = Intent(context, AgentService::class.java).apply {
            putExtra(AgentService.EXTRA_SERVER_BASE_URL, preferences.getServerBaseUrl())
            putExtra(AgentService.EXTRA_SERVER_DEVICE_ID, deviceId)
        }
        try {
            ContextCompat.startForegroundService(context, serviceIntent)
        } catch (e: IllegalStateException) {
            android.util.Log.w("BootReceiver", "Cannot start FGS after boot yet", e)
        } catch (e: SecurityException) {
            android.util.Log.w("BootReceiver", "Cannot start FGS after boot", e)
        }
    }
}
