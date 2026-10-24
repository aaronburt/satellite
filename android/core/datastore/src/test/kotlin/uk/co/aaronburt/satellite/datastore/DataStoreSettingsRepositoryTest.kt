package uk.co.aaronburt.satellite.datastore

import androidx.datastore.core.DataStore
import androidx.datastore.core.okio.OkioStorage
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.PreferencesSerializer
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.model.WebhookSettings
import okio.FileSystem
import okio.Path.Companion.toOkioPath
import java.io.File
import java.util.concurrent.atomic.AtomicInteger

class DataStoreSettingsRepositoryTest {

    @get:Rule
    val folder = TemporaryFolder()

    private val file: File by lazy { File(folder.root, "settings.preferences_pb") }
    private var job = Job()
    private lateinit var dataStore: DataStore<Preferences>

    /** Each call returns a different value, so any id generated twice is visible. */
    private val counter = AtomicInteger()
    private val randomHex = {
        val n = counter.incrementAndGet()
        "%04x%04x".format(n, n) + "0".repeat(24)
    }

    private val cipher = FakeCipher()

    /** Reads the raw file, bypassing the repository, to see what is stored at rest. */
    private suspend fun raw(): Preferences = dataStore.data.first()

    private fun repository(): DataStoreSettingsRepository {
        job = Job()
        // OkioStorage, not the default java.io FileStorage: FileStorage commits
        // each write with File.renameTo, which on Windows refuses to replace an
        // existing file, so every second write to settings.preferences_pb threw
        // IOException (FileStorage.kt:114) on Windows hosts. Okio's atomicMove
        // replaces the target on every OS. Android's filesystem is POSIX, so the
        // app is unaffected; this only makes the JVM tests host-independent.
        dataStore = PreferenceDataStoreFactory.create(
            storage = OkioStorage(FileSystem.SYSTEM, PreferencesSerializer) { file.toOkioPath() },
            migrations = listOf(LegacySecretMigration(cipher)),
            scope = CoroutineScope(Dispatchers.IO + job),
        )
        return DataStoreSettingsRepository(dataStore, cipher, randomHex)
    }

    /** Closes the current store, as process death would, and opens the same file again. */
    private suspend fun reopen(): DataStoreSettingsRepository {
        job.cancelAndJoin()
        return repository()
    }

    @After
    fun tearDown() = runBlocking { job.cancelAndJoin() }

    @Test
    fun `defaults before anything is saved`() = runTest {
        val repo = repository()

        assertNull(repo.brokerSettings.first())
        assertNull(repo.webhookSettings.first())
        assertEquals(Transport.MQTT, repo.transport.first())
        assertTrue(repo.reportingEnabled.first())
        assertEquals(emptySet<String>(), repo.enabledControls.first())
        assertEquals(UpdateMode.PERIODIC, repo.updateMode.first())
        assertEquals(UpdateInterval.FIVE_MINUTES, repo.updateInterval.first())
        assertEquals(ThemePreference.SYSTEM, repo.themePreference.first())
        assertFalse(repo.isConfigured())
    }

    @Test
    fun `broker settings round trip and survive a restart`() = runTest {
        val settings = BrokerSettings("broker.local", 1884, "user", "p@ss", useTls = false)
        repository().saveBrokerSettings(settings)

        assertEquals(settings, reopen().brokerSettings.first())
    }

    @Test
    fun `blank broker credentials are read back as none`() = runTest {
        val repo = repository()
        repo.saveBrokerSettings(BrokerSettings("broker.local", 8883, " ", "", useTls = true))

        val saved = repo.brokerSettings.first()!!
        assertNull(saved.username)
        assertNull(saved.password)
        assertTrue(saved.useTls)
    }

    @Test
    fun `clearing credentials removes the old ones`() = runTest {
        val repo = repository()
        repo.saveBrokerSettings(BrokerSettings("b", 1, "user", "secret", useTls = true))
        repo.saveBrokerSettings(BrokerSettings("b", 1, null, null, useTls = true))

        val saved = repo.brokerSettings.first()!!
        assertNull(saved.username)
        assertNull(saved.password)
    }

