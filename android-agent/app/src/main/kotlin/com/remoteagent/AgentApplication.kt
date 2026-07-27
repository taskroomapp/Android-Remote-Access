package com.remoteagent

import android.app.Application
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.os.Build
import com.remoteagent.R
import com.remoteagent.cryptokit.BcProvider

class AgentApplication : Application() {
    companion object {
        const val CHANNEL_ID = "agent_service_channel"
        const val CHANNEL_NAME = "Agent Service"
        private lateinit var instance: AgentApplication

        fun getInstance(): AgentApplication = instance
    }

    override fun onCreate() {
        super.onCreate()
        instance = this
        BcProvider.ensureInstalled()
        createNotificationChannel()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                CHANNEL_NAME,
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Keeps the agent connected to the server"
                setShowBadge(false)
                enableLights(false)
                enableVibration(false)
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager?.createNotificationChannel(channel)
        }
    }

    fun getForegroundNotification(): Notification {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            Notification.Builder(this)
        }.apply {
            setContentTitle(getString(R.string.agent_notification_title))
            setContentText(getString(R.string.agent_notification_text))
            setSmallIcon(R.drawable.ic_notification)
            setPriority(Notification.PRIORITY_LOW)
            setOngoing(true)
        }.build()
    }
}