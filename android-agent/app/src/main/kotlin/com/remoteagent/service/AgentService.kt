package com.remoteagent.service

import android.app.Service
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.ServiceInfo
import android.os.Binder
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log
import androidx.core.content.ContextCompat
import com.remoteagent.config.AgentPreferences
import com.remoteagent.cryptokit.Ckx1Handshake
import com.remoteagent.cryptokit.Ckx1Session
import com.remoteagent.cryptokit.KeyStoreIdentity
import com.remoteagent.network.ServerConnection
import com.remoteagent.protocol.CommandHandler
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import com.remoteagent.security.TLSConnectionFactory
import com.remoteagent.util.DeviceEnv
import org.json.JSONException
import org.json.JSONObject
import java.util.Base64
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

class AgentService : Service() {
    private val binder = LocalBinder()
    private lateinit var handler: Handler
    private lateinit var messageExecutor: ExecutorService
    private lateinit var commandExecutor: ExecutorService

    private var serverConnection: ServerConnection? = null
    private lateinit var commandHandler: CommandHandler
    private lateinit var identity: KeyStoreIdentity
    private var handshake: Ckx1Handshake? = null
    private var session: Ckx1Session? = null

    private var agentDeviceUuid: String = ""
    private var webSocketUrl: String? = null
    private var isRunning = false
    @Volatile
    private var connected = false
    @Volatile
    private var sessionReady = false

    inner class LocalBinder : Binder() {
        fun getService(): AgentService = this@AgentService
    }

    override fun onCreate() {
        super.onCreate()
        handler = Handler(Looper.getMainLooper())
        messageExecutor = Executors.newSingleThreadExecutor()
        commandExecutor = Executors.newFixedThreadPool(4)
        identity = KeyStoreIdentity(this)
        commandHandler = CommandHandler(this)
        agentDeviceUuid = identity.deviceUuid()
        Log.i(TAG, "AgentService created with agent UUID: $agentDeviceUuid")
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        intent?.let {
            val baseUrl = it.getStringExtra(EXTRA_SERVER_BASE_URL)
            val serverDeviceId = it.getStringExtra(EXTRA_SERVER_DEVICE_ID)
            if (baseUrl != null && serverDeviceId != null) {
                webSocketUrl = AgentPreferences.buildWebSocketUrl(baseUrl, serverDeviceId)
            }
        }
        if (webSocketUrl.isNullOrEmpty()) {
            val preferences = AgentPreferences(this)
            val deviceId = preferences.getServerDeviceId()
            if (deviceId != null) {
                webSocketUrl = AgentPreferences.buildWebSocketUrl(preferences.getServerBaseUrl(), deviceId)
            }
        }

        startForegroundService()
        startConnection()
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder = binder

    private fun startForegroundService() {
        val notification = com.remoteagent.AgentApplication.getInstance().getForegroundNotification()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(1, notification, foregroundServiceTypes())
        } else {
            startForeground(1, notification)
        }
    }

