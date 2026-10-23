package uk.co.aaronburt.satellite.testing

import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.model.WebhookSettings
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.update

/**
 * In-memory [SettingsRepository]. Every setting is a [MutableStateFlow] that
 * tests can read or drive directly, and the save functions write through to it,
 * so code under test sees its own writes exactly like it would with DataStore.
 */
class FakeSettingsRepository(
    broker: BrokerSettings? = null,
    webhook: WebhookSettings? = null,
    transport: Transport = Transport.MQTT,
    reportingEnabled: Boolean = true,
    enabledControls: Set<String> = emptySet(),
    updateMode: UpdateMode = UpdateMode.PERIODIC,
    updateInterval: UpdateInterval = UpdateInterval.FIVE_MINUTES,
    theme: ThemePreference = ThemePreference.SYSTEM,
    var deviceIdValue: String = DEVICE_ID,
    deviceName: String = DEVICE_NAME,
) : SettingsRepository {

    val brokerState = MutableStateFlow(broker)
    val webhookState = MutableStateFlow(webhook)
    val transportState = MutableStateFlow(transport)
    val reportingState = MutableStateFlow(reportingEnabled)
    val controlsState = MutableStateFlow(enabledControls)
    val updateModeState = MutableStateFlow(updateMode)
    val updateIntervalState = MutableStateFlow(updateInterval)
    val themeState = MutableStateFlow(theme)
    val deviceNameState = MutableStateFlow(deviceName)

    override val brokerSettings: Flow<BrokerSettings?> = brokerState
    override val webhookSettings: Flow<WebhookSettings?> = webhookState
    override val transport: Flow<Transport> = transportState
    override val reportingEnabled: Flow<Boolean> = reportingState
    override val enabledControls: Flow<Set<String>> = controlsState
    override val updateMode: Flow<UpdateMode> = updateModeState
    override val updateInterval: Flow<UpdateInterval> = updateIntervalState
    override val themePreference: Flow<ThemePreference> = themeState
    override val deviceNameFlow: Flow<String> = deviceNameState

    override suspend fun saveBrokerSettings(settings: BrokerSettings) {
        brokerState.value = settings
    }

    override suspend fun saveWebhookSettings(settings: WebhookSettings) {
        webhookState.value = settings
    }

    override suspend fun saveTransport(transport: Transport) {
        transportState.value = transport
    }

    override suspend fun saveReportingEnabled(enabled: Boolean) {
        reportingState.value = enabled
    }

    override suspend fun setControlEnabled(key: String, enabled: Boolean) {
        controlsState.update { if (enabled) it + key else it - key }
    }

    override suspend fun saveUpdateMode(mode: UpdateMode) {
        updateModeState.value = mode
    }

    override suspend fun saveUpdateInterval(interval: UpdateInterval) {
        updateIntervalState.value = interval
    }

    override suspend fun saveThemePreference(preference: ThemePreference) {
        themeState.value = preference
    }

    override suspend fun saveDeviceName(name: String) {
        deviceNameState.value = name
    }

    override suspend fun deviceName(): String = deviceNameState.value

    override suspend fun deviceId(): String = deviceIdValue

    override suspend fun isConfigured(): Boolean =
        brokerState.value != null || !webhookState.value?.url.isNullOrBlank()

    companion object {
        const val DEVICE_ID = "a1b2c3d4"
        const val DEVICE_NAME = "Test Phone"

        val BROKER = BrokerSettings(
            host = "broker.local",
            port = 1883,
            username = "user",
            password = "secret",
            useTls = false,
        )
    }
}
