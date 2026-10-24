package uk.co.aaronburt.satellite.update

import android.content.Context
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * Streams a release APK into app-private storage. Refuses anything that is not
 * HTTPS, and never trusts the caller's URL beyond the release asset it was given.
 */
@Singleton
class UpdateDownloader @Inject constructor(@ApplicationContext private val context: Context) : ApkDownloader {

    override suspend fun download(asset: ReleaseAsset, onProgress: (Int) -> Unit): File = withContext(Dispatchers.IO) {
        require(asset.downloadUrl.startsWith("https://")) {
            "Refusing a non-HTTPS download"
        }

        // The asset name comes from the network: keep only its last path
        // segment so it can never point outside the updates directory.
        val fileName = File(asset.name).name
        require(fileName.isNotBlank() && fileName != "." && fileName != "..") {
            "Refusing an unnamed download"
        }

        val directory = File(context.filesDir, DIRECTORY).apply { mkdirs() }
        val target = File(directory, fileName)
        val partial = File(directory, "$fileName.part")

        val connection = (URL(asset.downloadUrl).openConnection() as HttpURLConnection).apply {
            instanceFollowRedirects = true
            connectTimeout = CONNECT_TIMEOUT_MILLIS
            readTimeout = READ_TIMEOUT_MILLIS
            setRequestProperty("User-Agent", USER_AGENT)
        }

        try {
            val code = connection.responseCode
            check(code in 200..299) { "Download failed: HTTP $code" }

            val total = asset.size.takeIf { it > 0 } ?: connection.contentLengthLong
            connection.inputStream.use { input ->
                partial.outputStream().use { output ->
                    val buffer = ByteArray(BUFFER_BYTES)
                    var copied = 0L
                    while (true) {
                        val read = input.read(buffer)
                        if (read <= 0) break
                        output.write(buffer, 0, read)
                        copied += read
                        if (total > 0) {
                            onProgress(((copied * 100) / total).toInt().coerceIn(0, 100))
                        }
                    }
                    // HttpURLConnection reports a connection that dropped
                    // early as a normal end of stream, so check the length
                    // ourselves: half an APK must never be treated as done.
                    val declared = connection.contentLengthLong
                    check(declared < 0 || copied == declared) {
                        "Download was cut short ($copied of $declared bytes)"
                    }
                    check(asset.size <= 0 || copied == asset.size) {
                        "Download size does not match the release ($copied of ${asset.size} bytes)"
                    }
                }
            }
        } catch (error: Throwable) {
            partial.delete()
            throw error
        } finally {
            connection.disconnect()
        }

        if (target.exists()) target.delete()
        check(partial.renameTo(target)) { "Could not finalise the download" }
        target
    }

    private companion object {
        const val DIRECTORY = "updates"
        const val USER_AGENT = "SatelliteAndroid"
        const val CONNECT_TIMEOUT_MILLIS = 15_000
        const val READ_TIMEOUT_MILLIS = 30_000
        const val BUFFER_BYTES = 64 * 1024
    }
}
