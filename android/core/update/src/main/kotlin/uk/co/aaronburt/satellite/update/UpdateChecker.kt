package uk.co.aaronburt.satellite.update

import java.io.Reader
import java.net.HttpURLConnection
import java.net.URL
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * Reads the latest published release from GitHub. Unauthenticated, so it is
 * deliberately called only on an explicit user action (60 requests/hour limit).
 */
@Singleton
class UpdateChecker internal constructor(private val latestReleaseUrl: String) : ReleaseSource {

    @Inject
    constructor() : this(LATEST_RELEASE_URL)

    override suspend fun latestRelease(): GitHubRelease = withContext(Dispatchers.IO) {
        for (page in 1..MAX_PAGES) {
            val (body, hasNext) = fetchPage(page)
            val selected = GitHubReleaseParser.latestAndroid(body)
            if (selected != null) return@withContext selected
            check(hasNext) { "No Android release with an APK was found" }
        }
        error("No Android release found in the first 1000 releases")
    }

    private fun fetchPage(page: Int): Pair<String, Boolean> {
        val connection = (
            URL(
                "$latestReleaseUrl?per_page=100&page=$page"
            ).openConnection() as HttpURLConnection
            ).apply {
            requestMethod = "GET"
            connectTimeout = CONNECT_TIMEOUT_MILLIS
            readTimeout = READ_TIMEOUT_MILLIS
            setRequestProperty("Accept", "application/vnd.github+json")
            setRequestProperty("User-Agent", USER_AGENT)
        }
        try {
            val code = connection.responseCode
            check(code == HttpURLConnection.HTTP_OK) { "GitHub returned HTTP $code" }
            val body = connection.inputStream.bufferedReader().use(::readBody)
            return body to (connection.getHeaderField("Link")?.contains("rel=\"next\"") == true)
        } finally {
            connection.disconnect()
        }
    }

    private fun readBody(reader: Reader): String {
        val chars = CharArray(MAX_RESPONSE_CHARS + 1)
        var count = 0
        while (count < chars.size) {
            val read = reader.read(chars, count, chars.size - count)
            if (read < 0) break
            count += read
        }
        check(count <= MAX_RESPONSE_CHARS) { "GitHub response is too large" }
        return String(chars, 0, count)
    }

    companion object {
        /** Only this repository is ever consulted. */
        const val REPO = "aaronburt/satellite"

        private const val LATEST_RELEASE_URL = "https://api.github.com/repos/$REPO/releases"
        private const val MAX_PAGES = 10
        private const val MAX_RESPONSE_CHARS = 8 * 1024 * 1024
        private const val USER_AGENT = "SatelliteAndroid"
        private const val CONNECT_TIMEOUT_MILLIS = 10_000
        private const val READ_TIMEOUT_MILLIS = 15_000
    }
}
