package com.remoteagent.protocol.commands

import android.Manifest
import android.content.Context
import android.provider.CallLog
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import org.json.JSONArray
import org.json.JSONObject

/**
 * Call log / call history commands.
 */
class CallLogCommands(private val context: Context) {

    fun getCallLogs(command: DeviceCommand): CommandResult {
        if (!context.hasPermission(Manifest.permission.READ_CALL_LOG)) {
            return CommandResult.failed("PERMISSION_DENIED", "READ_CALL_LOG permission not granted")
        }

        return try {
            val callLogs = JSONArray()
            val limit = command.getInt("limit", 100)
            var count = 0

            context.contentResolver.query(
                CallLog.Calls.CONTENT_URI,
                null, null, null,
                "${CallLog.Calls.DATE} DESC"
            )?.use { cursor ->
                while (cursor.moveToNext() && count < limit) {
                    callLogs.put(JSONObject().apply {
                        put("id", cursor.getString(cursor.getColumnIndexOrThrow(CallLog.Calls._ID)))
                        put(
                            "number",
                            cursor.getString(cursor.getColumnIndexOrThrow(CallLog.Calls.NUMBER)) ?: "Unknown"
                        )
                        put(
                            "name",
                            cursor.getString(cursor.getColumnIndexOrThrow(CallLog.Calls.CACHED_NAME)) ?: ""
                        )
                        put(
                            "type",
                            callTypeString(cursor.getInt(cursor.getColumnIndexOrThrow(CallLog.Calls.TYPE)))
                        )
                        put("duration", cursor.getLong(cursor.getColumnIndexOrThrow(CallLog.Calls.DURATION)))
                        put("timestamp", cursor.getLong(cursor.getColumnIndexOrThrow(CallLog.Calls.DATE)))
                    })
                    count++
                }
            }

            CommandResult.success(
                JSONObject().apply {
                    put("calls", callLogs)
                    put("count", callLogs.length())
                }.toString()
            )
        } catch (e: Exception) {
            CommandResult.failed("Failed to get call logs: ${e.message}")
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.GET_CALL_LOGS] = CommandExecutor { getCallLogs(it) }
    }

    private fun callTypeString(type: Int): String = when (type) {
        CallLog.Calls.INCOMING_TYPE -> "incoming"
        CallLog.Calls.OUTGOING_TYPE -> "outgoing"
        CallLog.Calls.MISSED_TYPE -> "missed"
        CallLog.Calls.REJECTED_TYPE -> "rejected"
        CallLog.Calls.BLOCKED_TYPE -> "blocked"
        else -> "unknown"
    }
}
