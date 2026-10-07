package uk.co.aaronburt.satellite.mqtt

import uk.co.aaronburt.satellite.common.coroutines.ApplicationScope
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Owns the MQTT connection lifecycle. Observes saved settings and connects
 * whenever they change, so saving the settings screen is enough to (re)connect.
 *
 * [start] is idempotent: the foreground service can call it on every start
 * without spawning duplicate collectors.
 */
@Singleton
class MqttConnectionManager @Inject constructor(
    private val client: MqttClient,
    private val settingsRepository: SettingsRepository,
    @ApplicationScope private val appScope: CoroutineScope,
) {

    val connectionState: StateFlow<ConnectionState> = client.connectionState

    private var currentSettings: BrokerSettings? = null
    private var startJob: Job? = null

    fun start() {
        if (startJob?.isActive == true) return

        startJob = appScope.launch {
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

    fun stop() {
        startJob?.cancel()
        startJob = null

        appScope.launch {
            val settings = currentSettings
            if (settings != null) {
                val baseTopic = "satellite/${settingsRepository.deviceId()}"
                // Publish availability down before a clean disconnect (a clean
                // MQTT disconnect does not trigger the will).
                client.publish("$baseTopic/availability", "offline", qos = 1, retain = true)
            }
            client.disconnect()
            currentSettings = null
        }
    }

    suspend fun reconnect() {
        currentSettings?.let { connect(it) }
    }

    private suspend fun connect(settings: BrokerSettings) {
        val deviceId = settingsRepository.deviceId()
        val baseTopic = "satellite/$deviceId"

        client.connect(
            settings = settings,
            clientId = "satelliteandroid-$deviceId",
            willTopic = "$baseTopic/availability",
        ).onSuccess {
            client.publish(
                topic = "$baseTopic/availability",
                payload = "online",
                qos = 1,
                retain = true,
            )
        }
    }
}