    /**
     * dataSync keeps the agent connected; camera/microphone types are required on
     * Android 11+ / 14 so still capture and mic work while the UI is backgrounded.
     */
    private fun foregroundServiceTypes(): Int {
        var types = ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            types = types or
                ServiceInfo.FOREGROUND_SERVICE_TYPE_CAMERA or
                ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE
        }
        return types
    }

    fun startConnection() {
        if (isRunning || webSocketUrl.isNullOrEmpty()) return
        isRunning = true
        connectToServer()
        startHeartbeat()
    }

    fun stopConnection() {
        isRunning = false
        connected = false
        sessionReady = false
        clearCrypto()
        serverConnection?.disconnect()
        serverConnection = null
        handler.removeCallbacksAndMessages(null)
    }

    private fun clearCrypto() {
        session?.clear()
        session = null
        handshake?.clear()
        handshake = null
    }

    private fun connectToServer() {
        val url = webSocketUrl
        if (url.isNullOrEmpty()) {
            Log.w(TAG, "Missing WebSocket URL")
            return
        }

        serverConnection?.disconnect()
        sessionReady = false
        clearCrypto()
        handshake = Ckx1Handshake(
            deviceId = agentDeviceUuid,
            deviceX25519PublicB64 = identity.x25519PublicBase64(),
            deviceEd25519PublicB64 = identity.ed25519PublicBase64(),
            deviceEd25519Private = identity.ed25519Private(),
            deviceX25519Private = identity.x25519Private()
        )

        try {
            val tlsFactory = TLSConnectionFactory(this, null)
            serverConnection = ServerConnection(
                url,
                agentDeviceUuid,
                tlsFactory.sslContext,
                tlsFactory.trustManager,
                object : ServerConnection.ConnectionListener {
                    override fun onConnected() {
                        Log.i(TAG, "Connected to server; waiting for CKX1 key_offer")
                        connected = true
                        broadcastConnectionState(true)
                    }

                    override fun onDisconnected(reason: String) {
                        Log.w(TAG, "Disconnected from server: $reason")
                        connected = false
                        sessionReady = false
                        clearCrypto()
                        broadcastConnectionState(false)
                        scheduleReconnect()
                    }

                    override fun onMessage(message: String) {
                        handleServerMessage(message)
                    }

                    override fun onError(e: Exception) {
                        Log.e(TAG, "Connection error", e)
                        connected = false
                        sessionReady = false
                        clearCrypto()
                        broadcastConnectionState(false)
                    }
                }
            )
            serverConnection?.connect()
        } catch (e: Exception) {
            Log.e(TAG, "Failed to initialize TLS connection", e)
            scheduleReconnect()
        }
    }

    private fun scheduleReconnect() {
        if (!isRunning) return
        handler.postDelayed({
            if (isRunning && !connected) {
                Log.i(TAG, "Attempting to reconnect...")
                connectToServer()
            }
        }, RECONNECT_DELAY)
    }

    private fun startHeartbeat() {
        handler.postDelayed(object : Runnable {
            override fun run() {
                if (isRunning && connected && sessionReady) {
                    sendHeartbeat()
                    handler.postDelayed(this, HEARTBEAT_INTERVAL)
                } else if (isRunning) {
                    handler.postDelayed(this, HEARTBEAT_INTERVAL / 2)
                }
            }
        }, HEARTBEAT_INTERVAL)
    }

    private fun sendEncrypted(json: JSONObject) {
        val sess = session
        if (sess == null || !sessionReady || !sess.ready()) {
            Log.w(TAG, "Cannot send encrypted message before CKX1 session_ready")
            return
        }
        if (serverConnection != null && connected) {
            serverConnection?.send(sess.seal(json.toString()))
        }
    }

    private fun sendHeartbeat() {
        try {
            val heartbeat = JSONObject().apply {
                put("type", "heartbeat")
                put("battery_level", getBatteryLevel())
                put("timestamp", System.currentTimeMillis())
            }
            sendEncrypted(heartbeat)
        } catch (e: JSONException) {
            Log.e(TAG, "Failed to send heartbeat", e)
        }
    }

    private fun getBatteryLevel(): Int {
        val batteryIntent = ContextCompat.registerReceiver(
            this,
            null,
            IntentFilter(Intent.ACTION_BATTERY_CHANGED),
            ContextCompat.RECEIVER_NOT_EXPORTED
        )
        batteryIntent?.let {
            val level = it.getIntExtra(android.os.BatteryManager.EXTRA_LEVEL, -1)
            val scale = it.getIntExtra(android.os.BatteryManager.EXTRA_SCALE, -1)
            if (level >= 0 && scale > 0) {
                return (level * 100 / scale)
            }
        }
        return 0
    }

    private fun broadcastConnectionState(connected: Boolean) {
        val intent = Intent(ACTION_CONNECTION_STATE).apply {
            putExtra(EXTRA_CONNECTED, connected)
            setPackage(packageName)
        }
        sendBroadcast(intent)
    }

    private fun sendEnrollment() {
        try {
            val enrollment = JSONObject().apply {
                put("type", "enrollment")
                put("device_uuid", agentDeviceUuid)
                put("friendly_name", DeviceEnv.friendlyName())
                put("os_version", DeviceEnv.androidVersionLabel())
                put("hardware_model", DeviceEnv.hardwareLabel())
                put("manufacturer", Build.MANUFACTURER)
                put("sdk_version", Build.VERSION.SDK_INT)
                put("certificate_hash", identity.keyFingerprint())
                put("x25519_public_key", identity.x25519PublicBase64())
                put("ed25519_public_key", identity.ed25519PublicBase64())
                put("key_fingerprint", identity.keyFingerprint())
            }
            sendEncrypted(enrollment)
        } catch (e: JSONException) {
            Log.e(TAG, "Failed to send enrollment", e)
        }
    }

    private fun handleKeyOffer(json: JSONObject) {
        val hs = handshake ?: return
        if (sessionReady || session != null) {
            Log.w(TAG, "Ignoring key_offer; session already in progress or ready")
            return
        }
        try {
            val pinned = identity.loadPinnedServerFingerprint()
            hs.acceptKeyOffer(json, pinned)
            val exchange = hs.buildKeyExchange()
            serverConnection?.send(exchange.toString())
            if (pinned == null) {
                identity.pinServerFingerprint(hs.serverFingerprint)
            }
            Log.i(TAG, "Sent authenticated CKX1 key_exchange")
        } catch (e: Exception) {
            Log.e(TAG, "Failed CKX1 key_exchange", e)
            clearCrypto()
            handshake = Ckx1Handshake(
                deviceId = agentDeviceUuid,
                deviceX25519PublicB64 = identity.x25519PublicBase64(),
                deviceEd25519PublicB64 = identity.ed25519PublicBase64(),
                deviceEd25519Private = identity.ed25519Private(),
                deviceX25519Private = identity.x25519Private()
            )
        }
    }

    private fun handleServerMessage(message: String) {
        messageExecutor.execute {
            try {
                val json = JSONObject(message)
                when (json.optString("type", "")) {
                    "key_offer" -> {
                        handleKeyOffer(json)
                        return@execute
                    }
                    "session_ready" -> {
                        val hs = handshake
                        if (hs == null) {
                            Log.w(TAG, "session_ready without handshake")
                            return@execute
                        }
                        try {
                            hs.confirmSessionReady(json)
                            val (c2s, s2c) = hs.deriveSessionKeys()
                            session = Ckx1Session(agentDeviceUuid, hs.sessionId, c2s, s2c)
                            sessionReady = true
                            Log.i(TAG, "CKX1 session ready; sending enrollment")
                            sendEnrollment()
                        } catch (e: Exception) {
                            Log.e(TAG, "Failed to confirm CKX1 session", e)
                            clearCrypto()
                            sessionReady = false
                        }
                        return@execute
                    }
                    "enc" -> {
                        if (!sessionReady) {
                            Log.w(TAG, "Received enc before session_ready")
                            return@execute
                        }
                        val sess = session
                        if (sess == null || !sess.ready()) {
                            Log.w(TAG, "Received enc without established session")
                            return@execute
                        }
                        val inner = JSONObject(sess.open(json))
                        handleApplicationMessage(inner)
                        return@execute
                    }
                    else -> {
                        Log.w(TAG, "Ignoring unexpected plaintext type=${json.optString("type")}")
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "Failed to handle server message", e)
            }
        }
    }

    private fun handleApplicationMessage(json: JSONObject) {
        try {
            val type = json.optString("type", "")
            val command = if ("command" == type) {
                DeviceCommand.fromJSON(json)
            } else if (json.has("command_type") && json.has("transaction_id")) {
                DeviceCommand.fromJSON(json)
            } else {
                null
            }
            command?.let { cmd ->
                commandExecutor.execute { executeCommand(cmd) }
            }
        } catch (e: JSONException) {
            Log.e(TAG, "Failed to parse application message", e)
        }
    }

    private fun executeCommand(command: DeviceCommand) {
        Log.i(TAG, "Executing command: ${command.commandType} (txn: ${command.transactionId})")
        val result = commandHandler.handleCommand(command)
        sendCommandResponse(command, result)
    }

    private fun sendCommandResponse(command: DeviceCommand, result: CommandResult) {
        try {
            val response = JSONObject().apply {
                put("type", "response")
                put("transaction_id", command.transactionId)
                put("command_type", command.commandType?.value)
                put("status", result.status.value)
                result.data?.let { put("data", Base64.getEncoder().encodeToString(it)) }
                result.error?.let { put("error", it) }
                result.errorCode?.let { put("error_code", it) }
            }
            sendEncrypted(response)
        } catch (e: JSONException) {
            Log.e(TAG, "Failed to send command response", e)
        }
    }

    fun isConnected(): Boolean = connected && sessionReady
    fun getDeviceUUID(): String = agentDeviceUuid

    override fun onDestroy() {
        super.onDestroy()
        stopConnection()
        messageExecutor.shutdown()
        commandExecutor.shutdown()
        try {
            if (!messageExecutor.awaitTermination(5, TimeUnit.SECONDS)) {
                messageExecutor.shutdownNow()
            }
            if (!commandExecutor.awaitTermination(5, TimeUnit.SECONDS)) {
                commandExecutor.shutdownNow()
            }
        } catch (_: InterruptedException) {
            messageExecutor.shutdownNow()
            commandExecutor.shutdownNow()
        }
    }

    companion object {
        const val ACTION_CONNECTION_STATE = "com.remoteagent.CONNECTION_STATE"
        const val EXTRA_CONNECTED = "connected"
        const val EXTRA_SERVER_BASE_URL = "server_base_url"
        const val EXTRA_SERVER_DEVICE_ID = "server_device_id"

        private const val TAG = "AgentService"
        private const val HEARTBEAT_INTERVAL = 30000L
        private const val RECONNECT_DELAY = 5000L
    }
}
