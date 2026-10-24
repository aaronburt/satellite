package uk.co.aaronburt.satellite.feature.entities

import android.media.AudioManager
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_MIC_MUTE
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_VOLUME
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.MainDispatcherRule
import uk.co.aaronburt.satellite.testing.RefusingMicrophoneShadow
import uk.co.aaronburt.satellite.testing.TestDispatchers
import uk.co.aaronburt.satellite.testing.setBattery

@RunWith(RobolectricTestRunner::class)
class EntitiesViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val context = RuntimeEnvironment.getApplication()
    private val audioManager = context.getSystemService(AudioManager::class.java)
    private val audio = AudioControls(context)
    private val settings = FakeSettingsRepository()
    private var vm: EntitiesViewModel? = null

    @After
    fun tearDown() {
        vm?.viewModelScope?.cancel()
    }

    private fun TestScope.viewModel(): EntitiesViewModel {
        val created = EntitiesViewModel(
            DeviceStateReader(context, audio),
            audio,
            settings,
            TestDispatchers(main.dispatcher),
        )
        vm = created
        backgroundScope.launch(UnconfinedTestDispatcher(testScheduler)) { created.uiState.collect {} }
        runCurrent()
        return created
    }

    private fun EntitiesUiState.sensor(key: String) = sensors.firstOrNull { it.key == key }
    private fun EntitiesUiState.control(key: String) = controls.first { it.key == key }

    @Test
    fun `sensors show formatted live values, in catalog order, for what can be read`() = runTest {
        context.setBattery(level = 64, temperatureTenths = 315)
        val state = viewModel().uiState.value

        assertEquals(EntityRow("battery", "Battery", "64%"), state.sensor("battery"))
        assertEquals("31.5 °C", state.sensor("battery_temperature")?.value)
        assertEquals("Permission needed", state.sensor("carrier")?.value)
        val order = EntityCatalog.entities.map { it.key }
        assertEquals(state.sensors.map { it.key }, state.sensors.map { it.key }.sortedBy { order.indexOf(it) })
    }

    @Test
    fun `values refresh every 15 seconds`() = runTest {
        context.setBattery(level = 64)
        val vm = viewModel()

        context.setBattery(level = 63)
        advanceTimeBy(14_000)
        runCurrent()
        assertEquals("64%", vm.uiState.value.sensor("battery")?.value)

        advanceTimeBy(1_001)
        runCurrent()
        assertEquals("63%", vm.uiState.value.sensor("battery")?.value)
    }

    @Test
    fun `controls are listed, off by default, with their current values`() = runTest {
        shadowOf(audioManager).setStreamMaxVolume(10)
        audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, 7, 0)
        val state = viewModel().uiState.value

        assertEquals(EntityCatalog.controlEntities.map { it.key }, state.controls.map { it.key })
        assertEquals(ControlRow(CONTROL_VOLUME, "Media volume", "70%", enabled = false), state.control(CONTROL_VOLUME))
        assertEquals("Live", state.control(CONTROL_MIC_MUTE).value)
        assertTrue(state.control(CONTROL_MIC_MUTE).available)
    }

    @Test
    fun `turning the volume control on and off saves it`() = runTest {
        val vm = viewModel()

        vm.onControlToggled(CONTROL_VOLUME, true)
        runCurrent()
        assertEquals(setOf(CONTROL_VOLUME), settings.controlsState.value)
        assertTrue(vm.uiState.value.control(CONTROL_VOLUME).enabled)

        vm.onControlToggled(CONTROL_VOLUME, false)
        runCurrent()
        assertEquals(emptySet<String>(), settings.controlsState.value)
    }

    @Test
    fun `turning the microphone control on checks the device supports it first`() = runTest {
        val vm = viewModel()

        vm.onControlToggled(CONTROL_MIC_MUTE, true)
        runCurrent()

        assertEquals(setOf(CONTROL_MIC_MUTE), settings.controlsState.value)
        assertEquals(true, audio.microphoneMuteSupported.value)
        assertFalse("probing leaves the mic as it was", audioManager.isMicrophoneMute)
    }

    @Test
    @Config(shadows = [RefusingMicrophoneShadow::class])
    fun `the microphone control stays off, and is shown unavailable, where muting is refused`() = runTest {
        val vm = viewModel()

        vm.onControlToggled(CONTROL_MIC_MUTE, true)
        runCurrent()

        assertEquals(emptySet<String>(), settings.controlsState.value)
        assertFalse(vm.uiState.value.control(CONTROL_MIC_MUTE).available)
        assertTrue("volume is unaffected", vm.uiState.value.control(CONTROL_VOLUME).available)
    }

    @Test
    @Config(shadows = [RefusingMicrophoneShadow::class])
    fun `turning the microphone control off never needs the check`() = runTest {
        settings.controlsState.value = setOf(CONTROL_MIC_MUTE)
        val vm = viewModel()

        vm.onControlToggled(CONTROL_MIC_MUTE, false)
        runCurrent()

        assertEquals(emptySet<String>(), settings.controlsState.value)
        assertNull("not probed", audio.microphoneMuteSupported.value)
    }
}
