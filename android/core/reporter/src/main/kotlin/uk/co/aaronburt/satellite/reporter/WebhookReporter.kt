package uk.co.aaronburt.satellite.reporter

import android.content.Context
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import uk.co.aaronburt.satellite.common.coroutines.ApplicationScope
import uk.co.aaronburt.satellite.common.coroutines.DispatchersProvider
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.WebhookPayloadBuilder
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.net.HttpURLConnection
import java.net.URL
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Reports device state by HTTP POSTing a JSON snapshot to a webhook endpoint.
 * Honours the same update mode/interval settings as the MQTT reporter.
 */
@Singleton
class WebhookReporter @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val telemetry: DeviceStateReader,
    private val status: ReporterStatus,
    private val dispatchers: DispatchersProvider,
    @ApplicationContext private val context: Context,
    @ApplicationScope private val appScope: CoroutineScope,
) {

    private var settingsJob: Job? = null
    private var periodicJob: Job? = null
    private var intervalJob: Job? = null
    private var modeJob: Job? = null

    @Volatile
    private var intervalMillis: Long = UpdateInterval.FIVE_MINUTES.seconds * 1000

    private val changeWatcher = StateChangeWatcher(
        context = context,
        scope = appScope,
        isActive = { true },
        onChanged = { publish() },
    )

    fun start() {
        if (periodicJob?.isActive == true) return

        settingsJob = appScope.launch {
            settingsRepository.webhookSettings.collect { publish() }
        }

        periodicJob = appScope.launch {
            while (isActive) {
                delay(intervalMillis)
                publish()
            }
        }

        intervalJob = appScope.launch {
            settingsRepository.updateInterval.collect { interval ->
                intervalMillis = interval.seconds * 1000
            }
        }

        modeJob = appScope.launch {
            settingsRepository.updateMode.collect { mode ->
                if (mode == UpdateMode.EVENT_DRIVEN) changeWatcher.register() else changeWatcher.unregister()
            }
        }
    }

    fun stop() {
        settingsJob?.cancel()
        settingsJob = null
        periodicJob?.cancel()
        periodicJob = null
        intervalJob?.cancel()
        intervalJob = null
        modeJob?.cancel()
        modeJob = null
        changeWatcher.unregister()
    }

    suspend fun refresh() = publish()

    private suspend fun publish(): Boolean {
        val settings = settingsRepository.webhookSettings.first()
        if (settings == null || settings.url.isBlank()) return false

        val activeKeys = EntityCatalog.activeKeys(settingsRepository.enabledControls.first())
        val states = runCatching { telemetry.read() }
            .getOrDefault(emptyMap())
            .filterKeys { it in activeKeys }
        val payload = WebhookPayloadBuilder.build(
            deviceId = settingsRepository.deviceId(),
            deviceName = settingsRepository.deviceName(),
            manufacturer = Build.MANUFACTURER ?: "unknown",
            model = Build.MODEL ?: "Android",
            appVersion = appVersion(),
            androidVersion = Build.VERSION.RELEASE ?: "unknown",
            states = states,
            timestampEpochMillis = System.currentTimeMillis(),
        )

        val ok = post(settings.url, payload, settings.bearerToken)
        if (ok) status.markPublished()
        return ok
    }

    private suspend fun post(url: String, body: String, bearerToken: String?): Boolean =
        withContext(dispatchers.io) {
            runCatching {
                val connection = (URL(url).openConnection() as HttpURLConnection).apply {
                    requestMethod = "POST"
                    doOutput = true
                    connectTimeout = TIMEOUT_MILLIS
                    readTimeout = TIMEOUT_MILLIS
                    setRequestProperty("Content-Type", "application/json")
                    if (!bearerToken.isNullOrBlank()) {
                        setRequestProperty("Authorization", "Bearer $bearerToken")
                    }
                }
                connection.outputStream.use { it.write(body.toByteArray()) }
                val code = connection.responseCode
                connection.disconnect()
                code in 200..299
            }.getOrDefault(false)
        }

    private fun appVersion(): String = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName
    }.getOrNull() ?: "unknown"

    private companion object {
        const val TIMEOUT_MILLIS = 10_000
    }
}
