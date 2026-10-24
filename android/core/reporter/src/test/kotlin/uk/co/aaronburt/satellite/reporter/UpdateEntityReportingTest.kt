package uk.co.aaronburt.satellite.reporter

import android.app.Application
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.FakeMqttClient
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.DEVICE_ID
import uk.co.aaronburt.satellite.testing.setBattery
import uk.co.aaronburt.satellite.testing.settle
import uk.co.aaronburt.satellite.update.UpdateEntityPayload
import uk.co.aaronburt.satellite.update.UpdateState
import uk.co.aaronburt.satellite.update.UpdateStatesSource

@RunWith(RobolectricTestRunner::class)
class UpdateEntityReportingTest {

    private val context: Application = RuntimeEnvironment.getApplication()
    private val client = FakeMqttClient()
    private val audio = AudioControls(context)
    private val settings = FakeSettingsRepository()
    private val topics = Topics(DEVICE_ID)

    private class FakeUpdateStates : UpdateStatesSource {
        var state: UpdateState = UpdateState.Idle
        private val flow = MutableSharedFlow<Unit>(extraBufferCapacity = 8)
        override val changes = flow.asSharedFlow()
        override fun read() = UpdateEntityPayload.states("0.5.1", state)
        fun change(to: UpdateState) {
            state = to
            flow.tryEmit(Unit)
        }
    }

    private val updates = FakeUpdateStates()

    @Before
    fun setUp() {
        context.setBattery(level = 73)
    }

    private fun TestScope.startReporter() = SatelliteReporter(
        client = client,
        settingsRepository = settings,
        telemetry = DeviceStateReader(context, audio),
        commandRouter = CommandRouter(settings, audio),
        audioControls = audio,
        status = ReporterStatus(),
        context = context,
        appScope = backgroundScope,
        updateStates = updates,
    ).also {
        it.start()
        client.setConnected()
        settle()
    }

    @Test
    fun `catalog declares exactly the update entities and statuses the payload produces`() {
        val update = EntityCatalog.entities.single { it.key == UpdateEntityPayload.UPDATE_KEY }
        assertEquals("update", update.platform)
        assertEquals("firmware", update.deviceClass)
        assertFalse("the update entity must not be controllable", update.isControl)

        val status = EntityCatalog.entities.single { it.key == UpdateEntityPayload.STATUS_KEY }
        assertEquals(UpdateEntityPayload.STATUSES, status.options)
    }

    @Test
    fun `discovery announces the update entity as an update component`() = runTest {
        startReporter()

        val components = JSONObject(client.lastPayload(topics.discovery)!!).getJSONObject("components")
        val update = components.getJSONObject("app_update")
        assertEquals("update", update.getString("platform"))
        assertEquals(topics.state("app_update"), update.getString("state_topic"))
        assertFalse("no install command in this version", update.has("command_topic"))
    }

    @Test
    fun `publishes the update state, with only the installed version before any check`() = runTest {
        startReporter()

        val json = JSONObject(client.lastPayload(topics.state("app_update"))!!)
        assertEquals("0.5.1", json.getString("installed_version"))
        assertFalse(json.has("latest_version"))
        assertEquals("idle", client.lastPayload(topics.state("update_status")))
    }

    @Test
    fun `republishes as soon as the update state changes`() = runTest {
        startReporter()

        updates.change(UpdateState.Ready("0.6.0", 600, null, "https://example.test/r"))
        settle()

        val json = JSONObject(client.lastPayload(topics.state("app_update"))!!)
        assertEquals("0.6.0", json.getString("latest_version"))
        assertEquals("ready", client.lastPayload(topics.state("update_status")))
    }

    @Test
    fun `does not publish update changes while disconnected`() = runTest {
        startReporter()
        client.dropConnection()
        settle()
        client.clearRecords()

        updates.change(UpdateState.Checking)
        settle()

        assertTrue(client.published.isEmpty())
    }
}
