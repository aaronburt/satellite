package uk.co.aaronburt.satellite.datastore

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
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

    private fun repository(): DataStoreSettingsRepository {
        job = Job()
        // createWithPath (Okio storage) rather than create (java.io File storage):
        // the File storage commits each write with File.renameTo, which on
        // Windows refuses to replace an existing file, so every second write
        // to settings.preferences_pb failed on Windows hosts. Okio's atomicMove
        // replaces the target on every OS. Android's own filesystem is POSIX, so
        // the app is unaffected; this only makes the JVM tests host-independent.
        dataStore = PreferenceDataStoreFactory.createWithPath(scope = CoroutineScope(Dispatchers.IO + job)) {
            file.toOkioPath()
        }
        return DataStoreSettingsRepository(dataStore, randomHex)
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
}
