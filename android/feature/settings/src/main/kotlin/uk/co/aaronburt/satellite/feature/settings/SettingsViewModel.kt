package uk.co.aaronburt.satellite.feature.settings

import android.content.Context
import android.content.Intent
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.model.WebhookSettings
import uk.co.aaronburt.satellite.reporter.ReporterCoordinator
import uk.co.aaronburt.satellite.update.UpdateManager
import uk.co.aaronburt.satellite.update.UpdateState

@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val coordinator: ReporterCoordinator,
    private val updateManager: UpdateManager,
    @ApplicationContext private val context: Context
) : ViewModel() {

    private val _uiState = MutableStateFlow(SettingsUiState())
    val uiState: StateFlow<SettingsUiState> = _uiState.asStateFlow()

    init {
        viewModelScope.launch {
            updateManager.state.collect { update ->
                _uiState.update {
                    it.copy(update = update, canInstallUpdates = updateManager.canInstall())
                }
            }
        }

        viewModelScope.launch {
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

    fun onDeviceNameChange(value: String) = _uiState.update {
        it.copy(deviceName = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

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
        _uiState.update {
            it.copy(transport = transport, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
        }
    }

    fun onWebhookUrlChange(value: String) = _uiState.update {
        it.copy(webhookUrl = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

    fun onBearerTokenChange(value: String) = _uiState.update {
        it.copy(bearerToken = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

    fun onHostChange(value: String) = _uiState.update {
        it.copy(host = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

    fun onPortChange(value: String) = _uiState.update {
        it.copy(port = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

    fun onUsernameChange(value: String) = _uiState.update {
        it.copy(username = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

    fun onPasswordChange(value: String) = _uiState.update {
        it.copy(password = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

    fun onUseTlsChange(value: Boolean) = _uiState.update {
        it.copy(useTls = value, saved = false, errors = emptyMap(), feedback = null, testStatus = null)
    }

    fun save() = submit(test = false)

    fun onTest() = submit(test = true)

    private fun submit(test: Boolean) {
        val snapshot = _uiState.value
        if (snapshot.busy || (test && !snapshot.reportingEnabled)) return
        val errors = snapshot.validationErrors()
        if (errors.isNotEmpty()) {
            _uiState.update {
                it.copy(errors = errors, saved = false, feedback = R.string.error_correct_fields, testStatus = null)
            }
            return
        }
        _uiState.update {
            it.copy(busy = true, saved = false, feedback = R.string.saving, testStatus = null)
        }
        viewModelScope.launch {
            try {
                persist(snapshot)
                _uiState.update { it.copy(saved = true, feedback = R.string.saved) }
                if (test) {
                    _uiState.update { it.copy(testStatus = R.string.test_sending) }
                    val ok = coordinator.testNow()
                    _uiState.update {
                        it.copy(testStatus = if (ok) R.string.test_sent else R.string.test_failed)
                    }
                }
            } catch (cancelled: CancellationException) {
                throw cancelled
            } catch (_: Exception) {
                _uiState.update {
                    it.copy(
                        feedback = if (it.saved) R.string.saved else R.string.save_failed,
                        testStatus = if (test) R.string.test_failed else null,
                    )
                }
            } finally {
                _uiState.update { it.copy(busy = false) }
            }
        }
    }

    fun refreshUpdateCapabilities() {
        _uiState.update { it.copy(canInstallUpdates = updateManager.canInstall()) }
    }

    fun onCheckForUpdates() {
        refreshUpdateCapabilities()
        viewModelScope.launch { updateManager.checkAndDownload() }
    }

    fun onInstallUpdate() {
        updateManager.install().onFailure { error ->
            _uiState.update {
                it.copy(update = UpdateState.Failed(error.message ?: context.getString(R.string.install_failed)))
            }
        }
    }

    fun unknownSourcesIntent(): Intent = updateManager.unknownSourcesIntent()

    private suspend fun persist(state: SettingsUiState) {
        settingsRepository.saveDeviceName(state.deviceName.trim())
        if (state.transport == Transport.MQTT) {
            settingsRepository.saveBrokerSettings(
                BrokerSettings(
                    host = state.host.trim(),
                    port = state.port.trim().toInt(),
                    username = state.username.trim().ifBlank { null },
                    password = state.password.ifBlank { null },
                    useTls = state.useTls,
                ),
            )
        } else {
            settingsRepository.saveWebhookSettings(
                WebhookSettings(
                    url = state.webhookUrl.trim(),
                    bearerToken = state.bearerToken.trim().ifBlank { null },
                ),
            )
        }
        settingsRepository.saveTransport(state.transport)
    }

    private fun appVersion(): String = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName
    }.getOrNull() ?: context.getString(R.string.version_unknown)
}
