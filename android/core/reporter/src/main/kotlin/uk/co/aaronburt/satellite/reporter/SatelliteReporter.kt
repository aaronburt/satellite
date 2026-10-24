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
import kotlinx.coroutines.sync.Mutex
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
import uk.co.aaronburt.satellite.update.NoUpdateStates
import uk.co.aaronburt.satellite.update.UpdateStatesSource

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
@Suppress("LongParameterList", "TooManyFunctions") // Hilt-injected collaborators; one reporter lifecycle
class SatelliteReporter @Inject constructor(
    private val client: MqttClient,
    private val settingsRepository: SettingsRepository,
    private val telemetry: DeviceStateReader,
    private val commandRouter: CommandRouter,
    private val audioControls: AudioControls,
    private val status: ReporterStatus,
    @ApplicationContext private val context: Context,
    @ApplicationScope private val appScope: CoroutineScope,
    private val updateStates: UpdateStatesSource = NoUpdateStates
) {

    private var observerJob: Job? = null
    private var periodicJob: Job? = null
    private var modeJob: Job? = null
    private var controlsJob: Job? = null
    private var updateJob: Job? = null

    /** Held while a Home Assistant refresh request runs, so repeats cannot pile up. */
    private val refreshGate = Mutex()

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

        // The update flow only runs on an explicit check, so its progress is
        // pushed as it happens instead of waiting for the next interval.
        updateJob = appScope.launch {
            updateStates.changes.collect {
                if (client.connectionState.value is ConnectionState.Connected) publishStates()
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

                val cleared = removed + EntityCatalog.companionEntities
                    .filter { it.requires in removed }
                    .map { it.key }
                cleared.forEach { key ->
                    client.publish(topics.state(key), "", qos = 1, retain = true)
                }

                publishDiscovery(deviceId, topics)
                publishStates()
            }
        }
    }

    fun stop() {
        updateJob?.cancel()
        updateJob = null
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
        return republishAll(Topics(settingsRepository.deviceId())) == RefreshOutcome.OK
    }

    private suspend fun republishAll(topics: Topics): RefreshOutcome {
        // A control disabled while offline could not clear its retained state
        // at the time, so clear every inactive control's state on each connect.
        EntityCatalog.unexposedKeys(settingsRepository.enabledControls.first())
            .forEach { client.publish(topics.state(it), "", qos = 1, retain = true) }
        val discovered = publishDiscovery(settingsRepository.deviceId(), topics)
        val published = publishStates()
        return when {
            discovered && published -> RefreshOutcome.OK
            discovered || published -> RefreshOutcome.PARTIAL
            else -> RefreshOutcome.FAILED
        }
    }

    /**
     * Serves the Home Assistant "Refresh discovery" button. A press that
     * arrives while a refresh (or its cooldown) is running is dropped rather
     * than queued, so mashing the button republishes once. Reuses the existing
     * subscriptions and reports the outcome on the refresh result sensor.
     */
    private suspend fun refreshFromCommand(topics: Topics) {
        if (!refreshGate.tryLock()) return
        try {
            val outcome = if (client.connectionState.value is ConnectionState.Connected) {
                republishAll(topics)
            } else {
                RefreshOutcome.FAILED
            }
            client.publish(topics.state(EntityCatalog.REFRESH_RESULT), outcome.payload, qos = 1, retain = true)
            delay(REFRESH_COOLDOWN_MILLIS)
        } finally {
            refreshGate.unlock()
        }
    }

    private enum class RefreshOutcome(val payload: String) {
        OK("ok"),
        PARTIAL("partial"),
        FAILED("failed")
    }

    private suspend fun onConnected(): Boolean {
        val deviceId = settingsRepository.deviceId()
        val topics = Topics(deviceId)

        val outcome = republishAll(topics)

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
                if (!accepted) return@launch
                if (CommandParser.parse(message.topic, message.payload, topics) == SatelliteCommand.RefreshDiscovery) {
                    refreshFromCommand(topics)
                } else {
                    publishStates()
                }
            }
        }

        return outcome == RefreshOutcome.OK
    }

    /**
     * Deliberately removes this device from Home Assistant: deletes the
     * retained discovery config (which makes Home Assistant remove the device
     * and its entities from its registry), every retained state topic, and the
     * retained availability. Call it before switching MQTT off or uninstalling;
     * the next [refresh] or reconnect re-creates the device under the same ID.
     * Returns false when not connected or the broker refused any deletion.
     */
    suspend fun removeFromHomeAssistant(): Boolean {
        if (client.connectionState.value !is ConnectionState.Connected) return false
        val topics = Topics(settingsRepository.deviceId())
        val results = buildList {
            add(client.publish(topics.discovery, "", qos = 1, retain = true))
            EntityCatalog.all.forEach { spec ->
                add(client.publish(topics.state(spec.key), "", qos = 1, retain = true))
            }
            add(client.publish(topics.availability, "", qos = 1, retain = true))
        }
        return results.all { it.isSuccess }
    }

    private suspend fun publishDiscovery(deviceId: String, topics: Topics): Boolean {
        val enabledControls = settingsRepository.enabledControls.first()
        val unexposed = EntityCatalog.unexposedKeys(enabledControls)

        if (unexposed.isNotEmpty()) {
            publishDiscoveryPayload(deviceId, topics, enabledControls, unexposed)
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
        val states = runCatching { telemetry.read() + updateStates.read() }.getOrDefault(emptyMap())
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

    companion object {
        const val DISCOVERY_EXPIRY_SECONDS = 7 * 24 * 60 * 60L
        const val STATE_EXPIRY_SECONDS = 60 * 60L
        const val REFRESH_COOLDOWN_MILLIS = 2_000L
    }
}
