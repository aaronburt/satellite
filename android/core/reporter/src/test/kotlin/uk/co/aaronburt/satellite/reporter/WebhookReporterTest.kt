package uk.co.aaronburt.satellite.reporter

import android.content.Intent
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
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest
import java.net.ServerSocket
import java.util.concurrent.TimeUnit

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
}
