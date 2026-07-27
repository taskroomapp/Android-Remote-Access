package com.remoteagent.network

import android.os.Handler
import android.os.Looper
import android.util.Log
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLContext
import javax.net.ssl.X509TrustManager

class ServerConnection(
    private val serverURL: String,
    private val deviceUUID: String,
    sslContext: SSLContext?,
    trustManager: X509TrustManager?,
    private val listener: ConnectionListener?
) {
    interface ConnectionListener {
        fun onConnected()
        fun onDisconnected(reason: String)
        fun onMessage(message: String)
        fun onError(e: Exception)
    }

    private val handler = Handler(Looper.getMainLooper())
    private val okHttpClient: OkHttpClient
    @Volatile
    private var webSocket: WebSocket? = null
    @Volatile
    private var connected = false

    init {
        val builder = OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(0, TimeUnit.MILLISECONDS)
            .pingInterval(30, TimeUnit.SECONDS)
        if (serverURL.startsWith("wss://") && sslContext != null && trustManager != null) {
            builder.sslSocketFactory(sslContext.socketFactory, trustManager)
        }
        okHttpClient = builder.build()
    }

    fun connect() {
        val request = Request.Builder()
            .url(serverURL)
            .addHeader("X-Device-UUID", deviceUUID)
            .build()

        webSocket = okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: okhttp3.Response) {
                Log.i(TAG, "WebSocket connected")
                connected = true
                postToListener { listener?.onConnected() }
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                postToListener { listener?.onMessage(text) }
            }

            override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
                onMessage(webSocket, bytes.utf8())
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                webSocket.close(code, reason)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                Log.i(TAG, "WebSocket closed: $code $reason")
                connected = false
                val closeReason = "$code: $reason"
                postToListener {
                    listener?.onDisconnected(closeReason)
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: okhttp3.Response?) {
                Log.e(TAG, "WebSocket failure", t)
                connected = false
                val error = if (t is Exception) t else Exception(t)
                postToListener {
                    listener?.onError(error)
                    listener?.onDisconnected(error.message ?: "connection failed")
                }
            }
        })
    }

    private fun postToListener(run: () -> Unit) {
        handler.post(run)
    }

    fun disconnect() {
        connected = false
        webSocket?.let {
            try {
                it.close(1000, "Client disconnect")
            } catch (_: Exception) {}
            webSocket = null
        }
    }

    fun send(message: String) {
        if (webSocket != null && connected) {
            if (!webSocket!!.send(message)) {
                Log.w(TAG, "WebSocket send queue rejected message")
            }
        } else {
            Log.w(TAG, "Cannot send - not connected")
        }
    }

    fun isConnected(): Boolean = connected

    companion object {
        private const val TAG = "ServerConnection"
    }
}