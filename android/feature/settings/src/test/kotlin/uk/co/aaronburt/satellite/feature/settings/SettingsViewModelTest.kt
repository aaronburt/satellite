package uk.co.aaronburt.satellite.feature.settings

import android.content.Intent
import androidx.lifecycle.viewModelScope
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
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
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
import uk.co.aaronburt.satellite.testing.MainDispatcherRule
import uk.co.aaronburt.satellite.testing.TestDispatchers
import uk.co.aaronburt.satellite.update.ApkDownloader
import uk.co.aaronburt.satellite.update.ApkInstaller
import uk.co.aaronburt.satellite.update.ApkVerifier
import uk.co.aaronburt.satellite.update.GitHubRelease
import uk.co.aaronburt.satellite.update.InstalledVersion
import uk.co.aaronburt.satellite.update.ReleaseAsset
import uk.co.aaronburt.satellite.update.ReleaseSource
import uk.co.aaronburt.satellite.update.UpdateManager
import uk.co.aaronburt.satellite.update.UpdateState
import kotlinx.coroutines.cancel
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import java.io.File

@RunWith(RobolectricTestRunner::class)
class SettingsViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val context = RuntimeEnvironment.getApplication()
    private val client = FakeMqttClient()
    private var vm: SettingsViewModel? = null

    private val installer = object : ApkInstaller {
        var allowed = false
        var result: Result<Unit> = Result.success(Unit)
        override fun canInstall() = allowed
        override fun unknownSourcesIntent() = Intent("unknown-sources")
        override fun install(apk: File) = result
    }

    private val release = GitHubRelease(
        tag = "v9.0.0",
        versionCode = 90000,
        versionName = "9.0.0",
        title = "v9.0.0",
        notes = "Notes",
        htmlUrl = "https://github.com/aaronburt/satellite/releases/tag/v9.0.0",
        apk = ReleaseAsset("satellite.apk", "https://example/satellite.apk", 1, null),
    )

    private val updateManager = UpdateManager(
        installed = object : InstalledVersion {
            override val name = "0.5.1"
            override val code = 501L
        },
        checker = object : ReleaseSource {
            override suspend fun latestRelease() = release
        },
        downloader = object : ApkDownloader {
            override suspend fun download(asset: ReleaseAsset, onProgress: (Int) -> Unit): File =
                File.createTempFile("update", ".apk")
        },
        verifier = object : ApkVerifier {
            override fun verifySha256(file: File, expected: String?) = true
            override fun signatureMatchesInstalledApp(apk: File) = true
        },
        installer = installer,
    )

    @After
    fun tearDown() {
        vm?.viewModelScope?.cancel()
    }

    private fun TestScope.viewModel(settings: FakeSettingsRepository): SettingsViewModel {
        val audio = AudioControls(context)
        val telemetry = DeviceStateReader(context, audio)
        val status = ReporterStatus()
        val coordinator = ReporterCoordinator(
            settingsRepository = settings,
            mqttReporter = SatelliteReporter(
                client, settings, telemetry, CommandRouter(settings, audio), audio, status, context, backgroundScope,
            ),
            webhookReporter = WebhookReporter(
                settings, telemetry, status, TestDispatchers(main.dispatcher), context, backgroundScope,
            ),
            connectionManager = MqttConnectionManager(client, settings, backgroundScope),
            status = status,
            appScope = backgroundScope,
        )
        return SettingsViewModel(settings, coordinator, updateManager, context).also {
            vm = it
            runCurrent()
        }
    }

    @Test
    fun `loads the saved settings`() = runTest {
        val settings = FakeSettingsRepository(
            broker = BrokerSettings("broker.local", 1884, "user", "pw", useTls = false),
            webhook = WebhookSettings("https://ha.local/hook", "tok"),
            transport = Transport.WEBHOOK,
            reportingEnabled = false,
            updateMode = UpdateMode.EVENT_DRIVEN,
            updateInterval = UpdateInterval.ONE_MINUTE,
            theme = ThemePreference.DARK,
            deviceName = "Hall tablet",
        )

        val state = viewModel(settings).uiState.value

        assertEquals("Hall tablet", state.deviceName)
        assertEquals(ThemePreference.DARK, state.themePreference)
        assertFalse(state.reportingEnabled)
        assertEquals(UpdateMode.EVENT_DRIVEN, state.updateMode)
        assertEquals(UpdateInterval.ONE_MINUTE, state.updateInterval)
        assertEquals(Transport.WEBHOOK, state.transport)
        assertEquals("https://ha.local/hook", state.webhookUrl)
        assertEquals("tok", state.bearerToken)
        assertEquals("broker.local", state.host)
        assertEquals("1884", state.port)
        assertEquals("user", state.username)
        assertEquals("pw", state.password)
        assertFalse(state.useTls)
        assertTrue(state.appVersion.isNotBlank())
        assertFalse(state.saved)
    }

    @Test
    fun `nothing saved yet shows the defaults`() = runTest {
        val state = viewModel(FakeSettingsRepository()).uiState.value

        assertEquals(SettingsUiState.DEFAULT_HOST, state.host)
        assertEquals(SettingsUiState.DEFAULT_PORT, state.port)
        assertEquals(SettingsUiState.DEFAULT_WEBHOOK_URL, state.webhookUrl)
        assertEquals("", state.username)
        assertTrue(state.useTls)
    }

    @Test
    fun `editing a field updates the form and clears the saved mark`() = runTest {
        val vm = viewModel(FakeSettingsRepository())
        vm.save()
        runCurrent()
        assertTrue(vm.uiState.value.saved)

        vm.onHostChange("new.host")
        assertEquals("new.host", vm.uiState.value.host)
        assertFalse(vm.uiState.value.saved)

        vm.onPortChange("1"); vm.onUsernameChange("u"); vm.onPasswordChange("p"); vm.onUseTlsChange(false)
        vm.onDeviceNameChange("n"); vm.onWebhookUrlChange("w"); vm.onBearerTokenChange("b")
        val state = vm.uiState.value
        assertEquals(listOf("1", "u", "p", "n", "w", "b"), listOf(state.port, state.username, state.password, state.deviceName, state.webhookUrl, state.bearerToken))
        assertFalse(state.useTls)
    }

    @Test
    fun `typing does not save until Save is pressed`() = runTest {
        val settings = FakeSettingsRepository()
        val vm = viewModel(settings)

        vm.onHostChange("typed.host")
        runCurrent()

        assertNull(settings.brokerState.value)
    }

    @Test
    fun `save stores trimmed values`() = runTest {
        val settings = FakeSettingsRepository()
        val vm = viewModel(settings)

        vm.onDeviceNameChange("  Kitchen  ")
        vm.onHostChange(" broker.local ")
        vm.onPortChange(" 1883 ")
        vm.onUsernameChange(" user ")
        vm.onPasswordChange(" pass with spaces ")
        vm.onUseTlsChange(false)
        vm.onWebhookUrlChange(" https://ha.local/hook ")
        vm.onBearerTokenChange(" tok ")
        vm.save()
        runCurrent()

        assertEquals("Kitchen", settings.deviceNameState.value)
        assertEquals(
            BrokerSettings("broker.local", 1883, "user", " pass with spaces ", useTls = false),
            settings.brokerState.value,
        )
        assertEquals(WebhookSettings("https://ha.local/hook", "tok"), settings.webhookState.value)
        assertTrue(vm.uiState.value.saved)
    }

    @Test
    fun `blank optional fields are saved as absent`() = runTest {
        val settings = FakeSettingsRepository(deviceName = "Original")
        val vm = viewModel(settings)

        vm.onDeviceNameChange("   ")
        vm.onUsernameChange("  ")
        vm.onPasswordChange("")
        vm.onWebhookUrlChange("  ")
        vm.save()
        runCurrent()

        assertEquals("a blank name keeps the old one", "Original", settings.deviceNameState.value)
        assertNull(settings.brokerState.value!!.username)
        assertNull(settings.brokerState.value!!.password)
        assertNull("a blank webhook URL is not saved", settings.webhookState.value)
    }

    @Test
    fun `an invalid port falls back to 8883`() = runTest {
        val settings = FakeSettingsRepository()
        val vm = viewModel(settings)

        vm.onPortChange("eighty")
        vm.save()
        runCurrent()

        assertEquals(8883, settings.brokerState.value!!.port)
    }

    @Test
    fun `switches and choices are saved straight away`() = runTest {
        val settings = FakeSettingsRepository()
        val vm = viewModel(settings)

        vm.onThemeChange(ThemePreference.LIGHT)
        vm.onReportingChange(false)
        vm.onUpdateModeChange(UpdateMode.EVENT_DRIVEN)
        vm.onUpdateIntervalChange(UpdateInterval.FIFTEEN_MINUTES)
        vm.onTransportChange(Transport.WEBHOOK)
        runCurrent()

        assertEquals(ThemePreference.LIGHT, settings.themeState.value)
        assertFalse(settings.reportingState.value)
        assertEquals(UpdateMode.EVENT_DRIVEN, settings.updateModeState.value)
        assertEquals(UpdateInterval.FIFTEEN_MINUTES, settings.updateIntervalState.value)
        assertEquals(Transport.WEBHOOK, settings.transportState.value)
        val state = vm.uiState.value
        assertEquals(ThemePreference.LIGHT, state.themePreference)
        assertEquals(Transport.WEBHOOK, state.transport)
    }

    @Test
    fun `test saves the form first, then reports the result`() = runTest {
        val settings = FakeSettingsRepository()
        val vm = viewModel(settings)
        vm.onHostChange("broker.local")

        vm.onTest()
        runCurrent()

        assertEquals("broker.local", settings.brokerState.value?.host)
        assertEquals("Test failed — check the URL/token and that you're connected", vm.uiState.value.testStatus)

        client.setConnected()
        vm.onTest()
        runCurrent()
        assertEquals("Test sent ✓", vm.uiState.value.testStatus)
    }

    @Test
    fun `update checks flow through to the screen`() = runTest {
        val vm = viewModel(FakeSettingsRepository())
        assertEquals(UpdateState.Idle, vm.uiState.value.update)

        installer.allowed = true
        vm.onCheckForUpdates()
        runCurrent()

        assertTrue(vm.uiState.value.canInstallUpdates)
        assertEquals(
            UpdateState.Ready("9.0.0", 90000, "Notes", "https://github.com/aaronburt/satellite/releases/tag/v9.0.0"),
            vm.uiState.value.update,
        )
    }

    @Test
    fun `install permission is refreshed on request`() = runTest {
        val vm = viewModel(FakeSettingsRepository())
        assertFalse(vm.uiState.value.canInstallUpdates)

        installer.allowed = true
        vm.refreshUpdateCapabilities()

        assertTrue(vm.uiState.value.canInstallUpdates)
        assertEquals("unknown-sources", vm.unknownSourcesIntent().action)
    }

    @Test
    fun `a failed install is shown`() = runTest {
        val vm = viewModel(FakeSettingsRepository())
        vm.onCheckForUpdates()
        runCurrent()
        installer.result = Result.failure(IllegalStateException("Installer unavailable"))

        vm.onInstallUpdate()

        assertEquals(UpdateState.Failed("Installer unavailable"), vm.uiState.value.update)
    }

    @Test
    fun `installing before anything is downloaded is shown as a failure`() = runTest {
        val vm = viewModel(FakeSettingsRepository())

        vm.onInstallUpdate()

        assertEquals(UpdateState.Failed("No update has been downloaded yet"), vm.uiState.value.update)
    }
}
