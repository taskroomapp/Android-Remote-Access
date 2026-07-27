package com.remoteagent.protocol

enum class Status(val value: String) {
    SUCCESS("success"),
    FAILED("failed"),
    TIMEOUT("timeout"),
    PARTIAL("partial"),
    PENDING("pending")
}

class CommandResult {
    var status: Status = Status.FAILED
    var data: ByteArray? = null
    var error: String? = null
    var errorCode: String? = null

    constructor()

    constructor(status: Status) {
        this.status = status
    }

    fun getDataAsString(): String? = data?.let { String(it) }

    fun setDataAsString(data: String?) {
        this.data = data?.toByteArray()
    }

    fun isSuccess(): Boolean = status == Status.SUCCESS
    fun isFailed(): Boolean = status == Status.FAILED
    fun isTimeout(): Boolean = status == Status.TIMEOUT

    companion object {
        @JvmStatic
        fun success(data: ByteArray): CommandResult {
            return CommandResult(Status.SUCCESS).apply { this.data = data }
        }

        @JvmStatic
        fun success(data: String): CommandResult {
            return success(data.toByteArray())
        }

        @JvmStatic
        fun success(data: ByteArray, message: String): CommandResult {
            return success(data)
        }

        @JvmStatic
        fun failed(error: String): CommandResult {
            return CommandResult(Status.FAILED).apply { this.error = error }
        }

        @JvmStatic
        fun failed(errorCode: String, error: String): CommandResult {
            return CommandResult(Status.FAILED).apply {
                this.errorCode = errorCode
                this.error = error
            }
        }

        @JvmStatic
        fun timeout(): CommandResult {
            return CommandResult(Status.TIMEOUT).apply { error = "Command execution timed out" }
        }

        @JvmStatic
        fun pending(): CommandResult = CommandResult(Status.PENDING)

        @JvmStatic
        fun partial(data: ByteArray, error: String): CommandResult {
            return CommandResult(Status.PARTIAL).apply {
                this.data = data
                this.error = error
            }
        }
    }
}