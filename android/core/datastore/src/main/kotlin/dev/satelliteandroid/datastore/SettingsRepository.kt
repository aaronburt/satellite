package dev.satelliteandroid.datastore

import dev.satelliteandroid.model.BrokerSettings
import dev.satelliteandroid.model.ThemePreference
import dev.satelliteandroid.model.Transport
import dev.satelliteandroid.model.UpdateInterval
import dev.satelliteandroid.model.UpdateMode
import dev.satelliteandroid.model.WebhookSettings
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

    /** Webhook endpoint, when [Transport.WEBHOOK] is selected. */
    val webhookSettings: Flow<WebhookSettings?>

    suspend fun saveWebhookSettings(settings: WebhookSettings)

    /** Stable identifier for this install, used in MQTT topics. Generated on first use. */
    suspend fun deviceId(): String
}
