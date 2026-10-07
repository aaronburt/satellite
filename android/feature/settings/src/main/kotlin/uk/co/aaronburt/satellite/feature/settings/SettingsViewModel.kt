package uk.co.aaronburt.satellite.feature.settings

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.model.WebhookSettings
import uk.co.aaronburt.satellite.reporter.ReporterCoordinator
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val coordinator: ReporterCoordinator,
    @ApplicationContext private val context: Context,
) : ViewModel() {

    private val _uiState = MutableStateFlow(SettingsUiState())
    val uiState: StateFlow<SettingsUiState> = _uiState.asStateFlow()

    init {
        viewModelScope.launch {
            // Generates and persists a random name on first launch.
            val deviceName = settingsRepository.deviceName()
            val themePreference = settingsRepository.themePreference.first()
            val reportingEnabled = settingsRepository.reportingEnabled.first()
            val updateMode = settingsRepository.updateMode.first()
            val updateInterval = settingsRepository.updateInterval.first()
            val transport = settingsRepository.transport.first()
            val webhook = settingsRepository.webhookSettings.first()
            val existing = settingsRepository.brokerSettings.first()
            _uiState.update { state ->
                state.copy(
                    deviceName = deviceName,
                    themePreference = themePreference,
                    reportingEnabled = reportingEnabled,
                    updateMode = updateMode,
                    updateInterval = updateInterval,
                    transport = transport,
                    webhookUrl = webhook?.url ?: state.webhookUrl,
                    bearerToken = webhook?.bearerToken.orEmpty(),
                    host = existing?.host ?: state.host,
                    port = existing?.port?.toString() ?: state.port,
                    username = existing?.username.orEmpty(),
                    password = existing?.password.orEmpty(),
                    useTls = existing?.useTls ?: state.useTls,
                    appVersion = appVersion(),
                )
            }
        }
    }

    fun onDeviceNameChange(value: String) =
        _uiState.update { it.copy(deviceName = value, saved = false) }

    fun onThemeChange(preference: ThemePreference) {
        _uiState.update { it.copy(themePreference = preference) }
        viewModelScope.launch { settingsRepository.saveThemePreference(preference) }
    }

    fun onReportingChange(enabled: Boolean) {
        _uiState.update { it.copy(reportingEnabled = enabled) }
        viewModelScope.launch { settingsRepository.saveReportingEnabled(enabled) }
    }

    fun onUpdateModeChange(mode: UpdateMode) {
        _uiState.update { it.copy(updateMode = mode) }
        viewModelScope.launch { settingsRepository.saveUpdateMode(mode) }
    }

    fun onUpdateIntervalChange(interval: UpdateInterval) {
        _uiState.update { it.copy(updateInterval = interval) }
        viewModelScope.launch { settingsRepository.saveUpdateInterval(interval) }
    }

    fun onTransportChange(transport: Transport) {
        _uiState.update { it.copy(transport = transport, saved = false) }
        viewModelScope.launch { settingsRepository.saveTransport(transport) }
    }

    fun onWebhookUrlChange(value: String) =
        _uiState.update { it.copy(webhookUrl = value, saved = false) }

    fun onBearerTokenChange(value: String) =
        _uiState.update { it.copy(bearerToken = value, saved = false) }

    fun onHostChange(value: String) = _uiState.update { it.copy(host = value, saved = false) }

    fun onPortChange(value: String) = _uiState.update { it.copy(port = value, saved = false) }

    fun onUsernameChange(value: String) = _uiState.update { it.copy(username = value, saved = false) }

    fun onPasswordChange(value: String) = _uiState.update { it.copy(password = value, saved = false) }

    fun onUseTlsChange(value: Boolean) = _uiState.update { it.copy(useTls = value, saved = false) }

    fun save() {
        viewModelScope.launch {
            persist()
            _uiState.update { it.copy(saved = true) }
        }
    }

    fun onTest() {
        viewModelScope.launch {
            persist()
            _uiState.update { it.copy(testStatus = "Sending\u2026") }
            val ok = coordinator.testNow()
            _uiState.update {
                it.copy(
                    testStatus = if (ok) {
                        "Test sent \u2713"
                    } else {
                        "Test failed \u2014 check the URL/token and that you're connected"
                    },
                )
            }
        }
    }

    private suspend fun persist() {
        val state = _uiState.value

        state.deviceName.trim()
            .takeIf { it.isNotBlank() }
            ?.let { settingsRepository.saveDeviceName(it) }

        settingsRepository.saveBrokerSettings(
            BrokerSettings(
                host = state.host.trim(),
                port = state.port.trim().toIntOrNull() ?: DEFAULT_PORT,
                username = state.username.trim().ifBlank { null },
                password = state.password.ifBlank { null },
                useTls = state.useTls,
            ),
        )

        state.webhookUrl.trim()
            .takeIf { it.isNotBlank() }
            ?.let {
                settingsRepository.saveWebhookSettings(
                    WebhookSettings(
                        url = it,
                        bearerToken = state.bearerToken.trim().ifBlank { null },
                    ),
                )
            }
    }

    private fun appVersion(): String = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName
    }.getOrNull() ?: "unknown"

    private companion object {
        const val DEFAULT_PORT = 1883
    }
}