    @Test
    fun `webhook settings round trip, with a blank token read as none`() = runTest {
        val repo = repository()
        repo.saveWebhookSettings(WebhookSettings("https://ha.local/api/webhook/x", "tok"))
        assertEquals(WebhookSettings("https://ha.local/api/webhook/x", "tok"), repo.webhookSettings.first())

        repo.saveWebhookSettings(WebhookSettings("https://ha.local/api/webhook/x", " "))
        assertNull(repo.webhookSettings.first()!!.bearerToken)
    }

    @Test
    fun `configured once a broker or a non-blank webhook is saved`() = runTest {
        val repo = repository()
        repo.saveWebhookSettings(WebhookSettings("  "))
        assertFalse("a blank webhook URL is not a configuration", repo.isConfigured())

        repo.saveWebhookSettings(WebhookSettings("https://ha.local/hook"))
        assertTrue(repo.isConfigured())

        val brokerOnly = reopenEmpty()
        brokerOnly.saveBrokerSettings(BrokerSettings("b", 1, null, null, useTls = true))
        assertTrue(brokerOnly.isConfigured())
    }

    private suspend fun reopenEmpty(): DataStoreSettingsRepository {
        job.cancelAndJoin()
        file.delete()
        return repository()
    }

    @Test
    fun `simple settings round trip`() = runTest {
        val repo = repository()
        repo.saveTransport(Transport.WEBHOOK)
        repo.saveReportingEnabled(false)
        repo.saveUpdateMode(UpdateMode.EVENT_DRIVEN)
        repo.saveUpdateInterval(UpdateInterval.THIRTY_SECONDS)
        repo.saveThemePreference(ThemePreference.DARK)

        val reopened = reopen()
        assertEquals(Transport.WEBHOOK, reopened.transport.first())
        assertFalse(reopened.reportingEnabled.first())
        assertEquals(UpdateMode.EVENT_DRIVEN, reopened.updateMode.first())
        assertEquals(UpdateInterval.THIRTY_SECONDS, reopened.updateInterval.first())
        assertEquals(ThemePreference.DARK, reopened.themePreference.first())
    }

    @Test
    fun `unknown stored values fall back to the defaults instead of crashing`() = runTest {
        // e.g. a value written by a newer version, then the app is downgraded.
        val repo = repository()
        dataStore.edit {
            it[stringPreferencesKey("transport")] = "CARRIER_PIGEON"
            it[stringPreferencesKey("update_mode")] = "SOMETIMES"
            it[stringPreferencesKey("update_interval")] = "FORTNIGHTLY"
            it[stringPreferencesKey("theme_preference")] = "SEPIA"
        }

        assertEquals(Transport.MQTT, repo.transport.first())
        assertEquals(UpdateMode.PERIODIC, repo.updateMode.first())
        assertEquals(UpdateInterval.FIVE_MINUTES, repo.updateInterval.first())
        assertEquals(ThemePreference.SYSTEM, repo.themePreference.first())
    }

    @Test
    fun `controls are enabled and disabled individually`() = runTest {
        val repo = repository()
        repo.setControlEnabled("volume_media", true)
        repo.setControlEnabled("mic_muted", true)
        repo.setControlEnabled("volume_media", true)
        assertEquals(setOf("volume_media", "mic_muted"), repo.enabledControls.first())

        repo.setControlEnabled("volume_media", false)
        repo.setControlEnabled("not_enabled", false)
        assertEquals(setOf("mic_muted"), reopen().enabledControls.first())
    }

    @Test
    fun `device id is generated once, eight hex characters, and kept`() = runTest {
        val repo = repository()
        val id = repo.deviceId()

        assertTrue(id, Regex("[0-9a-f]{8}").matches(id))
        assertEquals(id, repo.deviceId())
        assertEquals("survives a restart", id, reopen().deviceId())
    }

    @Test
    fun `concurrent first calls agree on one device id`() = runBlocking {
        // On first launch the reporter and the connection manager ask at the
        // same time. Each used to save its own id, so they could publish to
        // different topics until the next restart.
        val repo = repository()

        val ids = (1..16).map { async(Dispatchers.IO) { repo.deviceId() } }.awaitAll()

        assertEquals(ids.toString(), 1, ids.toSet().size)
        assertEquals(ids.first(), repo.deviceId())
    }

