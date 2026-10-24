package uk.co.aaronburt.satellite.reporter

import android.content.Intent
import java.net.ServerSocket
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import mockwebserver3.RecordedRequest
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_VOLUME
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.model.WebhookSettings
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.DEVICE_ID
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.DEVICE_NAME
import uk.co.aaronburt.satellite.testing.TestDispatchers
import uk.co.aaronburt.satellite.testing.idleMainLooper
import uk.co.aaronburt.satellite.testing.setBattery
import uk.co.aaronburt.satellite.testing.settle

@RunWith(RobolectricTestRunner::class)
class WebhookReporterTest {

    private val context = RuntimeEnvironment.getApplication()
    private val audio = AudioControls(context)
    private val status = ReporterStatus()
    private val server = MockWebServer()
    private lateinit var settings: FakeSettingsRepository

    @Before
    fun setUp() {
        server.start()
        context.setBattery(level = 73)
        settings = FakeSettingsRepository(webhook = WebhookSettings(url("/hook"), bearerToken = "t0ken"))
    }

    @After
    fun tearDown() {
        server.close()
    }

    private fun url(path: String) = server.url(path).toString()

    private fun TestScope.reporter() = WebhookReporter(
        settingsRepository = settings,
        telemetry = DeviceStateReader(context, audio),
        status = status,
        dispatchers = TestDispatchers(StandardTestDispatcher(testScheduler)),
        context = context,
        appScope = backgroundScope,
    ).also { r ->
        r.jitter = { 0.0 } // delays are exactly half the backoff: 1s, 2s, 4s, 8s
        r.clock = { testScheduler.currentTime + EPOCH }
    }

    private fun status(code: Int, retryAfter: String? = null) = server.enqueue(
        MockResponse.Builder().code(code).apply {
            if (retryAfter !=
                null
            ) {
                addHeader("Retry-After", retryAfter)
            }
        }.build(),
    )

    private fun ok() = server.enqueue(MockResponse.Builder().code(200).build())

    private fun takeRequest(): RecordedRequest? = server.takeRequest(200, TimeUnit.MILLISECONDS)

    private fun RecordedRequest.json() = JSONObject(body!!.utf8())

