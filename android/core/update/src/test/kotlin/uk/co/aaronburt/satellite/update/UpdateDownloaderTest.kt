package uk.co.aaronburt.satellite.update

import kotlinx.coroutines.runBlocking
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.tls.HandshakeCertificates
import okhttp3.tls.HeldCertificate
import okio.Buffer
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import java.io.File
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSocketFactory

@RunWith(RobolectricTestRunner::class)
class UpdateDownloaderTest {

    private val context = RuntimeEnvironment.getApplication()
    private val downloader = UpdateDownloader(context)
    private val updates = File(context.filesDir, "updates")
    private val server = MockWebServer()

    private lateinit var defaultFactory: SSLSocketFactory
    private lateinit var defaultVerifier: HostnameVerifier

    @Before
    fun setUp() {
        // The downloader only accepts HTTPS, so serve it with a throwaway
        // certificate that this test (and only this test) trusts.
        val certificate = HeldCertificate.Builder()
            .addSubjectAlternativeName("localhost")
            .addSubjectAlternativeName("127.0.0.1")
            .build()
        server.useHttps(HandshakeCertificates.Builder().heldCertificate(certificate).build().sslSocketFactory())
        server.start()

        val clientTrust = HandshakeCertificates.Builder().addTrustedCertificate(certificate.certificate).build()
        defaultFactory = HttpsURLConnection.getDefaultSSLSocketFactory()
        defaultVerifier = HttpsURLConnection.getDefaultHostnameVerifier()
        HttpsURLConnection.setDefaultSSLSocketFactory(clientTrust.sslSocketFactory())
        HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
    }

    @After
    fun tearDown() {
        HttpsURLConnection.setDefaultSSLSocketFactory(defaultFactory)
        HttpsURLConnection.setDefaultHostnameVerifier(defaultVerifier)
        server.close()
    }

    private val payload = ByteArray(200_000) { (it % 251).toByte() }

    private fun serve(bytes: ByteArray = payload, code: Int = 200) =
        server.enqueue(MockResponse.Builder().code(code).body(Buffer().write(bytes)).build())

    private fun asset(name: String = "satellite.apk", size: Long = payload.size.toLong()) = ReleaseAsset(
        name = name,
        downloadUrl = server.url("/download/$name").toString(),
        size = size,
        sha256 = null,
    )

    private fun download(asset: ReleaseAsset, progress: MutableList<Int> = mutableListOf()) =
        runBlocking { downloader.download(asset) { progress += it } }

    private fun failureOf(block: () -> Unit): Throwable {
        try {
            block()
        } catch (expected: Throwable) {
            return expected
        }
        fail("expected a failure")
        error("unreachable")
    }

    @Test
    fun `downloads the asset into the app's updates folder`() {
        serve()

        val file = download(asset())

        assertEquals(File(updates, "satellite.apk"), file)
        assertArrayEquals(payload, file.readBytes())
        assertEquals("SatelliteAndroid", server.takeRequest().headers["User-Agent"])
    }

    @Test
    fun `reports progress up to 100 percent, never going backwards`() {
        serve()
        val progress = mutableListOf<Int>()

        download(asset(), progress)

        assertTrue(progress.isNotEmpty())
        assertEquals(100, progress.last())
        assertEquals(progress.sorted(), progress)
        assertTrue(progress.all { it in 0..100 })
    }

    @Test
    fun `uses the response length for progress when the asset size is unknown`() {
        serve()
        val progress = mutableListOf<Int>()

        download(asset(size = 0), progress)

        assertEquals(100, progress.last())
    }

    @Test
    fun `replaces an earlier download of the same file`() {
        updates.mkdirs()
        File(updates, "satellite.apk").writeText("old")
        serve()

        val file = download(asset())

        assertArrayEquals(payload, file.readBytes())
    }

    @Test
    fun `refuses plain HTTP`() {
        val error = failureOf {
            download(asset().copy(downloadUrl = "http://example.com/satellite.apk"))
        }

        assertEquals("Refusing a non-HTTPS download", error.message)
        assertEquals(0, server.requestCount)
    }

    @Test
    fun `a path in the asset name cannot escape the updates folder`() {
        serve()

        val file = download(asset(name = "../../shared_prefs/evil.apk"))

        assertEquals(File(updates, "evil.apk"), file)
        assertFalse(File(context.filesDir.parentFile, "shared_prefs/evil.apk").exists())
    }

    @Test
    fun `refuses an asset with no usable name`() {
        for (name in listOf("", "..", "/")) {
            val error = failureOf { download(asset(name = name)) }
            assertEquals(name, "Refusing an unnamed download", error.message)
        }
        assertEquals(0, server.requestCount)
    }

    @Test
    fun `an HTTP error fails and leaves nothing behind`() {
        serve(ByteArray(10), code = 404)

        val error = failureOf { download(asset()) }

        assertEquals("Download failed: HTTP 404", error.message)
        assertFalse(File(updates, "satellite.apk").exists())
        assertFalse(File(updates, "satellite.apk.part").exists())
    }

    @Test
    fun `a connection cut mid-download leaves no partial file`() {
        server.enqueue(
            MockResponse.Builder()
                .body(Buffer().write(payload))
                // Promise twice the bytes we send, so closing the socket is a
                // truncated response rather than a complete one.
                .setHeader("Content-Length", payload.size * 2)
                .onResponseBody(mockwebserver3.SocketEffect.ShutdownConnection)
                .build(),
        )

        failureOf { download(asset()) }

        assertFalse(File(updates, "satellite.apk").exists())
        assertFalse(File(updates, "satellite.apk.part").exists())
    }

    @Test
    fun `a download that differs from the published size is rejected`() {
        serve()

        failureOf { download(asset(size = payload.size + 1L)) }

        assertFalse(File(updates, "satellite.apk").exists())
        assertFalse(File(updates, "satellite.apk.part").exists())
    }
}