    @Test
    fun `concurrent first calls agree on one device name`() = runBlocking {
        val repo = repository()

        val names = (1..16).map { async(Dispatchers.IO) { repo.deviceName() } }.awaitAll()

        assertEquals(names.toString(), 1, names.toSet().size)
    }

    @Test
    fun `device name is generated as Satellite-XXXX, then editable`() = runTest {
        val repo = repository()
        val generated = repo.deviceName()
        assertTrue(generated, Regex("Satellite-[0-9A-F]{4}").matches(generated))
        assertEquals(generated, repo.deviceName())

        repo.saveDeviceName("Kitchen tablet")
        assertEquals("Kitchen tablet", repo.deviceName())
        assertEquals("Kitchen tablet", reopen().deviceName())
    }

    @Test
    fun `device name flow generates a name on first read and follows edits`() = runTest {
        val repo = repository()

        val generated = repo.deviceNameFlow.first()
        assertTrue(generated, generated.startsWith("Satellite-"))
        assertEquals("the generated name is saved", generated, repo.deviceName())

        repo.saveDeviceName("Hall")
        assertEquals("Hall", repo.deviceNameFlow.first())
    }

    // --- Secret storage (issue #15) ---

    /** Writes a file the way older app versions did, with plaintext secrets. */
    private suspend fun seedLegacyFile(vararg entries: Pair<String, String>) {
        repository()
        dataStore.edit { prefs -> entries.forEach { (k, v) -> prefs[stringPreferencesKey(k)] = v } }
    }

    private fun Preferences.string(key: String) = this[stringPreferencesKey(key)]

    @Test
    fun `secrets are encrypted at rest and never stored in plaintext`() = runTest {
        val repo = repository()
        repo.saveBrokerSettings(BrokerSettings("broker.local", 8883, "user", "hunter2", useTls = true))
        repo.saveWebhookSettings(WebhookSettings("https://hook", bearerToken = "tok-123"))

        val stored = raw()
        assertNull(stored.string("broker_password"))
        assertNull(stored.string("webhook_token"))
        assertEquals(cipher.encrypt("hunter2"), stored.string("broker_password_enc"))
        assertEquals(cipher.encrypt("tok-123"), stored.string("webhook_token_enc"))
        assertFalse(file.readBytes().toString(Charsets.ISO_8859_1).contains("hunter2"))
        assertFalse(file.readBytes().toString(Charsets.ISO_8859_1).contains("tok-123"))
        // Non-secret settings stay directly readable.
        assertEquals("broker.local", stored.string("broker_host"))
        assertEquals("user", stored.string("broker_username"))
    }

    @Test
    fun `encrypted secrets round trip across a restart`() = runTest {
        repository().saveWebhookSettings(WebhookSettings("https://hook", bearerToken = "tok-123"))

        assertEquals("tok-123", reopen().webhookSettings.first()!!.bearerToken)
    }

    @Test
    fun `clearing a secret removes its stored value`() = runTest {
        val repo = repository()
        repo.saveBrokerSettings(BrokerSettings("broker.local", 8883, "user", "hunter2", useTls = true))
        repo.saveWebhookSettings(WebhookSettings("https://hook", bearerToken = "tok"))

        repo.saveBrokerSettings(BrokerSettings("broker.local", 8883, "user", null, useTls = true))
        repo.saveWebhookSettings(WebhookSettings("https://hook", bearerToken = " "))

        assertNull(raw().string("broker_password_enc"))
        assertNull(raw().string("webhook_token_enc"))
        assertNull(repo.brokerSettings.first()!!.password)
        assertNull(repo.webhookSettings.first()!!.bearerToken)
    }

    @Test
    fun `existing plaintext install is migrated and keeps every setting`() = runTest {
        seedLegacyFile(
            "broker_host" to "old.local",
            "broker_username" to "olduser",
            "broker_password" to "oldpass",
            "webhook_url" to "https://old-hook",
            "webhook_token" to "oldtoken",
            "device_id" to "abcd1234",
        )

        val repo = reopen()

        val broker = repo.brokerSettings.first()!!
        assertEquals("old.local", broker.host)
        assertEquals("olduser", broker.username)
        assertEquals("oldpass", broker.password)
        assertEquals("oldtoken", repo.webhookSettings.first()!!.bearerToken)
        assertEquals("abcd1234", repo.deviceId())
        assertNull(raw().string("broker_password"))
        assertNull(raw().string("webhook_token"))
        assertEquals(cipher.encrypt("oldpass"), raw().string("broker_password_enc"))
    }

