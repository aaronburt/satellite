package uk.co.aaronburt.satellite.reporter

import android.content.Context
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
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
    @ApplicationScope private val appScope: CoroutineScope
) {

    private var observerJob: Job? = null
    private var periodicJob: Job? = null
    private var modeJob: Job? = null
    private var controlsJob: Job? = null

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

        // The timer restarts whenever the interval setting changes, and the
        // very first wait already uses the saved interval.
        periodicJob = appScope.launch {
            settingsRepository.updateInterval.collectLatest { interval ->
                while (true) {
                    delay(interval.seconds * 1000)
                    if (client.connectionState.value is ConnectionState.Connected) {
                        publishStates()
                    }
                }
            }
        }

        modeJob = appScope.launch {
            settingsRepository.updateMode.collect { mode ->
                if (mode == UpdateMode.EVENT_DRIVEN) changeWatcher.register() else changeWatcher.unregister()
            }
        }

        controlsJob = appScope.launch {
            var previous: Set<String>? = null
            settingsRepository.enabledControls.collect { enabled ->
                val removed = previous?.minus(enabled).orEmpty()
                previous = enabled

                // Handing the microphone back is a local safety step: it must
                // happen even while offline, or a mic muted from Home Assistant
                // would stay muted with nothing left that can unmute it.
                if (EntityCatalog.CONTROL_MIC_MUTE in removed) {
                    audioControls.setMicrophoneMuted(false)
                }

                if (client.connectionState.value !is ConnectionState.Connected) return@collect

                val deviceId = settingsRepository.deviceId()
                val topics = Topics(deviceId)

                removed.forEach { key ->
                    client.publish(topics.state(key), "", qos = 1, retain = true)
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
        modeJob?.cancel()
        modeJob = null
        controlsJob?.cancel()
        controlsJob = null
        changeWatcher.unregister()
    }

    /**
     * Force an immediate discovery + state publish. Returns false when not
     * connected or when the broker did not accept the publishes.
     */
    suspend fun refresh(): Boolean {
        if (client.connectionState.value !is ConnectionState.Connected) return false
        return onConnected()
    }

    private suspend fun onConnected(): Boolean {
        val deviceId = settingsRepository.deviceId()
        val topics = Topics(deviceId)

        val discovered = publishDiscovery(deviceId, topics)
        val published = publishStates()

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
                if (accepted) publishStates()
            }
        }

        return discovered && published
    }

    private suspend fun publishDiscovery(deviceId: String, topics: Topics): Boolean {
        val enabledControls = settingsRepository.enabledControls.first()
        val unexposed = EntityCatalog.controlEntities
            .map { it.key }
            .filterNot { it in enabledControls }

        if (unexposed.isNotEmpty()) {
            publishDiscoveryPayload(deviceId, topics, enabledControls, unexposed.toSet())
        }
        return publishDiscoveryPayload(deviceId, topics, enabledControls, emptySet()).isSuccess
    }

    private suspend fun publishDiscoveryPayload(
        deviceId: String,
        topics: Topics,
        enabledControls: Set<String>,
        stubControls: Set<String>
    ): Result<Unit> {
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
        return client.publish(
            topic = topics.discovery,
            payload = payload,
            qos = 1,
            retain = true,
            expirySeconds = DISCOVERY_EXPIRY_SECONDS,
        )
    }

    /**
     * Publishes every active state. Only counts as published (for the "last
     * published" time and the test button) when the broker took all of them.
     */
    private suspend fun publishStates(): Boolean {
        val topics = Topics(settingsRepository.deviceId())
        val activeKeys = EntityCatalog.activeKeys(settingsRepository.enabledControls.first())
        val states = runCatching { telemetry.read() }.getOrDefault(emptyMap())
            .filterKeys { it in activeKeys }
        if (states.isEmpty()) return false

        val allPublished = states.map { (key, value) ->
            client.publish(
                topic = topics.state(key),
                payload = value,
                qos = 1,
                retain = true,
                expirySeconds = STATE_EXPIRY_SECONDS,
            ).isSuccess
        }.all { it }

        if (allPublished) status.markPublished()
        return allPublished
    }

    private fun appVersion(): String = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName
    }.getOrNull() ?: "unknown"

    private companion object {
        const val DISCOVERY_EXPIRY_SECONDS = 7 * 24 * 60 * 60L
        const val STATE_EXPIRY_SECONDS = 60 * 60L
    }
}
