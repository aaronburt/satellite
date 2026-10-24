package uk.co.aaronburt.satellite.update

import kotlinx.coroutines.runBlocking
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test

class UpdateCheckerTest {

    private val server = MockWebServer()

    @Before
    fun setUp() = server.start()

    @After
    fun tearDown() = server.close()

    private fun checker() = UpdateChecker(server.url("/repos/aaronburt/satellite/releases/latest").toString())

    private fun respond(code: Int, body: String = "") =
        server.enqueue(MockResponse.Builder().code(code).body(body).build())

    private fun failureOf(block: suspend () -> Unit): Throwable = try {
        runBlocking { block() }
        fail("expected a failure")
        error("unreachable")
    } catch (expected: Throwable) {
        expected
    }

    @Test
    fun `reads the latest release`() = runBlocking {
        respond(
            200,
            """{"tag_name":"v1.2.3","name":"Satellite 1.2.3","html_url":"https://example/r","body":"Notes",
               "assets":[{"name":"satellite.apk","browser_download_url":"https://example/a.apk","size":10}]}""",
        )

        val release = checker().latestRelease()

        assertEquals(10203, release.versionCode)
        assertEquals("Satellite 1.2.3", release.title)
        assertEquals("satellite.apk", release.apk?.name)
    }

    @Test
    fun `asks GitHub for its JSON API with an identifying user agent`() = runBlocking {
        respond(200, """{"tag_name":"v1.0.0"}""")

        checker().latestRelease()

        val request = server.takeRequest()
        assertEquals("GET", request.method)
        assertEquals("/repos/aaronburt/satellite/releases/latest", request.url.encodedPath)
        assertEquals("application/vnd.github+json", request.headers["Accept"])
        assertEquals("SatelliteAndroid", request.headers["User-Agent"])
    }

    @Test
    fun `a rate-limited or missing release is an error naming the status`() {
        for (code in listOf(403, 404, 500)) {
            respond(code, """{"message":"nope"}""")
            val error = failureOf { checker().latestRelease() }
            assertEquals("GitHub returned HTTP $code", error.message)
        }
    }

    @Test
    fun `a malformed response is an error`() {
        respond(200, "<html>not json</html>")
        failureOf { checker().latestRelease() }
    }

    @Test
    fun `the app only ever checks its own repository`() {
        assertEquals("aaronburt/satellite", UpdateChecker.REPO)
    }
}
