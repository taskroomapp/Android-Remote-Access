package com.remoteagent.ui

import android.Manifest
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.ServiceConnection
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.provider.Settings
import android.widget.Button
import android.widget.TextView
import android.widget.Toast
import androidx.annotation.RequiresApi
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.google.android.material.textfield.TextInputEditText
import com.remoteagent.AgentApplication
import com.remoteagent.R
import com.remoteagent.config.AgentPreferences
import com.remoteagent.cryptokit.KeyStoreIdentity
import com.remoteagent.network.EnrollmentClient
import com.remoteagent.service.AgentService
import com.remoteagent.util.DeviceEnv
import okhttp3.OkHttpClient
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

class MainActivity : AppCompatActivity() {
    private lateinit var preferences: AgentPreferences
    private lateinit var identity: KeyStoreIdentity
    private lateinit var backgroundExecutor: ExecutorService
    private lateinit var httpClient: OkHttpClient

    private lateinit var statusText: TextView
    private lateinit var deviceUuidText: TextView
    private lateinit var adminUserInput: TextInputEditText
    private lateinit var adminPassInput: TextInputEditText

    private var agentService: AgentService? = null
    private var serviceBound = false
    private val uiHandler = Handler(Looper.getMainLooper())
    private var connectionPollRunnable: Runnable? = null
    private var pendingPermissionCallback: Runnable? = null

