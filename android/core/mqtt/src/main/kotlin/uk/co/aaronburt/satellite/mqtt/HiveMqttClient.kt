package uk.co.aaronburt.satellite.mqtt

import com.hivemq.client.mqtt.MqttClient as HiveMqClient
import com.hivemq.client.mqtt.datatypes.MqttQos
import com.hivemq.client.mqtt.mqtt5.Mqtt5AsyncClient
import uk.co.aaronburt.satellite.common.coroutines.DispatchersProvider
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class HiveMqttClient @Inject constructor(
    private val dispatchers: DispatchersProvider,
) : MqttClient {

    private val _connectionState = MutableStateFlow<ConnectionState>(ConnectionState.Disconnected)
    override val connectionState: StateFlow<ConnectionState> = _connectionState.asStateFlow()

    @Volatile
    private var client: Mqtt5AsyncClient? = null

    override suspend fun connect(
        settings: BrokerSettings,
        clientId: String,
        willTopic: String,
    ): Result<Unit> = withContext(dispatchers.io) {
        runCatching {
            disconnectInternal()

            val builder = HiveMqClient.builder()
                .useMqttVersion5()
                .identifier(clientId)
                .serverHost(settings.host)
                .serverPort(settings.port)
                .automaticReconnectWithDefaultConfig()
                .addConnectedListener { _ ->
                    _connectionState.value = ConnectionState.Connected(
                        broker = settings.serverUri,
                        connectedSinceEpochMillis = System.currentTimeMillis(),
                    )
                }
                .addDisconnectedListener { context ->
                    _connectionState.value = ConnectionState.Error(
                        context.cause?.message ?: "Disconnected from broker",
                    )
                }

            if (settings.useTls) {
                builder.sslWithDefaultConfig()
            }

            val newClient = builder.buildAsync()
            _connectionState.value = ConnectionState.Connecting

            val sendBuilder = newClient.connectWith()
                .cleanStart(false)
                .sessionExpiryInterval(SESSION_EXPIRY_SECONDS)
                .willPublish()
                    .topic(willTopic)
                    .payload(OFFLINE_PAYLOAD.toByteArray())
                    .qos(MqttQos.AT_LEAST_ONCE)
                    .retain(true)
                .applyWillPublish()

            val authBuilder = settings.username?.let { username ->
                sendBuilder.simpleAuth()
                    .username(username)
                    .password(settings.password.orEmpty().toByteArray())
                    .applySimpleAuth()
            } ?: sendBuilder

            authBuilder.send().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            client = newClient
        }.onFailure { error ->
            _connectionState.value = ConnectionState.Error(
                error.message ?: error::class.simpleName ?: "Connection failed",
            )
            disconnectInternal()
        }
    }

    override suspend fun disconnect() = withContext(dispatchers.io) {
        disconnectInternal()
        _connectionState.value = ConnectionState.Disconnected
    }

    override suspend fun publish(
        topic: String,
        payload: String,
        qos: Int,
        retain: Boolean,
        expirySeconds: Long?,
    ): Result<Unit> = withContext(dispatchers.io) {
        runCatching {
            val current = client ?: error("Not connected to a broker")
            val builder = current.publishWith()
                .topic(topic)
                .payload(payload.toByteArray())
                .qos(qos.toMqttQos())
                .retain(retain)
            if (expirySeconds != null) {
                builder.messageExpiryInterval(expirySeconds)
            }
            builder.send().get(PUBLISH_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            Unit
        }
    }

    override suspend fun subscribe(
        topicFilter: String,
        qos: Int,
        onMessage: (MqttMessage) -> Unit,
    ): Result<Unit> = withContext(dispatchers.io) {
        runCatching {
            val current = client ?: error("Not connected to a broker")
            current.subscribeWith()
                .topicFilter(topicFilter)
                .qos(qos.toMqttQos())
                .callback { publish ->
                    onMessage(
                        MqttMessage(
                            topic = publish.topic.toString(),
                            payload = String(publish.payloadAsBytes),
                        ),
                    )
                }
                .send()
                .get(SUBSCRIBE_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            Unit
        }
    }

    private fun disconnectInternal() {
        client?.let { runCatching { it.disconnect() } }
        client = null
    }

    private fun Int.toMqttQos(): MqttQos = when (this) {
        0 -> MqttQos.AT_MOST_ONCE
        2 -> MqttQos.EXACTLY_ONCE
        else -> MqttQos.AT_LEAST_ONCE
    }

    private companion object {
        const val OFFLINE_PAYLOAD = "offline"
        const val CONNECT_TIMEOUT_SECONDS = 15L
        const val PUBLISH_TIMEOUT_SECONDS = 10L
        const val SUBSCRIBE_TIMEOUT_SECONDS = 10L
        const val SESSION_EXPIRY_SECONDS = 3_600L
    }
}
