package com.remoteagent.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.Build
import android.util.Log
import androidx.core.content.ContextCompat
import com.remoteagent.config.AgentPreferences
import com.remoteagent.service.AgentService

class NetworkReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (!isNetworkAvailable(context)) return

        val preferences = AgentPreferences(context)
        val deviceId = preferences.getServerDeviceId()
        if (deviceId.isNullOrEmpty()) return

        val serviceIntent = Intent(context, AgentService::class.java).apply {
            putExtra(AgentService.EXTRA_SERVER_BASE_URL, preferences.getServerBaseUrl())
            putExtra(AgentService.EXTRA_SERVER_DEVICE_ID, deviceId)
        }
        // Android 12+ blocks starting a foreground service from the background
        // (connectivity broadcasts are not an allowed exemption).
        try {
            ContextCompat.startForegroundService(context, serviceIntent)
        } catch (e: IllegalStateException) {
            Log.w(TAG, "Skipped background FGS start from network change", e)
        } catch (e: SecurityException) {
            Log.w(TAG, "Skipped restricted FGS start from network change", e)
        }
    }

    private fun isNetworkAvailable(context: Context): Boolean {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager ?: return false
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val network = cm.activeNetwork ?: return false
            val capabilities = cm.getNetworkCapabilities(network) ?: return false
            return capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) ||
                    capabilities.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) ||
                    capabilities.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET)
        } else {
            @Suppress("DEPRECATION")
            val info = cm.activeNetworkInfo
            @Suppress("DEPRECATION")
            return info != null && info.isConnected
        }
    }

    companion object {
        private const val TAG = "NetworkReceiver"
    }
}
