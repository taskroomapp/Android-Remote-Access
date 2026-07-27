package com.remoteagent.protocol.commands

import android.Manifest
import android.content.Context
import com.remoteagent.media.CameraCaptureHelper
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand

/**
 * Camera still-capture commands.
 */
class CameraCommands(private val context: Context) {

    fun snapshot(command: DeviceCommand): CommandResult {
        if (!context.hasPermission(Manifest.permission.CAMERA)) {
            return CommandResult.failed("PERMISSION_DENIED", "CAMERA permission not granted")
        }
        return try {
            val front = "front".equals(command.getString("camera", "back"), ignoreCase = true)
            val jpeg = CameraCaptureHelper.captureJpeg(context.applicationContext, front)
            CommandResult.success(jpeg)
        } catch (e: SecurityException) {
            CommandResult.failed(
                "CAMERA_BLOCKED",
                e.message ?: "Camera blocked while app is backgrounded on this Android version"
            )
        } catch (e: Exception) {
            CommandResult.failed("Failed to capture camera snapshot: ${e.message}")
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.CAMERA_SNAPSHOT] = CommandExecutor { snapshot(it) }
    }
}
