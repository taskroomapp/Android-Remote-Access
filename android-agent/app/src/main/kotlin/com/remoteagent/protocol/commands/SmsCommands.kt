package com.remoteagent.protocol.commands

import android.Manifest
import android.content.Context
import android.net.Uri
import android.text.TextUtils
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import org.json.JSONArray
import org.json.JSONObject

/**
 * SMS / message inbox commands.
 */
class SmsCommands(private val context: Context) {

    fun getMessages(command: DeviceCommand): CommandResult {
        if (!context.hasPermission(Manifest.permission.READ_SMS)) {
            return CommandResult.failed("PERMISSION_DENIED", "READ_SMS permission not granted")
        }
        return try {
            val limit = command.getInt("limit", 100)
            val box = command.getString("box", "inbox")
            val address = command.getString("address", "")
            val query = command.getString("query", "")

            val uri = when {
                "sent".equals(box, ignoreCase = true) -> Uri.parse("content://sms/sent")
                "all".equals(box, ignoreCase = true) || "conversations".equals(box, ignoreCase = true) ->
                    Uri.parse("content://sms")
                else -> Uri.parse("content://sms/inbox")
            }

            val selectionParts = ArrayList<String>()
            val selectionArgs = ArrayList<String>()
            if (address.isNotEmpty()) {
                selectionParts.add("address = ?")
                selectionArgs.add(address)
            }
            if (query.isNotEmpty()) {
                selectionParts.add("(body LIKE ? OR address LIKE ?)")
                val like = "%$query%"
                selectionArgs.add(like)
                selectionArgs.add(like)
            }

            val selection = if (selectionParts.isEmpty()) null else TextUtils.join(" AND ", selectionParts)
            val selectionArgArray = if (selectionArgs.isEmpty()) null else selectionArgs.toTypedArray()

            val messages = JSONArray()
            var count = 0
            context.contentResolver.query(uri, null, selection, selectionArgArray, "date DESC")?.use { cursor ->
                while (cursor.moveToNext() && count < limit) {
                    messages.put(JSONObject().apply {
                        put("id", cursor.getString(cursor.getColumnIndexOrThrow("_id")))
                        put("address", cursor.getString(cursor.getColumnIndexOrThrow("address")))
                        put("body", cursor.getString(cursor.getColumnIndexOrThrow("body")))
                        put("date", cursor.getLong(cursor.getColumnIndexOrThrow("date")))
                        put("read", cursor.getInt(cursor.getColumnIndexOrThrow("read")) == 1)
                        put("type", cursor.getInt(cursor.getColumnIndexOrThrow("type")))
                    })
                    count++
                }
            }

            CommandResult.success(
                JSONObject().apply {
                    put("messages", messages)
                    put("count", messages.length())
                    put("box", box)
                }.toString()
            )
        } catch (e: Exception) {
            CommandResult.failed("Failed to get SMS messages: ${e.message}")
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.GET_SMS_MESSAGES] = CommandExecutor { getMessages(it) }
    }
}
