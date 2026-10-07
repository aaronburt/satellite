package dev.satelliteandroid.feature.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import dev.satelliteandroid.datastore.SettingsRepository
import dev.satelliteandroid.model.BrokerSettings
import dev.satelliteandroid.model.ThemePreference
import dev.satelliteandroid.model.Transport
import dev.satelliteandroid.model.UpdateInterval
import dev.satelliteandroid.model.UpdateMode
import dev.satelliteandroid.model.WebhookSettings
import dev.satelliteandroid.reporter.ReporterCoordinator
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
) : ViewModel() {

    private val _uiState = MutableStateFlow(SettingsUiState())
    val uiState: StateFlow<SettingsUiState> = _uiState.asStateFlow()

    init {
        viewModelScope.launch {
            // Generates and persists a random name on first launch.
            val deviceName = settingsRepository.deviceName()
            val themePreference = settingsRepository.themePreference.first()
            val updateMode = settingsRepository.updateMode.first()
            val updateInterval = settingsRepository.updateInterval.first()
            val transport = settingsRepository.transport.first()
            val webhook = settingsRepository.webhookSettings.first()
            val existing = settingsRepository.brokerSettings.first()
            _uiState.update { state ->
                state.copy(
                    deviceName = deviceName,
                    themePreference = themePreference,
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
            _uiState.update { it.copy(testStatus = "Sending…") }
            val ok = coordinator.testNow()
            _uiState.update {
                it.copy(
                    testStatus = if (ok) {
                        "Test sent ✓"
                    } else {
                        "Test failed — check the URL/token and that you're connected"
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

    private companion object {
        const val DEFAULT_PORT = 1883
    }
}
