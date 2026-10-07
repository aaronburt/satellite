package uk.co.aaronburt.satellite.datastore

import android.content.Context
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
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

class DataStoreSettingsRepository @Inject constructor(
    private val context: Context,
) : SettingsRepository {

    override val brokerSettings: Flow<BrokerSettings?> = context.dataStore.data.map { prefs ->
        val host = prefs[Keys.Host] ?: return@map null
        BrokerSettings(
            host = host,
            port = prefs[Keys.Port] ?: DEFAULT_PORT,
            username = prefs[Keys.Username]?.takeIf { it.isNotBlank() },
            password = prefs[Keys.Password]?.takeIf { it.isNotBlank() },
            useTls = prefs[Keys.UseTls] ?: false,
        )
    }

    override suspend fun saveBrokerSettings(settings: BrokerSettings) {
        context.dataStore.edit { prefs ->
            prefs[Keys.Host] = settings.host
            prefs[Keys.Port] = settings.port
            prefs[Keys.Username] = settings.username.orEmpty()
            prefs[Keys.Password] = settings.password.orEmpty()
            prefs[Keys.UseTls] = settings.useTls
        }
    }

    override suspend fun deviceId(): String {
        val existing = context.dataStore.data.map { it[Keys.DeviceId] }.first()
        if (existing != null) return existing

        val generated = UUID.randomUUID().toString().replace("-", "").take(DEVICE_ID_LENGTH)
        context.dataStore.edit { it[Keys.DeviceId] = generated }
        return generated
    }

    override suspend fun deviceName(): String {
        val existing = context.dataStore.data.map { it[Keys.DeviceName] }.first()
        if (existing != null) return existing

        val generated = generateDeviceName()
        context.dataStore.edit { it[Keys.DeviceName] = generated }
        return generated
    }

    override suspend fun saveDeviceName(name: String) {
        context.dataStore.edit { it[Keys.DeviceName] = name }
    }

    override val themePreference: Flow<ThemePreference> = context.dataStore.data.map { prefs ->
        prefs[Keys.Theme]?.let { stored ->
            runCatching { ThemePreference.valueOf(stored) }.getOrNull()
        } ?: ThemePreference.SYSTEM
    }

    override suspend fun saveThemePreference(preference: ThemePreference) {
        context.dataStore.edit { it[Keys.Theme] = preference.name }
    }

    override val updateMode: Flow<UpdateMode> = context.dataStore.data.map { prefs ->
        prefs[Keys.UpdateMode]?.let { stored ->
            runCatching { UpdateMode.valueOf(stored) }.getOrNull()
        } ?: UpdateMode.PERIODIC
    }

    override suspend fun saveUpdateMode(mode: UpdateMode) {
        context.dataStore.edit { it[Keys.UpdateMode] = mode.name }
    }

    override val updateInterval: Flow<UpdateInterval> = context.dataStore.data.map { prefs ->
        prefs[Keys.UpdateInterval]?.let { stored ->
            runCatching { UpdateInterval.valueOf(stored) }.getOrNull()
        } ?: UpdateInterval.FIVE_MINUTES
    }

    override suspend fun saveUpdateInterval(interval: UpdateInterval) {
        context.dataStore.edit { it[Keys.UpdateInterval] = interval.name }
    }

    override val transport: Flow<Transport> = context.dataStore.data.map { prefs ->
        prefs[Keys.Transport]?.let { stored ->
            runCatching { Transport.valueOf(stored) }.getOrNull()
        } ?: Transport.MQTT
    }

    override suspend fun saveTransport(transport: Transport) {
        context.dataStore.edit { it[Keys.Transport] = transport.name }
    }

    override val webhookSettings: Flow<WebhookSettings?> = context.dataStore.data.map { prefs ->
        val url = prefs[Keys.WebhookUrl] ?: return@map null
        WebhookSettings(
            url = url,
            bearerToken = prefs[Keys.WebhookToken]?.takeIf { it.isNotBlank() },
        )
    }

    override suspend fun saveWebhookSettings(settings: WebhookSettings) {
        context.dataStore.edit {
            it[Keys.WebhookUrl] = settings.url
            it[Keys.WebhookToken] = settings.bearerToken.orEmpty()
        }
    }

    override suspend fun isConfigured(): Boolean = context.dataStore.data.map { prefs ->
        prefs[Keys.Host] != null || !prefs[Keys.WebhookUrl].isNullOrBlank()
    }.first()

    private fun generateDeviceName(): String =
        "Satellite-" + UUID.randomUUID().toString().replace("-", "").take(4).uppercase()

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
        val WebhookUrl = stringPreferencesKey("webhook_url")
        val WebhookToken = stringPreferencesKey("webhook_token")
    }

    private companion object {
        const val DEFAULT_PORT = 1883
        const val DEVICE_ID_LENGTH = 8
    }
}
