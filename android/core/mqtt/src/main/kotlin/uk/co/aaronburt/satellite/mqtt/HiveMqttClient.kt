package uk.co.aaronburt.satellite.mqtt

import com.hivemq.client.mqtt.MqttClient as HiveMqClient
import com.hivemq.client.mqtt.MqttGlobalPublishFilter
import com.hivemq.client.mqtt.datatypes.MqttQos
import com.hivemq.client.mqtt.datatypes.MqttTopicFilter
import com.hivemq.client.mqtt.lifecycle.MqttDisconnectSource
import com.hivemq.client.mqtt.mqtt5.Mqtt5AsyncClient
import com.hivemq.client.mqtt.mqtt5.message.publish.Mqtt5Publish
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext
import uk.co.aaronburt.satellite.common.coroutines.DispatchersProvider
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState

@Singleton
class HiveMqttClient internal constructor(
    private val dispatchers: DispatchersProvider,
    private val connectTimeoutSeconds: Long
) : MqttClient {

    @Inject
    constructor(dispatchers: DispatchersProvider) : this(dispatchers, CONNECT_TIMEOUT_SECONDS)

    private val _connectionState = MutableStateFlow<ConnectionState>(ConnectionState.Disconnected)
    override val connectionState: StateFlow<ConnectionState> = _connectionState.asStateFlow()

    @Volatile
    private var client: Mqtt5AsyncClient? = null

    /**
     * Message handlers by topic filter. Every incoming message goes through one
     * callback per client that dispatches to these, so subscribing to the same
     * filter again (which callers do after every reconnect) replaces the handler
     * instead of delivering each message twice.
     */
    private val handlers = ConcurrentHashMap<String, Subscription>()

    private class Subscription(val filter: MqttTopicFilter, val onMessage: (MqttMessage) -> Unit)

    override suspend fun connect(settings: BrokerSettings, clientId: String, willTopic: String): Result<Unit> =
        withContext(dispatchers.io) {
            disconnectInternal()

            // Listeners only report for the client that is current. A replaced or
            // closed client can still fire late events, and they must not overwrite
            // the state of its successor.
            var newClient: Mqtt5AsyncClient? = null

            runCatching {
                val builder = HiveMqClient.builder()
                    .useMqttVersion5()
                    .identifier(clientId)
                    .serverHost(settings.host)
                    .serverPort(settings.port)
                    .automaticReconnectWithDefaultConfig()
                    .addConnectedListener { _ ->
                        if (client === newClient) {
                            _connectionState.value = ConnectionState.Connected(
                                broker = settings.serverUri,
                                connectedSinceEpochMillis = System.currentTimeMillis(),
                            )
                        }
                    }
                    .addDisconnectedListener { context ->
                        if (client === newClient && context.source != MqttDisconnectSource.USER) {
                            _connectionState.value = ConnectionState.Error(
                                context.cause.message ?: "Disconnected from broker",
                            )
                        }
                    }

                if (settings.useTls) {
                    builder.sslWithDefaultConfig()
                }

                val built = builder.buildAsync()
                built.publishes(MqttGlobalPublishFilter.ALL, ::dispatch)
                newClient = built
                // Assigned before connecting: the connected listener can run (and
                // callers can start publishing) before send() returns.
                client = built
                _connectionState.value = ConnectionState.Connecting

                val sendBuilder = built.connectWith()
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

                authBuilder.send().get(connectTimeoutSeconds, TimeUnit.SECONDS)
                Unit
            }.onFailure { error ->
                // The client stays alive: automatic reconnect keeps retrying, so a
                // broker that is unreachable at boot is picked up once it comes back.
                // A failed attempt has usually already reported its own, more
                // specific, error (e.g. connection refused); keep that one.
                if (client === newClient && _connectionState.value !is ConnectionState.Error) {
                    _connectionState.value = ConnectionState.Error(
                        if (error is TimeoutException) {
                            "No response from ${settings.serverUri}"
                        } else {
                            error.rootMessage() ?: "Connection failed"
                        },
                    )
                }
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
        expirySeconds: Long?
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

    override suspend fun subscribe(topicFilter: String, qos: Int, onMessage: (MqttMessage) -> Unit): Result<Unit> =
        withContext(dispatchers.io) {
            runCatching {
                val current = client ?: error("Not connected to a broker")
                handlers[topicFilter] = Subscription(MqttTopicFilter.of(topicFilter), onMessage)
                current.subscribeWith()
                    .topicFilter(topicFilter)
                    .qos(qos.toMqttQos())
                    .send()
                    .get(SUBSCRIBE_TIMEOUT_SECONDS, TimeUnit.SECONDS)
                Unit
            }
        }

    private fun dispatch(publish: Mqtt5Publish) {
        val message = MqttMessage(
            topic = publish.topic.toString(),
            payload = String(publish.payloadAsBytes, Charsets.UTF_8),
        )
        handlers.values
            .filter { it.filter.matches(publish.topic) }
            .forEach { subscription -> runCatching { subscription.onMessage(message) } }
    }

    private fun disconnectInternal() {
        val current = client
        client = null
        handlers.clear()
        current?.let { runCatching { it.disconnect() } }
    }

    private fun Int.toMqttQos(): MqttQos = when (this) {
        0 -> MqttQos.AT_MOST_ONCE
        2 -> MqttQos.EXACTLY_ONCE
        else -> MqttQos.AT_LEAST_ONCE
    }

    private fun Throwable.rootMessage(): String? {
        var current: Throwable = this
        while (current.message == null || current is java.util.concurrent.ExecutionException) {
            current = current.cause ?: break
        }
        return current.message ?: this::class.simpleName
    }

    private companion object {
        const val OFFLINE_PAYLOAD = "offline"
        const val CONNECT_TIMEOUT_SECONDS = 15L
        const val PUBLISH_TIMEOUT_SECONDS = 10L
        const val SUBSCRIBE_TIMEOUT_SECONDS = 10L
        const val SESSION_EXPIRY_SECONDS = 3_600L
    }
}
