package com.remoteagent.protocol.commands

import com.remoteagent.protocol.DeviceCommand
import com.remoteagent.protocol.CommandResult

fun interface CommandExecutor {
    fun execute(command: DeviceCommand): CommandResult
}
