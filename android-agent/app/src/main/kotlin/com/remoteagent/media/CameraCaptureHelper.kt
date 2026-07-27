package com.remoteagent.media

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.graphics.ImageFormat
import android.hardware.camera2.CameraAccessException
import android.hardware.camera2.CameraCaptureSession
import android.hardware.camera2.CameraCharacteristics
import android.hardware.camera2.CameraDevice
import android.hardware.camera2.CameraManager
import android.hardware.camera2.CaptureRequest
import android.hardware.camera2.params.OutputConfiguration
import android.hardware.camera2.params.SessionConfiguration
import android.media.ImageReader
import android.os.Build
import android.os.Handler
import android.os.HandlerThread
import android.util.Log
import android.util.Size
import androidx.core.content.ContextCompat
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executor
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference
import kotlin.math.abs

/**
 * Still JPEG capture via Camera2.
 *
 * On Android 14+, the calling process must hold an active foreground service that
 * includes [android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_CAMERA] (or the
 * activity must be visible). Otherwise [CameraManager.openCamera] throws
 * [SecurityException].
 */
object CameraCaptureHelper {
    private const val TAG = "CameraCaptureHelper"
    /** Prefer 720p — smaller CKX1 frames decrypt reliably on modern devices. */
    private const val TARGET_PIXELS = 1280 * 720
    private const val JPEG_QUALITY: Byte = 75

