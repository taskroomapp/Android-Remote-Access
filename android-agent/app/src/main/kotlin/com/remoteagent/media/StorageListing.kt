package com.remoteagent.media

import android.content.Context
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import android.webkit.MimeTypeMap
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

/**
 * Cross-version storage listing: filesystem when allowed, MediaStore merge for
 * shared media on scoped-storage devices (Android 10+), including background FGS.
 */
object StorageListing {
    private val sdf = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.US).apply {
        timeZone = TimeZone.getTimeZone("UTC")
    }

    private val SHARED_ROOTS = listOf(
        "DCIM", "Download", "Downloads", "Pictures", "Movies", "Music",
        "Documents", "Alarms", "Notifications", "Podcasts", "Ringtones",
        "Audiobooks", "Recordings"
    )

    fun list(context: Context, rawPath: String): JSONArray {
        val path = normalizePath(rawPath)
        if (path.isEmpty() || path == "/") {
            return listStorageRoots(context)
        }

        val dir = File(path)
        val byPath = LinkedHashMap<String, JSONObject>()

        // 1) Direct filesystem listing when the directory is readable.
        if (dir.isDirectory) {
            val children = dir.listFiles()
            if (children != null) {
                for (child in children) {
                    byPath[child.absolutePath] = toEntry(child)
                }
            }
        }

        // 2) On scoped storage, supplement shared folders from MediaStore so
        //    newly created photos/videos/docs appear even when File.listFiles is empty.
        if (shouldQueryMediaStore(path)) {
            mergeMediaStore(context, path, byPath)
        }

        // 3) If the path itself is a media "collection" root that doesn't exist as a folder,
        //    still return MediaStore hits under that relative prefix.
        if (byPath.isEmpty() && isUnderPrimaryExternal(path)) {
            mergeMediaStore(context, path, byPath)
        }

        val out = JSONArray()
        byPath.values
            .sortedWith(compareByDescending<JSONObject> { it.optBoolean("is_directory") }
                .thenBy { it.optString("name").lowercase(Locale.US) })
            .forEach { out.put(it) }
        return out
    }

    fun listStorageRoots(context: Context): JSONArray {
        val out = JSONArray()
        val seen = HashSet<String>()

        fun addRoot(file: File?, label: String? = null) {
            if (file == null) return
            val abs = file.absolutePath
            if (!seen.add(abs)) return
            if (!file.exists()) return
            out.put(toEntry(file).apply {
                if (!label.isNullOrBlank()) put("label", label)
            })
        }

        val primary = Environment.getExternalStorageDirectory()
        addRoot(primary, "Internal shared storage")

        // App-specific external dirs (always readable without all-files access).
        context.getExternalFilesDirs(null)?.forEach { addRoot(it, "App files") }
        context.getExternalCacheDirs()?.forEach { addRoot(it, "App cache") }
        addRoot(context.filesDir, "App private")

        // Common public folders even if File.listFiles on primary was sparse.
        if (primary != null) {
            for (name in SHARED_ROOTS) {
                addRoot(File(primary, name))
            }
        }

        // Secondary volumes (SD cards): parent of app-specific external dir.
        context.getExternalFilesDirs(null)?.forEach { appDir ->
            val volumeRoot = appDir?.parentFile?.parentFile?.parentFile?.parentFile
            addRoot(volumeRoot, "External volume")
        }

        return out
    }

    private fun shouldQueryMediaStore(path: String): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return false
        if (!isUnderPrimaryExternal(path)) return false
        val primary = Environment.getExternalStorageDirectory()?.absolutePath ?: return false
        val rel = path.removePrefix(primary).trim('/')
        if (rel.isEmpty()) return true
        val top = rel.substringBefore('/')
        return SHARED_ROOTS.any { it.equals(top, ignoreCase = true) } ||
            top.equals("Android", ignoreCase = true)
    }

    private fun isUnderPrimaryExternal(path: String): Boolean {
        val primary = Environment.getExternalStorageDirectory()?.absolutePath ?: return false
        return path == primary || path.startsWith("$primary/")
    }

    private fun mergeMediaStore(context: Context, dirPath: String, into: MutableMap<String, JSONObject>) {
        val primary = Environment.getExternalStorageDirectory()?.absolutePath ?: return
        val relativePrefix = dirPath.removePrefix(primary).trim('/').let { if (it.isEmpty()) "" else "$it/" }

        queryCollection(
            context,
            MediaStore.Files.getContentUri("external"),
            arrayOf(
                MediaStore.Files.FileColumns._ID,
                MediaStore.Files.FileColumns.DISPLAY_NAME,
                MediaStore.Files.FileColumns.DATA,
                MediaStore.Files.FileColumns.SIZE,
                MediaStore.Files.FileColumns.DATE_MODIFIED,
                MediaStore.Files.FileColumns.MIME_TYPE,
                MediaStore.Files.FileColumns.MEDIA_TYPE,
                MediaStore.Files.FileColumns.RELATIVE_PATH
            ),
            relativePrefix,
            into
        )

        // Also ensure image/video/audio collections are covered on OEM forks.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            queryCollection(
                context,
                MediaStore.Images.Media.EXTERNAL_CONTENT_URI,
                arrayOf(
                    MediaStore.Images.Media._ID,
                    MediaStore.Images.Media.DISPLAY_NAME,
                    MediaStore.Images.Media.DATA,
                    MediaStore.Images.Media.SIZE,
                    MediaStore.Images.Media.DATE_MODIFIED,
                    MediaStore.Images.Media.MIME_TYPE,
                    MediaStore.Images.Media.RELATIVE_PATH
                ),
                relativePrefix,
                into
            )
            queryCollection(
                context,
                MediaStore.Video.Media.EXTERNAL_CONTENT_URI,
                arrayOf(
                    MediaStore.Video.Media._ID,
                    MediaStore.Video.Media.DISPLAY_NAME,
                    MediaStore.Video.Media.DATA,
                    MediaStore.Video.Media.SIZE,
                    MediaStore.Video.Media.DATE_MODIFIED,
                    MediaStore.Video.Media.MIME_TYPE,
                    MediaStore.Video.Media.RELATIVE_PATH
                ),
                relativePrefix,
                into
            )
            queryCollection(
                context,
                MediaStore.Audio.Media.EXTERNAL_CONTENT_URI,
                arrayOf(
                    MediaStore.Audio.Media._ID,
                    MediaStore.Audio.Media.DISPLAY_NAME,
                    MediaStore.Audio.Media.DATA,
                    MediaStore.Audio.Media.SIZE,
                    MediaStore.Audio.Media.DATE_MODIFIED,
                    MediaStore.Audio.Media.MIME_TYPE,
                    MediaStore.Audio.Media.RELATIVE_PATH
                ),
                relativePrefix,
                into
            )
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                queryCollection(
                    context,
                    MediaStore.Downloads.EXTERNAL_CONTENT_URI,
                    arrayOf(
                        MediaStore.Downloads._ID,
                        MediaStore.Downloads.DISPLAY_NAME,
                        MediaStore.Downloads.DATA,
                        MediaStore.Downloads.SIZE,
                        MediaStore.Downloads.DATE_MODIFIED,
                        MediaStore.Downloads.MIME_TYPE,
                        MediaStore.Downloads.RELATIVE_PATH
                    ),
                    relativePrefix,
                    into
                )
            }
        }

        // Discover immediate child folders from MediaStore relative paths
        // (filesystem may not list them under scoped storage).
        discoverSubfolders(context, dirPath, relativePrefix, into)
        synthesizeSharedRoots(dirPath, relativePrefix, into)
    }

    private fun discoverSubfolders(
        context: Context,
        dirPath: String,
        relativePrefix: String,
        into: MutableMap<String, JSONObject>
    ) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return
        val kids = HashSet<String>()
        val prefixNorm = relativePrefix.trim('/')
        val like = if (prefixNorm.isEmpty()) "%" else "$prefixNorm/%"
        try {
            context.contentResolver.query(
                MediaStore.Files.getContentUri("external"),
                arrayOf(MediaStore.MediaColumns.RELATIVE_PATH),
                "${MediaStore.MediaColumns.RELATIVE_PATH} LIKE ?",
                arrayOf(like),
                null
            )?.use { cursor ->
                val relIdx = cursor.getColumnIndex(MediaStore.MediaColumns.RELATIVE_PATH)
                if (relIdx < 0) return
                while (cursor.moveToNext()) {
                    val rel = cursor.getString(relIdx)?.trim('/') ?: continue
                    val rest = when {
                        prefixNorm.isEmpty() -> rel
                        rel.equals(prefixNorm, ignoreCase = true) -> continue
                        rel.startsWith("$prefixNorm/", ignoreCase = true) ->
                            rel.substring(prefixNorm.length + 1)
                        else -> continue
                    }
                    val child = rest.substringBefore('/').trim()
                    if (child.isNotEmpty()) kids.add(child)
                }
            }
        } catch (_: Exception) {
        }

        for (name in kids) {
            val folder = File(dirPath, name)
            val key = folder.absolutePath
            if (!into.containsKey(key)) {
                into[key] = JSONObject().apply {
                    put("name", name)
                    put("path", key)
                    put("is_directory", true)
                    put("size", 0)
                    put("permissions", "r-d")
                    put("modified_time", sdf.format(Date()))
                    put("source", "mediastore")
                }
            }
        }
    }

    private fun queryCollection(
        context: Context,
        uri: Uri,
        projection: Array<String>,
        relativePrefix: String,
        into: MutableMap<String, JSONObject>
    ) {
        val resolver = context.contentResolver
        val selection: String?
        val args: Array<String>?
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            selection = "${MediaStore.MediaColumns.RELATIVE_PATH} LIKE ?"
            args = arrayOf(if (relativePrefix.isEmpty()) "%" else "$relativePrefix%")
        } else {
            val primary = Environment.getExternalStorageDirectory()?.absolutePath ?: return
            val absPrefix = if (relativePrefix.isEmpty()) primary else "$primary/$relativePrefix".trimEnd('/')
            selection = "${MediaStore.MediaColumns.DATA} LIKE ?"
            args = arrayOf("$absPrefix/%")
        }

        try {
            resolver.query(uri, projection, selection, args, null)?.use { cursor ->
                val nameIdx = cursor.getColumnIndex(MediaStore.MediaColumns.DISPLAY_NAME)
                val dataIdx = cursor.getColumnIndex(MediaStore.MediaColumns.DATA)
                val sizeIdx = cursor.getColumnIndex(MediaStore.MediaColumns.SIZE)
                val modIdx = cursor.getColumnIndex(MediaStore.MediaColumns.DATE_MODIFIED)
                val mimeIdx = cursor.getColumnIndex(MediaStore.MediaColumns.MIME_TYPE)
                val relIdx = cursor.getColumnIndex(MediaStore.MediaColumns.RELATIVE_PATH)

                while (cursor.moveToNext()) {
                    val name = if (nameIdx >= 0) cursor.getString(nameIdx) else null
                    val data = if (dataIdx >= 0) cursor.getString(dataIdx) else null
                    val rel = if (relIdx >= 0) cursor.getString(relIdx)?.trim('/') ?: "" else ""
                    val filePath = when {
                        !data.isNullOrBlank() -> data
                        name != null && Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q -> {
                            val primary = Environment.getExternalStorageDirectory()?.absolutePath
                                ?: continue
                            val folder = if (rel.isEmpty()) primary else "$primary/$rel"
                            // Only include files that live directly in the requested folder
                            // (not nested deeper than one segment beyond relativePrefix).
                            val relNorm = rel.trim('/')
                            val prefixNorm = relativePrefix.trim('/')
                            val ok = when {
                                prefixNorm.isEmpty() -> !relNorm.contains('/')
                                relNorm == prefixNorm -> true
                                else -> false
                            }
                            if (!ok) continue
                            "$folder/$name"
                        }
                        else -> continue
                    }

                    // Keep only direct children of the browsed directory.
                    val parent = File(filePath).parentFile?.absolutePath?.replace('\\', '/') ?: continue
                    val want = normalizePath(
                        if (relativePrefix.isEmpty()) {
                            Environment.getExternalStorageDirectory()?.absolutePath ?: continue
                        } else {
                            val primary = Environment.getExternalStorageDirectory()?.absolutePath
                                ?: continue
                            "$primary/${relativePrefix.trim('/')}"
                        }
                    )
                    if (normalizePath(parent) != want) continue

                    if (into.containsKey(filePath)) continue
                    val size = if (sizeIdx >= 0) cursor.getLong(sizeIdx) else 0L
                    val modifiedSec = if (modIdx >= 0) cursor.getLong(modIdx) else 0L
                    val mime = if (mimeIdx >= 0) cursor.getString(mimeIdx) else guessMime(name ?: filePath)
                    into[filePath] = JSONObject().apply {
                        put("name", name ?: File(filePath).name)
                        put("path", filePath)
                        put("is_directory", false)
                        put("size", size)
                        put("permissions", "r--")
                        put("mime_type", mime ?: "application/octet-stream")
                        put(
                            "modified_time",
                            sdf.format(Date(if (modifiedSec > 0) modifiedSec * 1000L else System.currentTimeMillis()))
                        )
                        put("source", "mediastore")
                    }
                }
            }
        } catch (_: SecurityException) {
            // Missing READ_MEDIA_* / storage permission — filesystem results still returned.
        } catch (_: Exception) {
        }
    }

    private fun synthesizeSharedRoots(
        dirPath: String,
        relativePrefix: String,
        into: MutableMap<String, JSONObject>
    ) {
        if (relativePrefix.isNotEmpty()) return
        val primary = Environment.getExternalStorageDirectory() ?: return
        if (normalizePath(dirPath) != normalizePath(primary.absolutePath)) return
        for (name in SHARED_ROOTS) {
            val folder = File(primary, name)
            val key = folder.absolutePath
            if (!into.containsKey(key)) {
                into[key] = JSONObject().apply {
                    put("name", name)
                    put("path", key)
                    put("is_directory", true)
                    put("size", 0)
                    put("permissions", "r-d")
                    put("modified_time", sdf.format(Date()))
                    put("source", "virtual")
                }
            }
        }
    }

    private fun toEntry(file: File): JSONObject {
        return JSONObject().apply {
            put("name", file.name.ifEmpty { file.absolutePath })
            put("path", file.absolutePath)
            put("is_directory", file.isDirectory)
            put("size", if (file.isFile) file.length() else 0L)
            put(
                "permissions",
                buildString {
                    append(if (file.canRead()) "r" else "-")
                    append(if (file.canWrite()) "w" else "-")
                    append(if (file.isDirectory) "d" else "-")
                }
            )
            put("modified_time", sdf.format(Date(file.lastModified().coerceAtLeast(0L))))
            if (file.isFile) {
                put("mime_type", guessMime(file.name) ?: "application/octet-stream")
            }
            put("source", "filesystem")
        }
    }

    private fun guessMime(name: String): String? {
        val ext = name.substringAfterLast('.', "").lowercase(Locale.US)
        if (ext.isEmpty()) return null
        return MimeTypeMap.getSingleton().getMimeTypeFromExtension(ext)
    }

    private fun normalizePath(path: String): String {
        var p = path.trim().replace('\\', '/')
        while (p.contains("//")) p = p.replace("//", "/")
        if (p.length > 1 && p.endsWith("/")) p = p.dropLast(1)
        return p
    }
}
