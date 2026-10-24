package uk.co.aaronburt.satellite.reporter

import java.util.concurrent.TimeUnit
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
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
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.WebhookSettings
import uk.co.aaronburt.satellite.mqtt.MqttConnectionManager
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.FakeMqttClient
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.BROKER
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.DEVICE_ID
import uk.co.aaronburt.satellite.testing.TestDispatchers
import uk.co.aaronburt.satellite.testing.setBattery
import uk.co.aaronburt.satellite.testing.settle

/** The real reporters and connection manager, wired as in the app, over a fake broker and a local webhook. */
@RunWith(RobolectricTestRunner::class)
class ReporterCoordinatorTest {

    private val context = RuntimeEnvironment.getApplication()
    private val client = FakeMqttClient()
    private val status = ReporterStatus()
    private val server = MockWebServer()
    private val topics = Topics(DEVICE_ID)
    private lateinit var settings: FakeSettingsRepository

    @Before
    fun setUp() {
        server.start()
        repeat(10) { server.enqueue(MockResponse.Builder().code(200).build()) }
        context.setBattery(level = 64)
        settings = FakeSettingsRepository(
            broker = BROKER,
            webhook = WebhookSettings(server.url("/hook").toString()),
        )
    }

    @After
    fun tearDown() {
        server.close()
    }

    private fun TestScope.coordinator(): ReporterCoordinator {
        val audio = AudioControls(context)
        val telemetry = DeviceStateReader(context, audio)
        return ReporterCoordinator(
            settingsRepository = settings,
            mqttReporter = SatelliteReporter(
                client,
                settings,
                telemetry,
                CommandRouter(settings, audio),
                audio,
                status,
                context,
                backgroundScope,
            ),
            webhookReporter = WebhookReporter(
                settings,
                telemetry,
                status,
                TestDispatchers(StandardTestDispatcher(testScheduler)),
                context,
                backgroundScope,
            ),
            connectionManager = MqttConnectionManager(client, settings, backgroundScope),
            status = status,
            appScope = backgroundScope,
        )
    }

    private fun webhookRequest() = server.takeRequest(200, TimeUnit.MILLISECONDS)

    @Test
    fun `MQTT transport connects and reports over MQTT only`() = runTest {
        coordinator().start()
        settle()

        assertTrue(client.connectionState.value is ConnectionState.Connected)
        assertEquals("online", client.lastPayload(topics.availability))
        assertEquals("64", client.lastPayload(topics.state("battery")))
        assertNull(webhookRequest())
    }

    @Test
    fun `switching to webhook closes MQTT, marks the device offline and posts instead`() = runTest {
        coordinator().start()
        settle()

        settings.saveTransport(Transport.WEBHOOK)
        settle()

        assertEquals("offline", client.lastPayload(topics.availability))
        assertEquals(ConnectionState.Disconnected, client.connectionState.value)
        assertNotNull(webhookRequest())
    }

    @Test
    fun `switching back to MQTT reconnects and stops the webhook`() = runTest {
        coordinator().start()
        settle()
        settings.saveTransport(Transport.WEBHOOK)
        settle()
        webhookRequest()

        settings.saveTransport(Transport.MQTT)
        settle()
        settle(15 * 60 * 1000L)

        assertTrue(client.connectionState.value is ConnectionState.Connected)
        assertEquals("online", client.lastPayload(topics.availability))
        assertNull("the webhook's periodic post must stop", webhookRequest())
    }

    @Test
    fun `turning reporting off stops everything and clears the last-published time`() = runTest {
        coordinator().start()
        settle()
        assertNotNull(status.lastPublishedAtMillis.value)

        settings.saveReportingEnabled(false)
        settle()
        client.clearRecords()
        settle(15 * 60 * 1000L)

        assertEquals(ConnectionState.Disconnected, client.connectionState.value)
        assertTrue(client.published.isEmpty())
        assertNull(webhookRequest())
        assertNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `turning reporting back on resumes`() = runTest {
        coordinator().start()
        settle()
        settings.saveReportingEnabled(false)
        settle()

        settings.saveReportingEnabled(true)
        settle()

        assertTrue(client.connectionState.value is ConnectionState.Connected)
        assertEquals("online", client.lastPayload(topics.availability))
    }

    @Test
    fun `starting with reporting off does not connect`() = runTest {
        settings.reportingState.value = false
        coordinator().start()
        settle()

        assertTrue(client.connects.isEmpty())
    }

    @Test
    fun `test now uses the active transport`() = runTest {
        val coordinator = coordinator()
        coordinator.start()
        settle()
        assertTrue("MQTT, connected", coordinator.testNow())

        settings.saveTransport(Transport.WEBHOOK)
        settle()
        webhookRequest()
        assertTrue("webhook", coordinator.testNow())
        assertNotNull(webhookRequest())

        settings.saveReportingEnabled(false)
        settle()
        settings.saveTransport(Transport.MQTT)
        settle()
        assertFalse("MQTT while reporting is off", coordinator.testNow())
    }

    @Test
    fun `stop shuts everything down and start is idempotent`() = runTest {
        val coordinator = coordinator()
        coordinator.start()
        coordinator.start()
        settle()
        assertEquals(1, client.connects.size)

        coordinator.stop()
        settle()

        assertEquals(ConnectionState.Disconnected, client.connectionState.value)
        assertEquals("offline", client.lastPayload(topics.availability))
    }
}
