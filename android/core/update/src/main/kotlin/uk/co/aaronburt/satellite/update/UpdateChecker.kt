package uk.co.aaronburt.satellite.update

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.HttpURLConnection
import java.net.URL
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Reads the latest published release from GitHub. Unauthenticated, so it is
 * deliberately called only on an explicit user action (60 requests/hour limit).
 */
@Singleton
class UpdateChecker @Inject constructor() {

    suspend fun latestRelease(): GitHubRelease = withContext(Dispatchers.IO) {
        val connection = (URL(LATEST_RELEASE_URL).openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            connectTimeout = CONNECT_TIMEOUT_MILLIS
            readTimeout = READ_TIMEOUT_MILLIS
            setRequestProperty("Accept", "application/vnd.github+json")
            setRequestProperty("User-Agent", USER_AGENT)
        }

        try {
            val code = connection.responseCode
            check(code == HttpURLConnection.HTTP_OK) { "GitHub returned HTTP $code" }
            val body = connection.inputStream.bufferedReader().use { it.readText() }
            GitHubReleaseParser.parse(body)
        } finally {
            connection.disconnect()
        }
    }

    companion object {
        /** Only this repository is ever consulted. */
        const val REPO = "aaronburt/satellite-android"

        private const val LATEST_RELEASE_URL = "https://api.github.com/repos/$REPO/releases/latest"
        private const val USER_AGENT = "SatelliteAndroid"
        private const val CONNECT_TIMEOUT_MILLIS = 10_000
        private const val READ_TIMEOUT_MILLIS = 15_000
    }
}