    @Test
    fun `posts a JSON snapshot with the bearer token`() = runTest {
        ok()

        val sent = reporter().refresh()

        assertTrue(sent)
        val request = takeRequest()!!
        assertEquals("POST", request.method)
        assertEquals("/hook", request.url.encodedPath)
        assertEquals("application/json", request.headers["Content-Type"])
        assertEquals("Bearer t0ken", request.headers["Authorization"])

        val json = request.json()
        assertEquals(DEVICE_ID, json.getString("device_id"))
        assertEquals(DEVICE_NAME, json.getString("device_name"))
        assertEquals("73", json.getJSONObject("state").getString("battery"))
        assertTrue(json.getLong("timestamp") > 0)
        assertNotNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `sends no Authorization header without a token`() = runTest {
        settings.webhookState.value = WebhookSettings(url("/hook"), bearerToken = null)
        ok()

        assertTrue(reporter().refresh())
        assertNull(takeRequest()!!.headers["Authorization"])
    }

    @Test
    fun `does nothing without a webhook URL`() = runTest {
        settings.webhookState.value = null
        assertFalse(reporter().refresh())

        settings.webhookState.value = WebhookSettings("   ")
        assertFalse(reporter().refresh())

        assertEquals(0, server.requestCount)
        assertNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `an error response counts as not sent`() = runTest {
        server.enqueue(MockResponse.Builder().code(500).build())

        assertFalse(reporter().refresh())
        assertNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `an unreachable endpoint counts as not sent`() = runTest {
        val closedPort = ServerSocket(0).use { it.localPort }
        settings.webhookState.value = WebhookSettings("http://127.0.0.1:$closedPort/hook")

        assertFalse(reporter().refresh())
    }

    @Test
    fun `an invalid URL counts as not sent`() = runTest {
        settings.webhookState.value = WebhookSettings("not a url")

        assertFalse(reporter().refresh())
    }

    @Test
    fun `controls are only included once enabled`() = runTest {
        ok()
        ok()
        val reporter = reporter()

        reporter.refresh()
        assertFalse(takeRequest()!!.json().getJSONObject("state").has(CONTROL_VOLUME))

        settings.setControlEnabled(CONTROL_VOLUME, true)
        reporter.refresh()
        assertTrue(takeRequest()!!.json().getJSONObject("state").has(CONTROL_VOLUME))
    }

    @Test
    fun `start posts straight away and again whenever the webhook settings change`() = runTest {
        repeat(2) { ok() }
        reporter().start()
        settle()
        assertEquals("/hook", takeRequest()?.url?.encodedPath)

        settings.saveWebhookSettings(WebhookSettings(url("/other")))
        settle()
        assertEquals("/other", takeRequest()?.url?.encodedPath)
    }

    @Test
    fun `start posts on the configured interval`() = runTest {
        settings.updateIntervalState.value = UpdateInterval.THIRTY_SECONDS
        repeat(3) { ok() }
        reporter().start()
        settle()
        takeRequest()

        settle(30_000)
        assertNotNull("first tick", takeRequest())
        settle(30_000)
        assertNotNull("second tick", takeRequest())
    }

    @Test
    fun `event-driven mode posts shortly after a relevant broadcast`() = runTest {
        settings.updateModeState.value = UpdateMode.EVENT_DRIVEN
        repeat(2) { ok() }
        reporter().start()
        settle()
        takeRequest()

        context.sendBroadcast(Intent(Intent.ACTION_POWER_CONNECTED))
        idleMainLooper()
        settle(2_500)

        assertNotNull(takeRequest())
    }

    @Test
    fun `stop ends all posting`() = runTest {
        settings.updateIntervalState.value = UpdateInterval.THIRTY_SECONDS
        settings.updateModeState.value = UpdateMode.EVENT_DRIVEN
        repeat(5) { ok() }
        val reporter = reporter()
        reporter.start()
        settle()
        takeRequest()

        reporter.stop()
        settle(61_000)
        context.sendBroadcast(Intent(Intent.ACTION_POWER_CONNECTED))
        idleMainLooper()
        settle(5_000)
        settings.saveWebhookSettings(WebhookSettings(url("/again")))
        settle()

        assertNull(takeRequest())
    }

    @Test
    fun `start is idempotent`() = runTest {
        repeat(3) { ok() }
        val reporter = reporter()
        reporter.start()
        reporter.start()
        settle()

        assertNotNull(takeRequest())
        assertNull("one post, not one per start()", takeRequest())
    }

    @Test
    fun `server errors are retried with backoff and the original timestamp`() = runTest {
        status(500)
        status(503)
        ok()
        val reporter = reporter()

        assertFalse(reporter.refresh())
        val first = takeRequest()!!.json().getLong("timestamp")

        settle(999)
        assertNull("waits the 1s backoff", takeRequest())
        settle(1)
        val second = takeRequest()!!.json().getLong("timestamp")
        settle(2_000)
        val third = takeRequest()!!.json().getLong("timestamp")

        assertEquals(first, second)
        assertEquals(first, third)
        assertNotNull(status.lastPublishedAtMillis.value)
        settle(60_000)
        assertNull("no more retries after success", takeRequest())
    }

    @Test
    fun `timeouts and rate limits are retried, honouring Retry-After`() = runTest {
        status(408)
        status(429, retryAfter = "20")
        ok()
        reporter().refresh()
        takeRequest()

        settle(1_000)
        assertNotNull("408 retried after 1s", takeRequest())
        settle(19_999)
        assertNull("429 waits for Retry-After", takeRequest())
        settle(1)
        assertNotNull(takeRequest())
        assertNotNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `permanent client errors are not retried`() = runTest {
        for (code in listOf(400, 401, 404)) {
            status(code)
            reporter().refresh()
            takeRequest()
        }
        settle(10 * 60_000)
        assertEquals(3, server.requestCount)
    }

    @Test
    fun `an invalid URL is not retried`() = runTest {
        settings.webhookState.value = WebhookSettings("not a url")
        reporter().refresh()
        settle(10 * 60_000)
        assertEquals(0, server.requestCount)
    }

    @Test
    fun `offline delivery gives up after the attempt limit`() = runTest {
        repeat(10) { status(503) }
        reporter().refresh()

        settle(10 * 60_000)

        assertEquals(WebhookDeliveryPolicy.MAX_ATTEMPTS, server.requestCount)
        assertNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `delivery recovers when an offline endpoint comes back`() = runTest {
        val port = server.port
        server.close()
        val reporter = reporter()

        assertFalse(reporter.refresh())

        val revived = MockWebServer()
        try {
            revived.enqueue(MockResponse.Builder().code(200).build())
            revived.start(port)
            settle(1_000)
            assertNotNull("retried once the endpoint is back", revived.takeRequest(200, TimeUnit.MILLISECONDS))
            assertNotNull(status.lastPublishedAtMillis.value)
        } finally {
            revived.close()
        }
    }

    @Test
    fun `a newer publish replaces a pending retry`() = runTest {
        status(500)
        ok()
        val reporter = reporter()
        reporter.refresh()
        val stale = takeRequest()!!.json().getLong("timestamp")

        settle(500)
        assertTrue(reporter.refresh())
        val fresh = takeRequest()!!.json().getLong("timestamp")
        assertTrue(fresh > stale)

        settle(10 * 60_000)
        assertEquals("stale retry cancelled", 2, server.requestCount)
    }

    @Test
    fun `clearing the webhook or changing its URL cancels retries`() = runTest {
        status(500)
        val reporter = reporter()
        reporter.refresh()
        settings.webhookState.value = null
        settle(10 * 60_000)
        assertEquals(1, server.requestCount)

        settings.webhookState.value = WebhookSettings(url("/hook"))
        status(500)
        reporter.refresh()
        settings.webhookState.value = WebhookSettings(url("/moved"))
        settle(10 * 60_000)
        assertEquals(2, server.requestCount)
    }

    @Test
    fun `a changed token is used on retry`() = runTest {
        status(500)
        ok()
        reporter().refresh()
        takeRequest()
        settings.webhookState.value = WebhookSettings(url("/hook"), bearerToken = "n3w")
        settle(1_000)
        assertEquals("Bearer n3w", takeRequest()!!.headers["Authorization"])
    }

    @Test
    fun `stop cancels a pending retry`() = runTest {
        status(500)
        val reporter = reporter()
        reporter.refresh()
        reporter.stop()
        settle(10 * 60_000)
        assertEquals(1, server.requestCount)
    }

    @Test
    fun `snapshots older than the age limit are dropped`() = runTest {
        repeat(5) { status(429, retryAfter = "300") } // each wait hits the 5 min cap
        reporter().refresh()
        settle(60 * 60_000)
        // attempt 1 at t=0, attempt 2 at exactly 5 min (still in age), attempt 3 would be at 10 min.
        assertEquals(2, server.requestCount)
    }

    private companion object {
        const val EPOCH = 1_800_000_000_000L
    }
}
