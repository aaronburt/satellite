package uk.co.aaronburt.satellite.feature.dashboard

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.mqtt.MqttConnectionManager
import uk.co.aaronburt.satellite.reporter.ReporterCoordinator
import uk.co.aaronburt.satellite.reporter.ReporterStatus
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class StatusViewModel @Inject constructor(
    connectionManager: MqttConnectionManager,
    private val settingsRepository: SettingsRepository,
    private val telemetry: DeviceStateReader,
    private val coordinator: ReporterCoordinator,
    status: ReporterStatus,
) : ViewModel() {

    private val telemetryValues = flow {
        while (true) {
            emit(runCatching { telemetry.read() }.getOrDefault(emptyMap()))
            delay(REFRESH_MILLIS)
        }
    }

    private val _testStatus = MutableStateFlow<String?>(null)
    val testStatus: StateFlow<String?> = _testStatus.asStateFlow()

    private val core = combine(
        settingsRepository.deviceNameFlow,
        settingsRepository.transport,
        connectionManager.connectionState,
        settingsRepository.reportingEnabled,
        status.lastPublishedAtMillis,
    ) { deviceName, transport, connection, enabled, lastPublished ->
        CoreState(deviceName, transport, connection, enabled, lastPublished)
    }

    private val configured = combine(
        settingsRepository.brokerSettings,
        settingsRepository.webhookSettings,
    ) { broker, webhook -> broker != null || webhook != null }

    val uiState: StateFlow<StatusUiState> = combine(
        core,
        configured,
        telemetryValues,
    ) { coreState, isConfigured, values ->
        StatusUiState(
            deviceName = coreState.deviceName,
            transport = coreState.transport,
            connectionState = coreState.connection,
            reportingEnabled = coreState.enabled,
            lastPublishedAtMillis = coreState.lastPublished,
            entityCount = EntityCatalog.entities.size,
            configured = isConfigured,
            highlights = highlights(values),
        )
    }.stateIn(
        scope = viewModelScope,
        started = SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS),
        initialValue = StatusUiState(entityCount = EntityCatalog.entities.size),
    )

    fun onReportingChange(enabled: Boolean) {
        viewModelScope.launch { settingsRepository.saveReportingEnabled(enabled) }
    }

    fun onUseWebhook() {
        viewModelScope.launch { settingsRepository.saveTransport(Transport.WEBHOOK) }
    }

    fun onTest() {
        viewModelScope.launch {
            _testStatus.value = "Sending\u2026"
            val ok = coordinator.testNow()
            _testStatus.value = if (ok) {
                "Test sent \u2713"
            } else {
                "Test failed \u2014 check your connection"
            }
            delay(TEST_STATUS_MILLIS)
            _testStatus.value = null
        }
    }

    private fun highlights(values: Map<String, String>): List<StatusHighlight> = buildList {
        values["battery"]?.let { add(StatusHighlight("battery", "Battery", "$it%")) }
        values["charging"]?.let {
            add(StatusHighlight("charging", "Charging", if (it == "ON") "On" else "Off"))
        }
        val wifi = values["wifi_ssid"]
        if (wifi != null) {
            add(StatusHighlight("wifi_ssid", "Wi-Fi", wifi))
        } else {
            values["network_transport"]?.let {
                add(StatusHighlight("network_transport", "Network", it.displayEnum()))
            }
        }
    }

    private data class CoreState(
        val deviceName: String,
        val transport: Transport,
        val connection: ConnectionState,
        val enabled: Boolean,
        val lastPublished: Long?,
    )

    private companion object {
        const val STOP_TIMEOUT_MILLIS = 5_000L
        const val REFRESH_MILLIS = 15_000L
        const val TEST_STATUS_MILLIS = 4_000L
    }
}

private fun String.displayEnum(): String =
    replaceFirstChar { it.uppercase() }