    private val connectionReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent?) {
            if (intent?.action != AgentService.ACTION_CONNECTION_STATE) return
            refreshConnectionStatus()
        }
    }

    private val serviceConnection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, service: IBinder) {
            val binder = service as AgentService.LocalBinder
            agentService = binder.getService()
            serviceBound = true
            refreshConnectionStatus()
        }

        override fun onServiceDisconnected(name: ComponentName) {
            serviceBound = false
            agentService = null
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        preferences = AgentPreferences(this)
        identity = KeyStoreIdentity(this)
        backgroundExecutor = Executors.newSingleThreadExecutor()
        httpClient = OkHttpClient.Builder()
            .connectTimeout(20, java.util.concurrent.TimeUnit.SECONDS)
            .readTimeout(20, java.util.concurrent.TimeUnit.SECONDS)
            .writeTimeout(20, java.util.concurrent.TimeUnit.SECONDS)
            .build()

        statusText = findViewById(R.id.statusText)
        deviceUuidText = findViewById(R.id.deviceUuidText)
        adminUserInput = findViewById(R.id.adminUserInput)
        adminPassInput = findViewById(R.id.adminPassInput)
        val connectButton = findViewById<Button>(R.id.connectButton)
        val disconnectButton = findViewById<Button>(R.id.disconnectButton)

        deviceUuidText.text = getString(R.string.device_uuid_label, loadAgentUuid())

        connectButton.setOnClickListener {
            requestOperationalPermissions { enrollAndConnect() }
        }

        disconnectButton.setOnClickListener { stopAgentService() }

        requestOperationalPermissions(null)
        bindAgentService()
    }

    override fun onStart() {
        super.onStart()
        ContextCompat.registerReceiver(
            this,
            connectionReceiver,
            IntentFilter(AgentService.ACTION_CONNECTION_STATE),
            ContextCompat.RECEIVER_NOT_EXPORTED
        )
        startConnectionPolling()
    }

    override fun onStop() {
        super.onStop()
        stopConnectionPolling()
        try {
            unregisterReceiver(connectionReceiver)
        } catch (_: IllegalArgumentException) {}
    }

    override fun onDestroy() {
        super.onDestroy()
        if (serviceBound) {
            unbindService(serviceConnection)
            serviceBound = false
        }
        backgroundExecutor.shutdownNow()
    }

    private fun enrollAndConnect() {
        val username = adminUserInput.text?.toString()?.trim() ?: ""
        val password = adminPassInput.text?.toString() ?: ""
        if (username.isEmpty() || password.isEmpty()) {
            Toast.makeText(this, "Enter username and password to connect", Toast.LENGTH_LONG).show()
            return
        }

        setStatus(getString(R.string.status_enrolling), false)
        backgroundExecutor.execute {
            try {
                val client = EnrollmentClient(
                    httpClient,
                    preferences.getServerBaseUrl(),
                    identity.loadPinnedServerFingerprint()
                )
                val login = client.login(username, password)
                val ckx1 = client.establishAdminCkx1(login.accessToken, login.adminId)
                val agentUuid = identity.deviceUuid()
                val fp = identity.keyFingerprint()
                val deviceId = client.enrollDevice(
                    login.accessToken,
                    ckx1,
                    agentUuid,
                    DeviceEnv.friendlyName(),
                    DeviceEnv.androidVersionLabel(),
                    DeviceEnv.hardwareLabel(),
                    fp,
                    identity.x25519PublicBase64(),
                    identity.ed25519PublicBase64(),
                    fp
                )
                preferences.setServerDeviceId(deviceId)
                runOnUiThread {
                    Toast.makeText(this@MainActivity, "Registered with server", Toast.LENGTH_SHORT).show()
                    startAgentService(deviceId)
                }
            } catch (e: Exception) {
                runOnUiThread {
                    setStatus(getString(R.string.status_disconnected), false)
                    Toast.makeText(this@MainActivity, "Connection failed: ${e.message}", Toast.LENGTH_LONG).show()
                }
            }
        }
    }

    private fun startAgentService(deviceId: String) {
        if (deviceId.isEmpty() || !isValidServerDeviceId(deviceId)) {
            Toast.makeText(this, "Server did not return a valid device ID", Toast.LENGTH_LONG).show()
            setStatus(getString(R.string.status_disconnected), false)
            return
        }

        val intent = Intent(this, AgentService::class.java).apply {
            putExtra(AgentService.EXTRA_SERVER_BASE_URL, preferences.getServerBaseUrl())
            putExtra(AgentService.EXTRA_SERVER_DEVICE_ID, deviceId)
        }
        ContextCompat.startForegroundService(this, intent)
        setStatus(getString(R.string.status_connecting), false)
        bindAgentService()
        startConnectionPolling()
    }

    private fun stopAgentService() {
        if (serviceBound && agentService != null) {
            agentService?.stopConnection()
        }
        stopService(Intent(this, AgentService::class.java))
        setStatus(getString(R.string.status_disconnected), false)
    }

    private fun bindAgentService() {
        bindService(Intent(this, AgentService::class.java), serviceConnection, Context.BIND_AUTO_CREATE)
    }

    private fun isValidServerDeviceId(deviceId: String): Boolean {
        return try {
            java.util.UUID.fromString(deviceId)
            true
        } catch (_: IllegalArgumentException) {
            false
        }
    }

    private fun loadAgentUuid(): String = identity.deviceUuid()

    private fun refreshConnectionStatus() {
        if (agentService != null && agentService!!.isConnected()) {
            setStatus(getString(R.string.status_connected), true)
            stopConnectionPolling()
        } else {
            val current = statusText.text
            when {
                current == getString(R.string.status_connecting) -> setStatus(getString(R.string.status_connecting), false)
                current == getString(R.string.status_enrolling) -> setStatus(getString(R.string.status_enrolling), false)
                else -> setStatus(getString(R.string.status_disconnected), false)
            }
        }
    }

    private fun startConnectionPolling() {
        stopConnectionPolling()
        connectionPollRunnable = object : Runnable {
            override fun run() {
                refreshConnectionStatus()
                uiHandler.postDelayed(this, 1000)
            }
        }
        uiHandler.post(connectionPollRunnable!!)
    }

    private fun stopConnectionPolling() {
        connectionPollRunnable?.let {
            uiHandler.removeCallbacks(it)
            connectionPollRunnable = null
        }
    }

    private fun setStatus(text: String, connected: Boolean) {
        statusText.text = text
        statusText.setTextColor(ContextCompat.getColor(
            this,
            if (connected) R.color.status_ok else R.color.status_error
        ))
    }

    private fun requestOperationalPermissions(onGranted: Runnable?) {
        val needed = mutableListOf<String>().apply {
            addIfMissing(Manifest.permission.CAMERA)
            addIfMissing(Manifest.permission.RECORD_AUDIO)
            addIfMissing(Manifest.permission.READ_CONTACTS)
            addIfMissing(Manifest.permission.READ_CALL_LOG)
            addIfMissing(Manifest.permission.READ_SMS)
            addIfMissing(Manifest.permission.ACCESS_FINE_LOCATION)
            addIfMissing(Manifest.permission.ACCESS_COARSE_LOCATION)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                addIfMissing(Manifest.permission.POST_NOTIFICATIONS)
                addIfMissing(Manifest.permission.READ_MEDIA_IMAGES)
                addIfMissing(Manifest.permission.READ_MEDIA_VIDEO)
                addIfMissing(Manifest.permission.READ_MEDIA_AUDIO)
            } else if (Build.VERSION.SDK_INT <= Build.VERSION_CODES.S_V2) {
                addIfMissing(Manifest.permission.READ_EXTERNAL_STORAGE)
            }
        }

        val toRequest = needed.filter {
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED
        }.toTypedArray()

        if (toRequest.isEmpty()) {
            maybeRequestAllFilesAccess()
            maybeRequestUsageAccess()
            onGranted?.run()
            return
        }

        pendingPermissionCallback = onGranted
        ActivityCompat.requestPermissions(this, toRequest, PERMISSION_REQUEST)
    }

    private fun MutableList<String>.addIfMissing(permission: String) {
        if (!contains(permission)) add(permission)
    }

    /** All-files access unlocks full paths on Android 11+ (needed for non-media files). */
    private fun maybeRequestAllFilesAccess() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) return
        if (Environment.isExternalStorageManager()) return
        try {
            Toast.makeText(
                this,
                "Allow all files access so the panel can list every file type",
                Toast.LENGTH_LONG
            ).show()
            val intent = Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION).apply {
                data = Uri.parse("package:$packageName")
            }
            startActivity(intent)
        } catch (_: Exception) {
            try {
                startActivity(Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION))
            } catch (_: Exception) {
            }
        }
    }

    private fun maybeRequestUsageAccess() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.LOLLIPOP) return
        try {
            val appOps = getSystemService(Context.APP_OPS_SERVICE) as android.app.AppOpsManager
            val mode = appOps.checkOpNoThrow(
                android.app.AppOpsManager.OPSTR_GET_USAGE_STATS,
                android.os.Process.myUid(),
                packageName
            )
            if (mode != android.app.AppOpsManager.MODE_ALLOWED) {
                Toast.makeText(
                    this,
                    "Enable usage access for foreground app detection",
                    Toast.LENGTH_LONG
                ).show()
                startActivity(Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS))
            }
        } catch (_: Exception) {}
    }

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode != PERMISSION_REQUEST) return
        maybeRequestAllFilesAccess()
        maybeRequestUsageAccess()
        pendingPermissionCallback?.run()
        pendingPermissionCallback = null
    }

    companion object {
        private const val PERMISSION_REQUEST = 2001
    }
}