package uk.co.aaronburt.satellite.feature.dashboard

import androidx.lifecycle.viewModelScope
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.WebhookSettings
import uk.co.aaronburt.satellite.mqtt.MqttConnectionManager
import uk.co.aaronburt.satellite.reporter.CommandRouter
import uk.co.aaronburt.satellite.reporter.ReporterCoordinator
import uk.co.aaronburt.satellite.reporter.ReporterStatus
import uk.co.aaronburt.satellite.reporter.SatelliteReporter
import uk.co.aaronburt.satellite.reporter.WebhookReporter
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.FakeMqttClient
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.BROKER
import uk.co.aaronburt.satellite.testing.MainDispatcherRule
import uk.co.aaronburt.satellite.testing.TestDispatchers
import uk.co.aaronburt.satellite.testing.setBattery
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest

@RunWith(RobolectricTestRunner::class)
class StatusViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val context = RuntimeEnvironment.getApplication()
    private val client = FakeMqttClient()
    private val status = ReporterStatus()
    private val settings = FakeSettingsRepository(broker = BROKER)
    private val viewModels = mutableListOf<StatusViewModel>()

    @After
    fun tearDown() = viewModels.forEach { it.viewModelScope.cancel() }

    private fun TestScope.viewModel(): StatusViewModel {
        context.setBattery(level = 80)
        val audio = AudioControls(context)
        val telemetry = DeviceStateReader(context, audio)
        val connection = MqttConnectionManager(client, settings, backgroundScope)
        val coordinator = ReporterCoordinator(
            settingsRepository = settings,
            mqttReporter = SatelliteReporter(
                client, settings, telemetry, CommandRouter(settings, audio), audio, status, context, backgroundScope,
            ),
            webhookReporter = WebhookReporter(
                settings, telemetry, status, TestDispatchers(main.dispatcher), context, backgroundScope,
            ),
            connectionManager = connection,
            status = status,
            appScope = backgroundScope,
        )
        val vm = StatusViewModel(connection, settings, telemetry, coordinator, status)
        viewModels += vm
        backgroundScope.launch(UnconfinedTestDispatcher(testScheduler)) { vm.uiState.collect {} }
        runCurrent()
        return vm
    }

    @Test
    fun `shows the device, transport, connection and reporting state`() = runTest {
        settings.deviceNameState.value = "Hall tablet"
        val vm = viewModel()
        client.setConnected("tcp://broker.local:1883")
        status.markPublished()
        runCurrent()

        val state = vm.uiState.value
        assertEquals("Hall tablet", state.deviceName)
        assertEquals(Transport.MQTT, state.transport)
        assertEquals("tcp://broker.local:1883", (state.connectionState as ConnectionState.Connected).broker)
        assertTrue(state.reportingEnabled)
        assertNotNull(state.lastPublishedAtMillis)
        assertEquals(EntityCatalog.entities.size, state.entityCount)
        assertTrue(state.configured)
    }

    @Test
    fun `shows live battery highlights and refreshes them`() = runTest {
        val vm = viewModel()
        assertEquals("80%", vm.uiState.value.highlights.first { it.key == "battery" }.value)

        context.setBattery(level = 79)
        advanceTimeBy(15_001)
        runCurrent()

        assertEquals("79%", vm.uiState.value.highlights.first { it.key == "battery" }.value)
    }

    @Test
    fun `not configured without a broker or a webhook`() = runTest {
        settings.brokerState.value = null
        val vm = viewModel()
        assertFalse(vm.uiState.value.configured)

        settings.webhookState.value = WebhookSettings("https://ha.local/hook")
        runCurrent()
        assertTrue(vm.uiState.value.configured)
    }

    @Test
    fun `follows settings changes`() = runTest {
        val vm = viewModel()

        settings.transportState.value = Transport.WEBHOOK
        settings.reportingState.value = false
        runCurrent()

        assertEquals(Transport.WEBHOOK, vm.uiState.value.transport)
        assertFalse(vm.uiState.value.reportingEnabled)
    }

    @Test
    fun `the reporting switch and webhook shortcut save settings`() = runTest {
        val vm = viewModel()

        vm.onReportingChange(false)
        vm.onUseWebhook()
        runCurrent()

        assertFalse(settings.reportingState.value)
        assertEquals(Transport.WEBHOOK, settings.transportState.value)
    }

    @Test
    fun `test button reports success, then clears the message`() = runTest {
        val vm = viewModel()
        // Connect through the coordinator, as the service would.
        settings.reportingState.value = true
        client.setConnected()
        runCurrent()

        vm.onTest()
        runCurrent()
        assertEquals("Test sent ✓", vm.testStatus.value)

        advanceTimeBy(4_001)
        runCurrent()
        assertNull(vm.testStatus.value)
    }

    @Test
    fun `test button reports failure when nothing could be sent`() = runTest {
        val vm = viewModel()

        vm.onTest()
        runCurrent()

        assertEquals("Test failed — check your connection", vm.testStatus.value)
    }
}