    @Test
    fun `blank legacy secrets are dropped rather than encrypted`() = runTest {
        seedLegacyFile("broker_host" to "old.local", "broker_password" to "")

        val repo = reopen()

        assertNull(repo.brokerSettings.first()!!.password)
        assertNull(raw().string("broker_password"))
        assertNull(raw().string("broker_password_enc"))
    }

    @Test
    fun `interrupted migration leaves plaintext in place and retries on next open`() = runTest {
        seedLegacyFile("broker_host" to "old.local", "broker_password" to "oldpass")
        job.cancelAndJoin()
        cipher.failEncrypt = true

        // Keystore unavailable: the migration throws, nothing is committed.
        repository()
        assertTrue(runCatching { dataStore.data.first() }.isFailure)
        job.cancelAndJoin()

        cipher.failEncrypt = false
        val repo = repository()
        assertEquals("oldpass", repo.brokerSettings.first()!!.password)
        assertNull(raw().string("broker_password"))
        assertEquals("old.local", repo.brokerSettings.first()!!.host)
    }

    @Test
    fun `legacy value already shadowed by an encrypted one is discarded`() = runTest {
        repository()
        dataStore.edit {
            it[stringPreferencesKey("broker_host")] = "h"
            it[stringPreferencesKey("broker_password_enc")] = cipher.encrypt("newer")
            it[stringPreferencesKey("broker_password")] = "older"
        }

        assertEquals("newer", reopen().brokerSettings.first()!!.password)
        assertNull(raw().string("broker_password"))
    }

    @Test
    fun `undecryptable secret reads as absent without losing other settings`() = runTest {
        val repo = repository()
        repo.saveBrokerSettings(BrokerSettings("broker.local", 1884, "user", "hunter2", useTls = false))
        repo.saveWebhookSettings(WebhookSettings("https://hook", bearerToken = "tok"))
        repo.saveTransport(Transport.WEBHOOK)

        // Simulates a wiped/invalidated Keystore key, or a restore onto another device.
        cipher.failDecrypt = true

        val broker = repo.brokerSettings.first()!!
        assertEquals(BrokerSettings("broker.local", 1884, "user", null, useTls = false), broker)
        assertEquals(WebhookSettings("https://hook", null), repo.webhookSettings.first())
        assertEquals(Transport.WEBHOOK, repo.transport.first())
        assertTrue(repo.isConfigured())

        // Re-entering the secret recovers.
        cipher.failDecrypt = false
        repo.saveBrokerSettings(broker.copy(password = "again"))
        assertEquals("again", repo.brokerSettings.first()!!.password)
    }

    @Test
    fun `failed encryption on save writes nothing`() = runTest {
        val repo = repository()
        repo.saveBrokerSettings(BrokerSettings("broker.local", 8883, "user", "hunter2", useTls = true))
        cipher.failEncrypt = true

        val result = runCatching {
            repo.saveBrokerSettings(BrokerSettings("other.local", 8883, "user", "new", useTls = true))
        }

        assertTrue(result.isFailure)
        cipher.failEncrypt = false
        assertEquals("broker.local", repo.brokerSettings.first()!!.host)
        assertEquals("hunter2", repo.brokerSettings.first()!!.password)
    }

    /** Deterministic, reversible stand-in for the Keystore cipher. */
    private class FakeCipher : SecretCipher {
        var failEncrypt = false
        var failDecrypt = false

        override fun encrypt(plaintext: String): String {
            if (failEncrypt) throw IllegalStateException("keystore unavailable")
            return "fake:" + plaintext.reversed().map { it + 1 }.joinToString("")
        }

        override fun decrypt(ciphertext: String): String? {
            if (failDecrypt || !ciphertext.startsWith("fake:")) return null
            return ciphertext.removePrefix("fake:").map { it - 1 }.joinToString("").reversed()
        }
    }
}
