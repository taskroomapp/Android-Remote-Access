package com.remoteagent.protocol.commands

import android.app.usage.UsageStats
import android.app.usage.UsageStatsManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.os.BatteryManager
import android.os.Build
import android.os.Environment
import android.os.StatFs
import android.provider.Settings
import androidx.core.content.ContextCompat
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import com.remoteagent.util.DeviceEnv
import org.json.JSONArray
import org.json.JSONObject

/**
 * Device metadata commands: info, foreground app, installed apps.
 */
class DeviceInfoCommands(private val context: Context) {

    fun getDeviceInfo(command: DeviceCommand): CommandResult {
        return try {
            val info = JSONObject().apply {
                put(
                    "device_id",
                    Settings.Secure.getString(context.contentResolver, Settings.Secure.ANDROID_ID)
                )
                put("model", Build.MODEL)
                put("manufacturer", Build.MANUFACTURER)
                put("android_version", DeviceEnv.androidVersionLabel())
                put("sdk_version", Build.VERSION.SDK_INT)
                put("build_number", Build.DISPLAY)
            }

            val batteryIntent = ContextCompat.registerReceiver(
                context,
                null,
                IntentFilter(Intent.ACTION_BATTERY_CHANGED),
                ContextCompat.RECEIVER_NOT_EXPORTED
            )
            if (batteryIntent != null) {
                val level = batteryIntent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1)
                val scale = batteryIntent.getIntExtra(BatteryManager.EXTRA_SCALE, -1)
                val status = batteryIntent.getIntExtra(BatteryManager.EXTRA_STATUS, -1)
                if (level >= 0 && scale > 0) {
                    info.put("battery_level", ((level / scale.toFloat()) * 100).toInt())
                }
                info.put("battery_status", batteryStatus(status))
            }

            val stat = StatFs(Environment.getDataDirectory().path)
            info.put("storage_total", stat.blockCountLong * stat.blockSizeLong)
            info.put("storage_available", stat.availableBlocksLong * stat.blockSizeLong)

            CommandResult.success(info.toString())
        } catch (e: Exception) {
            CommandResult.failed("Failed to get device info: ${e.message}")
        }
    }

    fun getForegroundApp(command: DeviceCommand): CommandResult {
        return try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
                val usm = context.getSystemService(Context.USAGE_STATS_SERVICE) as? UsageStatsManager
                if (usm != null) {
                    val time = System.currentTimeMillis()
                    val appList = usm.queryUsageStats(
                        UsageStatsManager.INTERVAL_DAILY,
                        time - 1000 * 1000,
                        time
                    )

                    if (!appList.isNullOrEmpty()) {
                        var recent: UsageStats? = null
                        for (usageStats in appList) {
                            if (recent == null || usageStats.lastTimeUsed > recent.lastTimeUsed) {
                                recent = usageStats
                            }
                        }

                        if (recent != null) {
                            val result = JSONObject().apply {
                                put("package_name", recent.packageName)
                                put("last_used", recent.lastTimeUsed)
                            }
                            val pm = context.packageManager
                            try {
                                val appInfo = pm.getApplicationInfo(recent.packageName, 0)
                                result.put("app_name", pm.getApplicationLabel(appInfo).toString())
                            } catch (_: PackageManager.NameNotFoundException) {
                                result.put("app_name", recent.packageName)
                            }
                            return CommandResult.success(result.toString())
                        }
                    }
                }
            }

            CommandResult.failed(
                "USAGE_STATS_PERMISSION_REQUIRED",
                "Usage stats permission required to get foreground app"
            )
        } catch (e: Exception) {
            CommandResult.failed("Failed to get foreground app: ${e.message}")
        }
    }

    fun getInstalledApps(command: DeviceCommand): CommandResult {
        return try {
            val pm = context.packageManager
            val apps = pm.getInstalledApplications(PackageManager.GET_META_DATA)
            val result = JSONArray()
            for (app in apps) {
                result.put(JSONObject().apply {
                    put("package_name", app.packageName)
                    put("app_name", pm.getApplicationLabel(app).toString())
                    put("system_app", (app.flags and ApplicationInfo.FLAG_SYSTEM) != 0)
                })
            }
            CommandResult.success(
                JSONObject().apply {
                    put("apps", result)
                    put("count", result.length())
                }.toString()
            )
        } catch (e: Exception) {
            CommandResult.failed("Failed to list installed apps: ${e.message}")
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.GET_DEVICE_INFO] = CommandExecutor { getDeviceInfo(it) }
        into[DeviceCommand.CommandType.GET_FOREGROUND_APP] = CommandExecutor { getForegroundApp(it) }
        into[DeviceCommand.CommandType.GET_INSTALLED_APPS] = CommandExecutor { getInstalledApps(it) }
    }

    private fun batteryStatus(status: Int): String = when (status) {
        BatteryManager.BATTERY_STATUS_CHARGING -> "charging"
        BatteryManager.BATTERY_STATUS_DISCHARGING -> "discharging"
        BatteryManager.BATTERY_STATUS_FULL -> "full"
        BatteryManager.BATTERY_STATUS_NOT_CHARGING -> "not_charging"
        else -> "unknown"
    }
}
