package com.remoteagent.media

import android.content.Context
import android.media.MediaRecorder
import android.os.Build
import java.io.File
import java.io.FileInputStream

object AudioCaptureHelper {
    @Volatile
    private var activeRecorder: MediaRecorder? = null
    @Volatile
    private var activeFile: File? = null

    @JvmStatic
    @Synchronized
    fun isRecording(): Boolean = activeRecorder != null

    @JvmStatic
    @Synchronized
    @Throws(Exception::class)
    fun startRecording(context: Context) {
        if (activeRecorder != null) {
            throw IllegalStateException("Recording already in progress")
        }
        val file = File.createTempFile("agent_mic_", ".m4a", context.cacheDir)
        val recorder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            MediaRecorder(context)
        } else {
            @Suppress("DEPRECATION")
            MediaRecorder()
        }.apply {
            setAudioSource(MediaRecorder.AudioSource.MIC)
            setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
            setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
            setAudioSamplingRate(44100)
            setAudioEncodingBitRate(128000)
            setOutputFile(file.absolutePath)
            prepare()
            start()
        }
        activeRecorder = recorder
        activeFile = file
    }

    @JvmStatic
    @Synchronized
    @Throws(Exception::class)
    fun stopRecording(): ByteArray {
        val recorder = activeRecorder ?: throw IllegalStateException("No active recording")
        activeRecorder = null
        val file = activeFile
        activeFile = null
        try {
            recorder.stop()
        } finally {
            try {
                recorder.release()
            } catch (_: Exception) {
            }
        }
        if (file == null || !file.exists()) {
            throw IllegalStateException("Recording file missing")
        }
        return readFile(file).also {
            file.delete()
        }
    }

    /** Blocking capture for a fixed duration (legacy / one-shot orders). */
    @JvmStatic
    @Throws(Exception::class)
    fun recordOgg(context: Context, durationSeconds: Int): ByteArray {
        val seconds = durationSeconds.coerceIn(1, 120)
        startRecording(context)
        return try {
            Thread.sleep(seconds * 1000L)
            stopRecording()
        } catch (e: Exception) {
            activeRecorder?.release()
            activeRecorder = null
            activeFile = null
            throw e
        }
    }

    @Throws(Exception::class)
    private fun readFile(file: File): ByteArray {
        FileInputStream(file).use { fis ->
            val data = ByteArray(file.length().toInt())
            val read = fis.read(data)
            if (read != data.size) {
                throw IllegalStateException("Failed to read audio file")
            }
            return data
        }
    }
}