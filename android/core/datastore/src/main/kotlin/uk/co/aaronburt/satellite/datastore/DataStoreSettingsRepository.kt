package uk.co.aaronburt.satellite.datastore

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.core.stringSetPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.model.WebhookSettings
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import java.util.UUID
import javax.inject.Inject

private val Context.dataStore by preferencesDataStore(name = "satellite_settings")

class DataStoreSettingsRepository internal constructor(
    private val dataStore: DataStore<Preferences>,
    private val randomHex: () -> String = { UUID.randomUUID().toString().replace("-", "") },
) : SettingsRepository {

    @Inject
    constructor(context: Context) : this(context.dataStore)

    override val brokerSettings: Flow<BrokerSettings?> = dataStore.data.map { prefs ->
        val host = prefs[Keys.Host] ?: return@map null
        BrokerSettings(
            host = host,
            port = prefs[Keys.Port] ?: DEFAULT_PORT,
            username = prefs[Keys.Username]?.takeIf { it.isNotBlank() },
            password = prefs[Keys.Password]?.takeIf { it.isNotBlank() },
            useTls = prefs[Keys.UseTls] ?: true,
        )
    }

    override suspend fun saveBrokerSettings(settings: BrokerSettings) {
        dataStore.edit { prefs ->
            prefs[Keys.Host] = settings.host
            prefs[Keys.Port] = settings.port
            prefs[Keys.Username] = settings.username.orEmpty()
            prefs[Keys.Password] = settings.password.orEmpty()
            prefs[Keys.UseTls] = settings.useTls
        }
    }

    override suspend fun deviceId(): String =
        getOrCreate(Keys.DeviceId) { randomHex().take(DEVICE_ID_LENGTH) }

    override suspend fun deviceName(): String =
        getOrCreate(Keys.DeviceName) { "Satellite-" + randomHex().take(4).uppercase() }

    /**
     * Returns the stored value, generating and saving one on first use. The
     * check runs inside the edit transaction, so callers racing on first launch
     * (the reporter and the connection manager both ask for the device id) all
     * get the same value instead of each saving their own.
     */
    private suspend fun getOrCreate(
        key: Preferences.Key<String>,
        generate: () -> String,
    ): String {
        dataStore.data.first()[key]?.let { return it }
        val saved = dataStore.edit { prefs ->
            if (prefs[key] == null) prefs[key] = generate()
        }
        return checkNotNull(saved[key])
    }

    override suspend fun saveDeviceName(name: String) {
        dataStore.edit { it[Keys.DeviceName] = name }
    }

    override val deviceNameFlow: Flow<String> = dataStore.data.map { prefs ->
        prefs[Keys.DeviceName] ?: deviceName()
    }

    override val themePreference: Flow<ThemePreference> = dataStore.data.map { prefs ->
        prefs[Keys.Theme]?.let { stored ->
            runCatching { ThemePreference.valueOf(stored) }.getOrNull()
        } ?: ThemePreference.SYSTEM
    }

    override suspend fun saveThemePreference(preference: ThemePreference) {
        dataStore.edit { it[Keys.Theme] = preference.name }
    }

    override val updateMode: Flow<UpdateMode> = dataStore.data.map { prefs ->
        prefs[Keys.UpdateMode]?.let { stored ->
            runCatching { UpdateMode.valueOf(stored) }.getOrNull()
        } ?: UpdateMode.PERIODIC
    }

    override suspend fun saveUpdateMode(mode: UpdateMode) {
        dataStore.edit { it[Keys.UpdateMode] = mode.name }
    }

    override val updateInterval: Flow<UpdateInterval> = dataStore.data.map { prefs ->
        prefs[Keys.UpdateInterval]?.let { stored ->
            runCatching { UpdateInterval.valueOf(stored) }.getOrNull()
        } ?: UpdateInterval.FIVE_MINUTES
    }

    override suspend fun saveUpdateInterval(interval: UpdateInterval) {
        dataStore.edit { it[Keys.UpdateInterval] = interval.name }
    }

    override val transport: Flow<Transport> = dataStore.data.map { prefs ->
        prefs[Keys.Transport]?.let { stored ->
            runCatching { Transport.valueOf(stored) }.getOrNull()
        } ?: Transport.MQTT
    }

    override suspend fun saveTransport(transport: Transport) {
        dataStore.edit { it[Keys.Transport] = transport.name }
    }

    override val reportingEnabled: Flow<Boolean> = dataStore.data.map { prefs ->
        prefs[Keys.ReportingEnabled] ?: true
    }

    override suspend fun saveReportingEnabled(enabled: Boolean) {
        dataStore.edit { it[Keys.ReportingEnabled] = enabled }
    }

    override val enabledControls: Flow<Set<String>> = dataStore.data.map { prefs ->
        prefs[Keys.EnabledControls].orEmpty()
    }

    override suspend fun setControlEnabled(key: String, enabled: Boolean) {
        dataStore.edit { prefs ->
            val current = prefs[Keys.EnabledControls].orEmpty()
            prefs[Keys.EnabledControls] = if (enabled) current + key else current - key
        }
    }

    override val webhookSettings: Flow<WebhookSettings?> = dataStore.data.map { prefs ->
        val url = prefs[Keys.WebhookUrl] ?: return@map null
        WebhookSettings(
            url = url,
            bearerToken = prefs[Keys.WebhookToken]?.takeIf { it.isNotBlank() },
        )
    }

    override suspend fun saveWebhookSettings(settings: WebhookSettings) {
        dataStore.edit {
            it[Keys.WebhookUrl] = settings.url
            it[Keys.WebhookToken] = settings.bearerToken.orEmpty()
        }
    }

    override suspend fun isConfigured(): Boolean = dataStore.data.map { prefs ->
        prefs[Keys.Host] != null || !prefs[Keys.WebhookUrl].isNullOrBlank()
    }.first()

    private object Keys {
        val Host = stringPreferencesKey("broker_host")
        val Port = intPreferencesKey("broker_port")
        val Username = stringPreferencesKey("broker_username")
        val Password = stringPreferencesKey("broker_password")
        val UseTls = booleanPreferencesKey("broker_use_tls")
        val DeviceId = stringPreferencesKey("device_id")
        val DeviceName = stringPreferencesKey("device_name")
        val Theme = stringPreferencesKey("theme_preference")
        val UpdateMode = stringPreferencesKey("update_mode")
        val UpdateInterval = stringPreferencesKey("update_interval")
        val Transport = stringPreferencesKey("transport")
        val ReportingEnabled = booleanPreferencesKey("reporting_enabled")
        val EnabledControls = stringSetPreferencesKey("enabled_controls")
        val WebhookUrl = stringPreferencesKey("webhook_url")
        val WebhookToken = stringPreferencesKey("webhook_token")
    }

    private companion object {
        const val DEFAULT_PORT = 8883
        const val DEVICE_ID_LENGTH = 8
    }
}
