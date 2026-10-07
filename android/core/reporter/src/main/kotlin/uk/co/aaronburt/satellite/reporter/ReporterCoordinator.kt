package uk.co.aaronburt.satellite.reporter

import uk.co.aaronburt.satellite.common.coroutines.ApplicationScope
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.mqtt.MqttConnectionManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.launch
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Starts the reporter that matches the selected [Transport] and stops the other,
 * so switching between MQTT and webhook is immediate and leak-free.
 *
 * Also honours the master [SettingsRepository.reportingEnabled] switch: when it
 * is off everything is stopped and the MQTT connection is closed (which publishes
 * a retained `offline` to the availability topic).
 */
@Singleton
class ReporterCoordinator @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val mqttReporter: SatelliteReporter,
    private val webhookReporter: WebhookReporter,
    private val connectionManager: MqttConnectionManager,
    private val status: ReporterStatus,
    @ApplicationScope private val appScope: CoroutineScope,
) {

    private var job: Job? = null

    @Volatile
    private var currentTransport: Transport = Transport.MQTT

    fun start() {
        if (job?.isActive == true) return

        job = appScope.launch {
            combine(
                settingsRepository.transport,
                settingsRepository.reportingEnabled,
            ) { transport, enabled -> transport to enabled }
                .collect { (transport, enabled) -> apply(transport, enabled) }
        }
    }

    fun stop() {
        job?.cancel()
        job = null
        stopAll()
    }

    /**
     * Fires one payload immediately using the active transport. Returns false if
     * it could not be delivered (not connected / bad URL / reporting disabled).
     */
    suspend fun testNow(): Boolean = when (currentTransport) {
        Transport.MQTT -> mqttReporter.refresh()
        Transport.WEBHOOK -> webhookReporter.refresh()
    }

    private fun apply(transport: Transport, enabled: Boolean) {
        currentTransport = transport

        if (!enabled) {
            stopAll()
            status.reset()
            return
        }

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

    private fun stopAll() {
        mqttReporter.stop()
        webhookReporter.stop()
        connectionManager.stop()
    }
}
