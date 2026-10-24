package uk.co.aaronburt.satellite.reporter

import android.app.Application
import android.content.Intent
import android.media.AudioManager
import org.json.JSONObject
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
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_MIC_MUTE
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_VOLUME
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.FakeMqttClient
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.DEVICE_ID
import uk.co.aaronburt.satellite.testing.idleMainLooper
import uk.co.aaronburt.satellite.testing.setBattery
import uk.co.aaronburt.satellite.testing.settle
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest

@RunWith(RobolectricTestRunner::class)
class SatelliteReporterTest {

    private val context: Application = RuntimeEnvironment.getApplication()
    private val audioManager = context.getSystemService(AudioManager::class.java)
    private val client = FakeMqttClient()
    private val audio = AudioControls(context)
    private val telemetry = DeviceStateReader(context, audio)
    private val status = ReporterStatus()
    private val topics = Topics(DEVICE_ID)
    private val settings = FakeSettingsRepository()

    @Before
    fun setUp() {
        context.setBattery(level = 73)
        audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, 0, 0)
    }

    private fun TestScope.startReporter(): SatelliteReporter =
        SatelliteReporter(
            client = client,
            settingsRepository = settings,
            telemetry = telemetry,
            commandRouter = CommandRouter(settings, audio),
            audioControls = audio,
            status = status,
            context = context,
            appScope = backgroundScope,
        ).also {
            it.start()
            settle()
        }

    private fun TestScope.connect() {
        client.setConnected()
        settle()
    }

    private fun lastDiscovery(): JSONObject = JSONObject(client.lastPayload(topics.discovery)!!)

    private fun stateKeysPublished(): Set<String> =
        client.published
            .filter { it.topic.startsWith("${topics.base}/state/") && it.payload.isNotEmpty() }
            .map { it.topic.removePrefix("${topics.base}/state/") }
            .toSet()

    @Test
    fun `on connect publishes discovery, retained, with a week's expiry`() = runTest {
        startReporter()
        connect()

        val discovery = client.publishesTo(topics.discovery).last()
        assertTrue(discovery.retain)
        assertEquals(1, discovery.qos)
        assertEquals(7 * 24 * 60 * 60L, discovery.expirySeconds)

        val components = lastDiscovery().getJSONObject("components")
        EntityCatalog.entities.forEach { assertTrue("missing ${it.key}", components.has(it.key)) }
    }

    @Test
    fun `controls stay out of discovery until enabled, and are first removed with stubs`() = runTest {
        startReporter()
        connect()

        val (stub, full) = client.publishesTo(topics.discovery).map { JSONObject(it.payload) }
        // Home Assistant's removal procedure: a stub with only the platform...
        val stubComponents = stub.getJSONObject("components")
        assertEquals("number", stubComponents.getJSONObject(CONTROL_VOLUME).getString("platform"))
        assertEquals(1, stubComponents.getJSONObject(CONTROL_VOLUME).length())
        // ...then the full config without the control.
        val components = full.getJSONObject("components")
        assertFalse(components.has(CONTROL_VOLUME))
        assertFalse(components.has(CONTROL_MIC_MUTE))
    }

    @Test
    fun `on connect publishes the device state, retained, with an hour's expiry`() = runTest {
        startReporter()
        connect()

        val battery = client.publishesTo(topics.state("battery")).single()
        assertEquals("73", battery.payload)
        assertTrue(battery.retain)
        assertEquals(60 * 60L, battery.expirySeconds)
        assertNotNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `only publishes state for active entities`() = runTest {
        startReporter()
        connect()

        val published = stateKeysPublished()
        assertTrue(published.isNotEmpty())
        assertTrue(
            "published $published, allowed ${EntityCatalog.activeKeys(emptySet())}",
            EntityCatalog.activeKeys(emptySet()).containsAll(published),
        )
        assertFalse(CONTROL_VOLUME in published)
        assertFalse(CONTROL_MIC_MUTE in published)
    }

    @Test
    fun `nothing is published while disconnected`() = runTest {
        startReporter()
        settle(UpdateInterval.FIVE_MINUTES.seconds * 1000 + 1)

        assertTrue(client.published.isEmpty())
        assertNull(status.lastPublishedAtMillis.value)
    }

    @Test
    fun `subscribes to the Home Assistant birth topic and this device's commands`() = runTest {
        startReporter()
        connect()

        assertTrue(Topics.BIRTH_TOPIC in client.subscribeCalls)
        assertTrue(topics.commandWildcard in client.subscribeCalls)
    }

    @Test
    fun `Home Assistant coming online triggers a full republish`() = runTest {
        startReporter()
        connect()
        client.clearRecords()

        client.deliver(Topics.BIRTH_TOPIC, "online")
        settle()

        assertEquals(2, client.publishesTo(topics.discovery).size)
        assertEquals("73", client.lastPayload(topics.state("battery")))
    }

    @Test
    fun `Home Assistant going offline does not republish`() = runTest {
        startReporter()
        connect()
        client.clearRecords()

        client.deliver(Topics.BIRTH_TOPIC, "offline")
        settle()

        assertTrue(client.published.isEmpty())
    }

    @Test
    fun `republishes everything and resubscribes after a reconnect`() = runTest {
        startReporter()
        connect()
        client.dropConnection()
        settle()
        client.clearRecords()

        context.setBattery(level = 40)
        connect()

        assertEquals("40", client.lastPayload(topics.state("battery")))
        assertTrue(client.publishesTo(topics.discovery).isNotEmpty())
        assertTrue(topics.commandWildcard in client.subscribeCalls)
    }

    @Test
    fun `periodic mode republishes state on the configured interval`() = runTest {
        settings.updateIntervalState.value = UpdateInterval.ONE_MINUTE
        startReporter()
        connect()
        client.clearRecords()

        // start and connect already used up 2s of the 60s wait
        context.setBattery(level = 60)
        settle(57_000)
        assertTrue("not yet due", client.published.isEmpty())

        settle(2_000)
        assertEquals("60", client.lastPayload(topics.state("battery")))
        assertTrue("the periodic tick only refreshes state", client.publishesTo(topics.discovery).isEmpty())
    }

    @Test
    fun `the periodic tick skips publishing while disconnected`() = runTest {
        settings.updateIntervalState.value = UpdateInterval.THIRTY_SECONDS
        startReporter()
        connect()
        client.dropConnection()
        settle()
        client.clearRecords()

        settle(31_000)

        assertTrue(client.calls.none { it.startsWith("publish") })
    }

    @Test
    fun `event-driven mode publishes shortly after a relevant broadcast`() = runTest {
        settings.updateModeState.value = UpdateMode.EVENT_DRIVEN
        startReporter()
        connect()
        client.clearRecords()

        context.setBattery(level = 55)
        context.sendBroadcast(Intent(Intent.ACTION_POWER_CONNECTED))
        idleMainLooper()
        settle(1_000)
        assertTrue("debounced", client.published.isEmpty())

        settle(1_500)
        assertEquals("55", client.lastPayload(topics.state("battery")))
    }

    @Test
    fun `periodic mode ignores broadcasts`() = runTest {
        startReporter()
        connect()
        client.clearRecords()

        context.sendBroadcast(Intent(Intent.ACTION_POWER_CONNECTED))
        idleMainLooper()
        settle(5_000)

        assertTrue(client.published.isEmpty())
    }

    @Test
    fun `a volume command is applied when the control is enabled and the new state published`() = runTest {
        settings.controlsState.value = setOf(CONTROL_VOLUME)
        startReporter()
        connect()
        client.clearRecords()

        client.deliver(topics.command("volume"), "100")
        settle()

        assertEquals(
            audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC),
            audioManager.getStreamVolume(AudioManager.STREAM_MUSIC),
        )
        assertEquals("100", client.lastPayload(topics.state(CONTROL_VOLUME)))
    }

    @Test
    fun `commands for a control that is not enabled are ignored`() = runTest {
        startReporter()
        connect()
        client.clearRecords()

        client.deliver(topics.command("volume"), "100")
        settle()

        assertEquals(0, audioManager.getStreamVolume(AudioManager.STREAM_MUSIC))
        assertTrue(client.published.isEmpty())
    }

    @Test
    fun `enabling a control republishes discovery with it and starts publishing its state`() = runTest {
        startReporter()
        connect()
        client.clearRecords()

        settings.setControlEnabled(CONTROL_VOLUME, true)
        settle()

        val volume = lastDiscovery().getJSONObject("components").getJSONObject(CONTROL_VOLUME)
        assertEquals("~/cmd/volume", volume.getString("command_topic"))
        assertNotNull(client.lastPayload(topics.state(CONTROL_VOLUME)))
    }

    @Test
    fun `disabling a control clears its retained state and removes it from discovery`() = runTest {
        settings.controlsState.value = setOf(CONTROL_VOLUME)
        startReporter()
        connect()
        client.clearRecords()

        settings.setControlEnabled(CONTROL_VOLUME, false)
        settle()

        val cleared = client.publishesTo(topics.state(CONTROL_VOLUME)).single()
        assertEquals("", cleared.payload)
        assertTrue("an empty retained message deletes the retained state", cleared.retain)
        assertFalse(lastDiscovery().getJSONObject("components").has(CONTROL_VOLUME))
    }

    @Test
    fun `disabling the microphone control unmutes the microphone`() = runTest {
        settings.controlsState.value = setOf(CONTROL_MIC_MUTE)
        startReporter()
        connect()
        client.deliver(topics.command("mute"), "ON")
        settle()
        assertTrue(audioManager.isMicrophoneMute)

        settings.setControlEnabled(CONTROL_MIC_MUTE, false)
        settle()

        assertFalse(audioManager.isMicrophoneMute)
    }

    @Test
    fun `disabling the microphone control while offline still unmutes the microphone`() = runTest {
        // Regression: the unmute used to be skipped when not connected, leaving
        // the mic muted with no control left to unmute it.
        settings.controlsState.value = setOf(CONTROL_MIC_MUTE)
        startReporter()
        connect()
        client.deliver(topics.command("mute"), "ON")
        settle()
        client.dropConnection()
        settle()

        settings.setControlEnabled(CONTROL_MIC_MUTE, false)
        settle()

        assertFalse(audioManager.isMicrophoneMute)
    }

    @Test
    fun `refresh publishes immediately when connected`() = runTest {
        val reporter = startReporter()
        connect()
        client.clearRecords()

        assertTrue(reporter.refresh())
        assertTrue(client.publishesTo(topics.discovery).isNotEmpty())
        assertEquals("73", client.lastPayload(topics.state("battery")))
    }

    @Test
    fun `refresh reports failure when disconnected`() = runTest {
        val reporter = startReporter()

        assertFalse(reporter.refresh())
        assertTrue(client.published.isEmpty())
    }

    @Test
    fun `refresh reports failure when the broker rejects the state`() = runTest {
        // Regression: the test button said "sent" whenever we were connected,
        // even if nothing reached the broker.
        val reporter = startReporter()
        connect()
        status.reset()
        client.failingTopics = { it.contains("/state/") }

        assertFalse(reporter.refresh())
        assertNull("a failed publish is not a publish", status.lastPublishedAtMillis.value)
    }

    @Test
    fun `stop ends periodic publishing and connection handling`() = runTest {
        settings.updateIntervalState.value = UpdateInterval.THIRTY_SECONDS
        val reporter = startReporter()
        connect()
        reporter.stop()
        client.clearRecords()

        settle(61_000)
        client.dropConnection()
        settle()
        connect()

        assertTrue(client.published.isEmpty())
    }

    @Test
    fun `start is idempotent`() = runTest {
        val reporter = startReporter()
        reporter.start()
        settle()
        connect()

        // One stub + one full discovery per connect, not one pair per start().
        assertEquals(2, client.publishesTo(topics.discovery).size)
    }

    @Test
    fun `can be started again after stop`() = runTest {
        val reporter = startReporter()
        reporter.stop()
        reporter.start()
        settle()
        connect()

        assertEquals("73", client.lastPayload(topics.state("battery")))
    }
}
