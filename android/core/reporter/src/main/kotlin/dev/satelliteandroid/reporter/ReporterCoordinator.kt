package dev.satelliteandroid.reporter

import dev.satelliteandroid.common.coroutines.ApplicationScope
import dev.satelliteandroid.datastore.SettingsRepository
import dev.satelliteandroid.model.Transport
import dev.satelliteandroid.mqtt.MqttConnectionManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.launch
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Starts the reporter that matches the selected [Transport] and stops the other,
 * so switching between MQTT and webhook is immediate and leak-free.
 */
@Singleton
class ReporterCoordinator @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val mqttReporter: SatelliteReporter,
    private val webhookReporter: WebhookReporter,
    private val connectionManager: MqttConnectionManager,
    @ApplicationScope private val appScope: CoroutineScope,
) {

    private var job: Job? = null

    @Volatile
    private var currentTransport: Transport = Transport.MQTT

    fun start() {
        if (job?.isActive == true) return

        job = appScope.launch {
            settingsRepository.transport.collect { transport ->
                currentTransport = transport
                when (transport) {
                    Transport.MQTT -> {
                        webhookReporter.stop()
                        connectionManager.start()
                        mqttReporter.start()
                    }
                    Transport.WEBHOOK -> {
                        mqttReporter.stop()
                        connectionManager.stop()
                        webhookReporter.start()
                    }
                }
            }
        }
    }

    fun stop() {
        job?.cancel()
        job = null
        mqttReporter.stop()
        webhookReporter.stop()
        connectionManager.stop()
    }

    /**
     * Fires one payload immediately using the active transport. Returns false if
     * it could not be delivered (not connected / bad URL).
     */
    suspend fun testNow(): Boolean = when (currentTransport) {
        Transport.MQTT -> mqttReporter.refresh()
        Transport.WEBHOOK -> webhookReporter.refresh()
    }
}
