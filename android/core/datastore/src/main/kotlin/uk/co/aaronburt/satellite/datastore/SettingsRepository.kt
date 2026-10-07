package uk.co.aaronburt.satellite.datastore

import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.model.WebhookSettings
import kotlinx.coroutines.flow.Flow

/**
 * Persisted app configuration.
 */
interface SettingsRepository {

    /** Emits the saved broker settings, or `null` when none have been configured. */
    val brokerSettings: Flow<BrokerSettings?>

    suspend fun saveBrokerSettings(settings: BrokerSettings)

    /** True once either transport has been configured (broker or webhook). */
    suspend fun isConfigured(): Boolean

    /** User-facing device name. Generated randomly on first use. */
    suspend fun deviceName(): String

    /** Device name as a stream, for screens that must react to edits. */
    val deviceNameFlow: Flow<String>

    suspend fun saveDeviceName(name: String)

    /** Selected colour scheme. */
    val themePreference: Flow<ThemePreference>

    suspend fun saveThemePreference(preference: ThemePreference)

    /** How state reaches Home Assistant (periodic vs event-driven). */
    val updateMode: Flow<UpdateMode>

    suspend fun saveUpdateMode(mode: UpdateMode)

    /** Base publish interval. */
    val updateInterval: Flow<UpdateInterval>

    suspend fun saveUpdateInterval(interval: UpdateInterval)

    /** Selected delivery transport. */
    val transport: Flow<Transport>

    suspend fun saveTransport(transport: Transport)

    /**
     * Master switch. When false the app stops publishing, marks itself offline in
     * Home Assistant and shuts the foreground service down — without the user
     * having to remove the app.
     */
    val reportingEnabled: Flow<Boolean>

    suspend fun saveReportingEnabled(enabled: Boolean)

    /** Webhook endpoint, when [Transport.WEBHOOK] is selected. */
    val webhookSettings: Flow<WebhookSettings?>

    suspend fun saveWebhookSettings(settings: WebhookSettings)

    /** Stable identifier for this install, used in MQTT topics. Generated on first use. */
    suspend fun deviceId(): String
}