    @JvmStatic
    @Throws(Exception::class)
    fun captureJpeg(context: Context, useFrontCamera: Boolean): ByteArray {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA)
            != PackageManager.PERMISSION_GRANTED
        ) {
            throw SecurityException("CAMERA permission not granted")
        }

        val manager = context.getSystemService(Context.CAMERA_SERVICE) as? CameraManager
            ?: throw IllegalStateException("Camera service unavailable")

        val cameraId = chooseCameraId(manager, useFrontCamera)
        val characteristics = manager.getCameraCharacteristics(cameraId)
        val size = chooseSize(characteristics)
            ?: throw IllegalStateException("No supported JPEG size")

        val thread = HandlerThread("camera-capture").apply { start() }
        val handler = Handler(thread.looper)
        val reader = ImageReader.newInstance(size.width, size.height, ImageFormat.JPEG, 2)
        val jpegRef = AtomicReference<ByteArray?>()
        val imageLatch = CountDownLatch(1)
        val errorRef = AtomicReference<Exception?>()

        reader.setOnImageAvailableListener({ r ->
            try {
                r.acquireLatestImage()?.use { image ->
                    val buffer = image.planes[0].buffer
                    val bytes = ByteArray(buffer.remaining())
                    buffer.get(bytes)
                    if (bytes.size >= 2 &&
                        (bytes[0].toInt() and 0xff) == 0xff &&
                        (bytes[1].toInt() and 0xff) == 0xd8
                    ) {
                        jpegRef.set(bytes)
                    } else {
                        errorRef.compareAndSet(
                            null,
                            IllegalStateException("Camera returned non-JPEG buffer (${bytes.size} bytes)")
                        )
                    }
                }
            } catch (e: Exception) {
                errorRef.compareAndSet(null, e)
            } finally {
                imageLatch.countDown()
            }
        }, handler)

        val deviceHolder = arrayOfNulls<CameraDevice>(1)
        val openLatch = CountDownLatch(1)

        try {
            manager.openCamera(cameraId, object : CameraDevice.StateCallback() {
                override fun onOpened(camera: CameraDevice) {
                    deviceHolder[0] = camera
                    openLatch.countDown()
                }

                override fun onDisconnected(camera: CameraDevice) {
                    camera.close()
                    errorRef.compareAndSet(null, IllegalStateException("Camera disconnected"))
                    openLatch.countDown()
                }

                override fun onError(camera: CameraDevice, error: Int) {
                    camera.close()
                    errorRef.compareAndSet(
                        null,
                        IllegalStateException(cameraErrorMessage(error))
                    )
                    openLatch.countDown()
                }
            }, handler)
        } catch (e: SecurityException) {
            thread.quitSafely()
            reader.close()
            throw SecurityException(
                "Camera blocked in background on this Android version. " +
                    "Keep the agent connected (notification visible) and retry. " +
                    "Details: ${e.message}",
                e
            )
        }

        if (!openLatch.await(12, TimeUnit.SECONDS)) {
            cleanup(null, reader, thread)
            throw IllegalStateException("Camera open timed out")
        }
        errorRef.get()?.let {
            cleanup(deviceHolder[0], reader, thread)
            throw it
        }

        val camera = deviceHolder[0] ?: run {
            cleanup(null, reader, thread)
            throw IllegalStateException("Camera device is null")
        }

        return try {
            val sessionLatch = CountDownLatch(1)
            val stateCallback = object : CameraCaptureSession.StateCallback() {
                override fun onConfigured(session: CameraCaptureSession) {
                    try {
                        // Longer settle on Android 10+ so AE/AF converge before still.
                        val settleMs = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) 600L else 350L
                        handler.postDelayed({
                            try {
                                val builder = camera.createCaptureRequest(
                                    CameraDevice.TEMPLATE_STILL_CAPTURE
                                )
                                builder.addTarget(reader.surface)
                                applyStillCaptureControls(builder, characteristics, useFrontCamera)
                                session.capture(builder.build(), null, handler)
                            } catch (e: Exception) {
                                errorRef.compareAndSet(null, e)
                                imageLatch.countDown()
                            } finally {
                                sessionLatch.countDown()
                            }
                        }, settleMs)
                    } catch (e: Exception) {
                        errorRef.compareAndSet(null, e)
                        sessionLatch.countDown()
                        imageLatch.countDown()
                    }
                }

                override fun onConfigureFailed(session: CameraCaptureSession) {
                    errorRef.compareAndSet(
                        null,
                        IllegalStateException("Camera session configure failed")
                    )
                    sessionLatch.countDown()
                    imageLatch.countDown()
                }
            }

            createSession(camera, reader, stateCallback, handler)

            if (!sessionLatch.await(12, TimeUnit.SECONDS)) {
                throw IllegalStateException("Camera session timed out")
            }
            if (!imageLatch.await(15, TimeUnit.SECONDS)) {
                throw IllegalStateException("Camera capture timed out")
            }
            errorRef.get()?.let { throw it }

            val jpeg = jpegRef.get()
            if (jpeg == null || jpeg.isEmpty()) {
                throw IllegalStateException("Empty camera capture")
            }
            Log.i(TAG, "Captured JPEG ${jpeg.size} bytes (${size.width}x${size.height})")
            jpeg
        } finally {
            cleanup(camera, reader, thread)
        }
    }

    private fun createSession(
        camera: CameraDevice,
        reader: ImageReader,
        stateCallback: CameraCaptureSession.StateCallback,
        handler: Handler
    ) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val executor = Executor { command -> handler.post(command) }
            val config = SessionConfiguration(
                SessionConfiguration.SESSION_REGULAR,
                listOf(OutputConfiguration(reader.surface)),
                executor,
                stateCallback
            )
            camera.createCaptureSession(config)
        } else {
            @Suppress("DEPRECATION")
            camera.createCaptureSession(listOf(reader.surface), stateCallback, handler)
        }
    }

    private fun applyStillCaptureControls(
        builder: CaptureRequest.Builder,
        characteristics: CameraCharacteristics,
        useFrontCamera: Boolean
    ) {
        builder.set(CaptureRequest.CONTROL_MODE, CaptureRequest.CONTROL_MODE_AUTO)
        builder.set(CaptureRequest.CONTROL_AE_MODE, CaptureRequest.CONTROL_AE_MODE_ON)
        val afModes = characteristics.get(CameraCharacteristics.CONTROL_AF_AVAILABLE_MODES)
        if (afModes != null && afModes.contains(CaptureRequest.CONTROL_AF_MODE_CONTINUOUS_PICTURE)) {
            builder.set(
                CaptureRequest.CONTROL_AF_MODE,
                CaptureRequest.CONTROL_AF_MODE_CONTINUOUS_PICTURE
            )
        }
        builder.set(CaptureRequest.JPEG_QUALITY, JPEG_QUALITY)
        val sensorOrientation =
            characteristics.get(CameraCharacteristics.SENSOR_ORIENTATION) ?: 0
        // Front sensors are typically mirrored relative to back; keep upright JPEG EXIF.
        val jpegOrientation = if (useFrontCamera) {
            (sensorOrientation + 180) % 360
        } else {
            sensorOrientation
        }
        builder.set(CaptureRequest.JPEG_ORIENTATION, jpegOrientation)
    }

    @Throws(CameraAccessException::class)
    private fun chooseCameraId(manager: CameraManager, front: Boolean): String {
        for (id in manager.cameraIdList) {
            val facing = manager.getCameraCharacteristics(id)
                .get(CameraCharacteristics.LENS_FACING)
            if (front && facing == CameraCharacteristics.LENS_FACING_FRONT) return id
            if (!front && facing == CameraCharacteristics.LENS_FACING_BACK) return id
        }
        val ids = manager.cameraIdList
        if (ids.isEmpty()) throw IllegalStateException("No cameras available")
        return ids[0]
    }

    private fun chooseSize(characteristics: CameraCharacteristics): Size? {
        val map = characteristics.get(CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP)
            ?: return null
        val sizes = map.getOutputSizes(ImageFormat.JPEG)
        if (sizes.isNullOrEmpty()) return null
        // Prefer ~720p — full-res stills bloat CKX1 frames and fail decrypt races on modern OEMs.
        return sizes.minByOrNull { abs(it.width.toLong() * it.height - TARGET_PIXELS) }
            ?: sizes.maxByOrNull { it.width.toLong() * it.height }
    }

    private fun cameraErrorMessage(error: Int): String = when (error) {
        CameraDevice.StateCallback.ERROR_CAMERA_IN_USE ->
            "Camera in use by another app"
        CameraDevice.StateCallback.ERROR_MAX_CAMERAS_IN_USE ->
            "Too many cameras open"
        CameraDevice.StateCallback.ERROR_CAMERA_DISABLED ->
            "Camera disabled by policy"
        CameraDevice.StateCallback.ERROR_CAMERA_DEVICE ->
            "Fatal camera device error"
        CameraDevice.StateCallback.ERROR_CAMERA_SERVICE ->
            "Camera service error"
        else -> "Camera error code $error"
    }

    private fun cleanup(camera: CameraDevice?, reader: ImageReader, thread: HandlerThread) {
        try {
            camera?.close()
        } catch (_: Exception) {
        }
        try {
            reader.close()
        } catch (_: Exception) {
        }
        thread.quitSafely()
    }
}
