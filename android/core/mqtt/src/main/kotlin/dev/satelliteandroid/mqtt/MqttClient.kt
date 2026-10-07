package dev.satelliteandroid.mqtt

import dev.satelliteandroid.model.BrokerSettings
import dev.satelliteandroid.model.ConnectionState
import kotlinx.coroutines.flow.StateFlow

data class MqttMessage(
    val topic: String,
    val payload: String,
)

/**
 * Thin abstraction over the MQTT client so the concrete library (HiveMQ) can be
 * swapped without touching the rest of the app.
 */
interface MqttClient {

    val connectionState: StateFlow<ConnectionState>

    /**
     * Connects to [settings]. [willTopic] is registered as the Last Will and
     * Testament topic (published as `offline`, retained, by the broker if the
     * connection drops).
     */
    suspend fun connect(settings: BrokerSettings, clientId: String, willTopic: String): Result<Unit>

    suspend fun disconnect()

    suspend fun publish(
        topic: String,
        payload: String,
        qos: Int = 1,
        retain: Boolean = false,
        expirySeconds: Long? = null,
    ): Result<Unit>

    suspend fun subscribe(
        topicFilter: String,
        qos: Int = 0,
        onMessage: (MqttMessage) -> Unit,
    ): Result<Unit>
}
