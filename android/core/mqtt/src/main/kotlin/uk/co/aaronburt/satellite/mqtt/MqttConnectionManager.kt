package uk.co.aaronburt.satellite.mqtt

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import uk.co.aaronburt.satellite.common.coroutines.ApplicationScope
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState

/**
 * Owns the MQTT connection lifecycle. Observes saved settings and connects
 * whenever they change, so saving the settings screen is enough to (re)connect.
 *
 * Availability follows the connection: `online` (retained) is published on
 * every successful connect, including the client's automatic reconnects, so
 * Home Assistant recovers after the broker has published our `offline` will.
 *
 * [start] is idempotent: the foreground service can call it on every start
 * without spawning duplicate collectors. [start] after [stop] waits for the
 * stop to finish, so a quick off/on never leaves the connection closed.
 */
@Singleton
class MqttConnectionManager @Inject constructor(
    private val client: MqttClient,
    private val settingsRepository: SettingsRepository,
    @ApplicationScope private val appScope: CoroutineScope
) {

    val connectionState: StateFlow<ConnectionState> = client.connectionState

    @Volatile
    private var currentSettings: BrokerSettings? = null
    private var startJob: Job? = null
    private var stopJob: Job? = null

    @Synchronized
    fun start() {
        if (startJob?.isActive == true) return

        val pendingStop = stopJob
        startJob = appScope.launch {
            pendingStop?.join()

            launch { announceOnlineOnConnect() }

            settingsRepository.brokerSettings.collect { settings ->
                when {
                    settings == null -> {
                        currentSettings = null
                        client.disconnect()
                    }
                    settings != currentSettings -> {
                        currentSettings = settings
                        connect(settings)
                    }
                }
            }
        }
    }

    @Synchronized
    fun stop() {
        val running = startJob
        startJob = null
        val previousStop = stopJob

        stopJob = appScope.launch {
            previousStop?.join()
            running?.cancelAndJoin()
            if (currentSettings != null) {
                client.publish(availabilityTopic(), OFFLINE, qos = 1, retain = true)
            }
            client.disconnect()
            currentSettings = null
        }
    }

    suspend fun reconnect() {
        currentSettings?.let { connect(it) }
    }

    private suspend fun announceOnlineOnConnect() {
        client.connectionState.collect { state ->
            if (state is ConnectionState.Connected) {
                client.publish(availabilityTopic(), ONLINE, qos = 1, retain = true)
            }
        }
    }

    private suspend fun connect(settings: BrokerSettings) {
        val deviceId = settingsRepository.deviceId()
        client.connect(
            settings = settings,
            clientId = clientId(deviceId),
            willTopic = Topics(deviceId).availability,
        )
    }

    private suspend fun availabilityTopic(): String = Topics(settingsRepository.deviceId()).availability

    companion object {
        const val ONLINE = "online"
        const val OFFLINE = "offline"

        fun clientId(deviceId: String): String = "satelliteandroid-$deviceId"
    }
}
