package uk.co.aaronburt.satellite.datastore

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.MutablePreferences
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

private val Context.dataStore by preferencesDataStore(
    name = "satellite_settings",
    produceMigrations = { listOf(LegacySecretMigration(sharedCipher)) },
)

/** One Keystore-backed cipher for the process, shared by the migration and the repository. */
private val sharedCipher: SecretCipher by lazy { KeystoreSecretCipher() }

/**
 * Settings in Preferences DataStore. Non-secret values are stored as-is; the
 * broker password and webhook token are encrypted with [SecretCipher] (see
 * [KeystoreSecretCipher]). A secret that cannot be decrypted reads as absent
 * so the user can re-enter it; the rest of the settings are unaffected.
 */
class DataStoreSettingsRepository internal constructor(
    private val dataStore: DataStore<Preferences>,
    private val cipher: SecretCipher,
    private val randomHex: () -> String = { UUID.randomUUID().toString().replace("-", "") },
) : SettingsRepository {

    @Inject
    constructor(context: Context) : this(context.dataStore, sharedCipher)

    override val brokerSettings: Flow<BrokerSettings?> = dataStore.data.map { prefs ->
        val host = prefs[Keys.Host] ?: return@map null
        BrokerSettings(
            host = host,
            port = prefs[Keys.Port] ?: DEFAULT_PORT,
            username = prefs[Keys.Username]?.takeIf { it.isNotBlank() },
            password = readSecret(prefs, Keys.EncryptedPassword, Keys.LegacyPassword),
            useTls = prefs[Keys.UseTls] ?: true,
        )
    }

    override suspend fun saveBrokerSettings(settings: BrokerSettings) {
        // Encrypt before the transaction: if the Keystore fails, nothing is written.
        val password = encryptOrNull(settings.password)
        dataStore.edit { prefs ->
            prefs[Keys.Host] = settings.host
            prefs[Keys.Port] = settings.port
            prefs[Keys.Username] = settings.username.orEmpty()
            prefs.putSecret(Keys.EncryptedPassword, Keys.LegacyPassword, password)
            prefs[Keys.UseTls] = settings.useTls
        }
    }

    private fun encryptOrNull(secret: String?): String? =
        secret?.takeIf { it.isNotBlank() }?.let(cipher::encrypt)

    private fun MutablePreferences.putSecret(
        encrypted: Preferences.Key<String>,
        legacy: Preferences.Key<String>,
        ciphertext: String?,
    ) {
        if (ciphertext == null) remove(encrypted) else this[encrypted] = ciphertext
        remove(legacy)
    }

    /**
     * Encrypted value first; the legacy plaintext only while a migration is
     * still pending (e.g. the Keystore was unavailable when it last ran).
     */
    private fun readSecret(
        prefs: Preferences,
        encrypted: Preferences.Key<String>,
        legacy: Preferences.Key<String>,
    ): String? {
        val stored = prefs[encrypted]
        val value = if (stored != null) cipher.decrypt(stored) else prefs[legacy]
        return value?.takeIf { it.isNotBlank() }
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
            bearerToken = readSecret(prefs, Keys.EncryptedWebhookToken, Keys.LegacyWebhookToken),
        )
    }

    override suspend fun saveWebhookSettings(settings: WebhookSettings) {
        val token = encryptOrNull(settings.bearerToken)
        dataStore.edit {
            it[Keys.WebhookUrl] = settings.url
            it.putSecret(Keys.EncryptedWebhookToken, Keys.LegacyWebhookToken, token)
        }
    }

    override suspend fun isConfigured(): Boolean = dataStore.data.map { prefs ->
        prefs[Keys.Host] != null || !prefs[Keys.WebhookUrl].isNullOrBlank()
    }.first()

    private object Keys {
        val Host = stringPreferencesKey("broker_host")
        val Port = intPreferencesKey("broker_port")
        val Username = stringPreferencesKey("broker_username")
        val LegacyPassword = LegacySecretMigration.LegacyPassword
        val EncryptedPassword = LegacySecretMigration.EncryptedPassword
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
        val LegacyWebhookToken = LegacySecretMigration.LegacyWebhookToken
        val EncryptedWebhookToken = LegacySecretMigration.EncryptedWebhookToken
    }

    private companion object {
        const val DEFAULT_PORT = 8883
        const val DEVICE_ID_LENGTH = 8
    }
}
