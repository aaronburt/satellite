package uk.co.aaronburt.satellite.reporter

import android.content.Context
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.random.Random
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import uk.co.aaronburt.satellite.common.coroutines.ApplicationScope
import uk.co.aaronburt.satellite.common.coroutines.DispatchersProvider
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.WebhookPayloadBuilder
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.update.NoUpdateStates
import uk.co.aaronburt.satellite.update.UpdateStatesSource

/**
 * Reports device state by HTTP POSTing a JSON snapshot to a webhook endpoint.
 * Honours the same update mode/interval settings as the MQTT reporter.
 *
 * Failed posts are retried in the background per [WebhookDeliveryPolicy].
 * A new publish, stop(), clearing the webhook or changing its URL cancels
 * any retry in progress.
 */
@Singleton
@Suppress("LongParameterList") // Hilt-injected collaborators
class WebhookReporter @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val telemetry: DeviceStateReader,
    private val status: ReporterStatus,
    private val dispatchers: DispatchersProvider,
    @ApplicationContext private val context: Context,
    @ApplicationScope private val appScope: CoroutineScope,
    private val updateStates: UpdateStatesSource = NoUpdateStates
) {

    private var settingsJob: Job? = null
    private var periodicJob: Job? = null
    private var modeJob: Job? = null
    private var retryJob: Job? = null

    /** Source of retry jitter in [0,1). Tests pin it. */
    internal var jitter: () -> Double = { Random.nextDouble() }
    internal var clock: () -> Long = { System.currentTimeMillis() }

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

        // The timer restarts whenever the interval setting changes, and the
        // very first wait already uses the saved interval.
        periodicJob = appScope.launch {
            settingsRepository.updateInterval.collectLatest { interval ->
                while (true) {
                    delay(interval.seconds * 1000)
                    publish()
                }
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
        modeJob?.cancel()
        modeJob = null
        retryJob?.cancel()
        retryJob = null
        changeWatcher.unregister()
    }

    suspend fun refresh() = publish()

    private suspend fun publish(): Boolean {
        // Newest snapshot wins: whatever is being retried is now stale.
        retryJob?.cancel()
        retryJob = null

        val settings = settingsRepository.webhookSettings.first()
        if (settings == null || settings.url.isBlank()) return false

        val activeKeys = EntityCatalog.activeKeys(settingsRepository.enabledControls.first())
        val states = runCatching { telemetry.read() + updateStates.read() }
            .getOrDefault(emptyMap())
            .filterKeys { it in activeKeys }
        val capturedAt = clock()
        val payload = WebhookPayloadBuilder.build(
            deviceId = settingsRepository.deviceId(),
            deviceName = settingsRepository.deviceName(),
            manufacturer = Build.MANUFACTURER ?: "unknown",
            model = Build.MODEL ?: "Android",
            appVersion = appVersion(),
            androidVersion = Build.VERSION.RELEASE ?: "unknown",
            states = states,
            timestampEpochMillis = capturedAt,
        )

        val result = post(settings.url, payload, settings.bearerToken)
        if (result.ok) {
            status.markPublished()
        } else if (result.retryable) {
            retryJob = appScope.launch { retry(settings.url, payload, capturedAt, result) }
        }
        return result.ok
    }

    /** Retries [payload] unchanged (original timestamp) until the policy gives up. */
    private suspend fun retry(url: String, payload: String, capturedAt: Long, first: PostResult) {
        var last = first
        for (attempt in 2..WebhookDeliveryPolicy.MAX_ATTEMPTS) {
            delay(WebhookDeliveryPolicy.delayMillis(attempt, last.retryAfterMillis, jitter()))
            if (clock() - capturedAt > WebhookDeliveryPolicy.MAX_AGE_MILLIS) return
            val current = settingsRepository.webhookSettings.first()
            if (current == null || current.url != url) return
            last = post(url, payload, current.bearerToken)
            if (last.ok) {
                status.markPublished()
                return
            }
            if (!last.retryable) return
        }
    }

    internal class PostResult(val code: Int?, val retryAfterMillis: Long = 0, val permanent: Boolean = false) {
        val ok get() = code != null && code in 200..299
        val retryable get() = !ok && !permanent && WebhookDeliveryPolicy.isRetryable(code)
    }

    // Exceptions are deliberately reduced to a retryable/permanent verdict.
    @Suppress("SwallowedException")
    private suspend fun post(url: String, body: String, bearerToken: String?): PostResult =
        withContext(dispatchers.io) {
            val connection = try {
                URL(url).openConnection() as HttpURLConnection
            } catch (e: IOException) {
                return@withContext PostResult(code = null, permanent = true) // malformed URL
            } catch (e: IllegalArgumentException) {
                return@withContext PostResult(code = null, permanent = true)
            }
            try {
                connection.apply {
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
                PostResult(code, WebhookDeliveryPolicy.parseRetryAfterMillis(connection.getHeaderField("Retry-After")))
            } catch (e: IllegalArgumentException) {
                PostResult(code = null, permanent = true)
            } catch (e: IOException) {
                PostResult(code = null) // offline, timeout, reset: retryable
            } finally {
                connection.disconnect()
            }
        }

    private fun appVersion(): String = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName
    }.getOrNull() ?: "unknown"

    private companion object {
        const val TIMEOUT_MILLIS = 10_000
    }
}
