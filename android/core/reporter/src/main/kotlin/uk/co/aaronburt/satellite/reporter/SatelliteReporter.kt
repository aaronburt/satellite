package uk.co.aaronburt.satellite.reporter

import android.content.Context
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import uk.co.aaronburt.satellite.common.coroutines.ApplicationScope
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.DiscoveryPayloadBuilder
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.mqtt.MqttClient
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import javax.inject.Inject
import javax.inject.Singleton

/**
 * MQTT reporter: publishes the discovery payload and device state on connect,
 * then per the user's [UpdateMode]:
 *
 * - [UpdateMode.PERIODIC]: every [UpdateInterval] (default 5 minutes).
 * - [UpdateMode.EVENT_DRIVEN]: immediately on relevant broadcasts, with the
 *   interval as a safety net.
 *
 * Discovery is re-published when Home Assistant restarts (birth message).
 */
@Singleton
class SatelliteReporter @Inject constructor(
    private val client: MqttClient,
    private val settingsRepository: SettingsRepository,
    private val telemetry: DeviceStateReader,
    @ApplicationContext private val context: Context,
    @ApplicationScope private val appScope: CoroutineScope,
) {

    private var observerJob: Job? = null
    private var periodicJob: Job? = null
    private var intervalJob: Job? = null
    private var modeJob: Job? = null

    @Volatile
    private var intervalMillis: Long = UpdateInterval.FIVE_MINUTES.seconds * 1000

    private val changeWatcher = StateChangeWatcher(
        context = context,
        scope = appScope,
        isActive = { client.connectionState.value is ConnectionState.Connected },
        onChanged = { publishStates() },
    )

    fun start() {
        if (observerJob?.isActive == true) return

        observerJob = appScope.launch {
            client.connectionState.collect { state ->
                if (state is ConnectionState.Connected) onConnected()
            }
        }

        periodicJob = appScope.launch {
            while (isActive) {
                delay(intervalMillis)
                if (client.connectionState.value is ConnectionState.Connected) {
                    publishStates()
                }
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
        observerJob?.cancel()
        observerJob = null
        periodicJob?.cancel()
        periodicJob = null
        intervalJob?.cancel()
        intervalJob = null
        modeJob?.cancel()
        modeJob = null
        changeWatcher.unregister()
    }

    /** Force an immediate discovery + state publish. Returns false when not connected. */
    suspend fun refresh(): Boolean {
        if (client.connectionState.value is ConnectionState.Connected) {
            onConnected()
            return true
        }
        return false
    }

    private suspend fun onConnected() {
        val deviceId = settingsRepository.deviceId()
        val topics = Topics(deviceId)

        publishDiscovery(deviceId, topics)
        publishStates()

        // Re-subscribe on every connect: the broker session may have expired, and
        // re-subscribing with the same filter simply replaces the subscription.
        client.subscribe(Topics.BIRTH_TOPIC, qos = 0) { message ->
            if (message.payload.trim() == "online") {
                appScope.launch {
                    publishDiscovery(deviceId, topics)
                    publishStates()
                }
            }
        }
    }

    private suspend fun publishDiscovery(deviceId: String, topics: Topics) {
        val payload = DiscoveryPayloadBuilder.build(
            deviceId = deviceId,
            deviceName = settingsRepository.deviceName(),
            manufacturer = Build.MANUFACTURER ?: "unknown",
            model = Build.MODEL ?: "Android",
            appVersion = appVersion(),
            androidVersion = Build.VERSION.RELEASE ?: "unknown",
        )
        client.publish(
            topic = topics.discovery,
            payload = payload,
            qos = 1,
            retain = true,
            expirySeconds = DISCOVERY_EXPIRY_SECONDS,
        )
    }

    private suspend fun publishStates() {
        val topics = Topics(settingsRepository.deviceId())
        val states = runCatching { telemetry.read() }.getOrDefault(emptyMap())
        states.forEach { (key, value) ->
            client.publish(
                topic = topics.state(key),
                payload = value,
                qos = 1,
                retain = true,
                expirySeconds = STATE_EXPIRY_SECONDS,
            )
        }
    }

    private fun appVersion(): String = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName
    }.getOrNull() ?: "unknown"

    private companion object {
        const val DISCOVERY_EXPIRY_SECONDS = 7 * 24 * 60 * 60L
        const val STATE_EXPIRY_SECONDS = 60 * 60L
    }
}
