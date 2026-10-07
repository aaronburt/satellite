package uk.co.aaronburt.satellite.reporter

import android.content.Context
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import uk.co.aaronburt.satellite.common.coroutines.ApplicationScope
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.DiscoveryPayloadBuilder
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.mqtt.MqttClient
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.first
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
 * Discovery is re-published when Home Assistant restarts (birth message) and
 * whenever the user opts a controllable entity in or out. Only entities the user
 * has enabled are ever published — controls are off by default.
 */
@Singleton
class SatelliteReporter @Inject constructor(
    private val client: MqttClient,
    private val settingsRepository: SettingsRepository,
    private val telemetry: DeviceStateReader,
    private val commandRouter: CommandRouter,
    private val audioControls: AudioControls,
    private val status: ReporterStatus,
    @ApplicationContext private val context: Context,
    @ApplicationScope private val appScope: CoroutineScope,
) {

    private var observerJob: Job? = null
    private var periodicJob: Job? = null
    private var intervalJob: Job? = null
    private var modeJob: Job? = null
    private var controlsJob: Job? = null

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

        // Toggling a control changes what Home Assistant should see, so
        // re-publish discovery (and state) immediately, and clean up after any
        // control the user has just hidden.
        controlsJob = appScope.launch {
            var previous: Set<String>? = null
            settingsRepository.enabledControls.collect { enabled ->
                val removed = previous?.minus(enabled).orEmpty()
                previous = enabled

                if (client.connectionState.value !is ConnectionState.Connected) return@collect

                val deviceId = settingsRepository.deviceId()
                val topics = Topics(deviceId)

                removed.forEach { key ->
                    // Drop the retained state so the broker keeps no ghost value
                    // for an entity Home Assistant no longer knows about.
                    client.publish(topics.state(key), "", qos = 1, retain = true)
                    if (key == EntityCatalog.CONTROL_MIC_MUTE) {
                        // Never strand the phone muted with no way back: hiding the
                        // control restores the microphone.
                        audioControls.setMicrophoneMuted(false)
                    }
                }

                publishDiscovery(deviceId, topics)
                publishStates()
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
        controlsJob?.cancel()
        controlsJob = null
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

        client.subscribe(topics.commandWildcard, qos = 1) { message ->
            appScope.launch {
                val accepted = commandRouter.handle(message.topic, message.payload, topics)
                // Confirm the resulting state, whether or not the change stuck.
                if (accepted) publishStates()
            }
        }
    }

    private suspend fun publishDiscovery(deviceId: String, topics: Topics) {
        val enabledControls = settingsRepository.enabledControls.first()
        val unexposed = EntityCatalog.controlEntities
            .map { it.key }
            .filterNot { it in enabledControls }

        // Home Assistant drops a component when it is republished with nothing but
        // its platform, so send that first, then the config without it.
        if (unexposed.isNotEmpty()) {
            publishDiscoveryPayload(deviceId, topics, enabledControls, unexposed.toSet())
        }
        publishDiscoveryPayload(deviceId, topics, enabledControls, emptySet())
    }

    private suspend fun publishDiscoveryPayload(
        deviceId: String,
        topics: Topics,
        enabledControls: Set<String>,
        stubControls: Set<String>,
    ) {
        val payload = DiscoveryPayloadBuilder.build(
            deviceId = deviceId,
            deviceName = settingsRepository.deviceName(),
            manufacturer = Build.MANUFACTURER ?: "unknown",
            model = Build.MODEL ?: "Android",
            appVersion = appVersion(),
            androidVersion = Build.VERSION.RELEASE ?: "unknown",
            enabledControls = enabledControls,
            stubControls = stubControls,
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
        val activeKeys = EntityCatalog.activeKeys(settingsRepository.enabledControls.first())
        val states = runCatching { telemetry.read() }.getOrDefault(emptyMap())
        states.filterKeys { it in activeKeys }.forEach { (key, value) ->
            client.publish(
                topic = topics.state(key),
                payload = value,
                qos = 1,
                retain = true,
                expirySeconds = STATE_EXPIRY_SECONDS,
            )
        }
        status.markPublished()
    }

    private fun appVersion(): String = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName
    }.getOrNull() ?: "unknown"

    private companion object {
        const val DISCOVERY_EXPIRY_SECONDS = 7 * 24 * 60 * 60L
        const val STATE_EXPIRY_SECONDS = 60 * 60L
    }
}
