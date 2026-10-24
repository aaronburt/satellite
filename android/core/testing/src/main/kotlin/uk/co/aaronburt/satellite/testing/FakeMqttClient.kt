package uk.co.aaronburt.satellite.testing

import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.mqtt.MqttClient
import uk.co.aaronburt.satellite.mqtt.MqttMessage

/**
 * Records everything sent to the broker and lets tests drive the connection
 * state and deliver incoming messages.
 *
 * Like the real client: publishing or subscribing while not connected fails,
 * and subscribing to the same filter again replaces its handler.
 */
class FakeMqttClient : MqttClient {

    data class Publish(
        val topic: String,
        val payload: String,
        val qos: Int,
        val retain: Boolean,
        val expirySeconds: Long?
    )

    data class Connect(val settings: BrokerSettings, val clientId: String, val willTopic: String)

    private val state = MutableStateFlow<ConnectionState>(ConnectionState.Disconnected)
    override val connectionState: StateFlow<ConnectionState> = state.asStateFlow()

    /** Every call in order, e.g. `connect`, `publish satellite/x/availability online`, `disconnect`. */
    val calls = CopyOnWriteArrayList<String>()
    val published = CopyOnWriteArrayList<Publish>()
    val connects = CopyOnWriteArrayList<Connect>()
    val subscribeCalls = CopyOnWriteArrayList<String>()
    private val handlers = ConcurrentHashMap<String, (MqttMessage) -> Unit>()

    /** When false, [connect] fails and leaves the state as [ConnectionState.Error]. */
    var connectSucceeds = true

    /** Topics for which [publish] reports failure even while connected. */
    var failingTopics: (String) -> Boolean = { false }

    /**
     * Virtual time each call takes, to model network round trips. With a test
     * dispatcher this lets other coroutines interleave mid-call, which is
     * where ordering bugs show up.
     */
    var latencyMillis = 0L

    private suspend fun network() {
        if (latencyMillis > 0) delay(latencyMillis)
    }

    private var connectedSince = 0L

    override suspend fun connect(settings: BrokerSettings, clientId: String, willTopic: String): Result<Unit> {
        network()
        calls += "connect ${settings.host}"
        connects += Connect(settings, clientId, willTopic)
        return if (connectSucceeds) {
            setConnected(settings.serverUri)
            Result.success(Unit)
        } else {
            state.value = ConnectionState.Error("refused")
            Result.failure(IllegalStateException("refused"))
        }
    }

    override suspend fun disconnect() {
        network()
        calls += "disconnect"
        state.value = ConnectionState.Disconnected
    }

    override suspend fun publish(
        topic: String,
        payload: String,
        qos: Int,
        retain: Boolean,
        expirySeconds: Long?
    ): Result<Unit> {
        network()
        if (state.value !is ConnectionState.Connected) {
            calls += "publish-failed $topic"
            return Result.failure(IllegalStateException("Not connected to a broker"))
        }
        if (failingTopics(topic)) {
            calls += "publish-failed $topic"
            return Result.failure(IllegalStateException("broker rejected $topic"))
        }
        calls += "publish $topic $payload"
        published += Publish(topic, payload, qos, retain, expirySeconds)
        return Result.success(Unit)
    }

    override suspend fun subscribe(topicFilter: String, qos: Int, onMessage: (MqttMessage) -> Unit): Result<Unit> {
        if (state.value !is ConnectionState.Connected) {
            return Result.failure(IllegalStateException("Not connected to a broker"))
        }
        calls += "subscribe $topicFilter"
        subscribeCalls += topicFilter
        handlers[topicFilter] = onMessage
        return Result.success(Unit)
    }

    /** Simulates the broker connection coming up (e.g. an automatic reconnect). */
    fun setConnected(broker: String = "tcp://broker.local:1883") {
        connectedSince += 1
        state.value = ConnectionState.Connected(broker, connectedSince)
    }

    /** Simulates the connection dropping. */
    fun dropConnection(message: String = "connection lost") {
        state.value = ConnectionState.Error(message)
    }

    /** Delivers an incoming message to every handler whose filter matches. */
    fun deliver(topic: String, payload: String) {
        handlers.filterKeys { matches(it, topic) }.values.forEach { it(MqttMessage(topic, payload)) }
    }

    /** The last payload published to [topic], or null. */
    fun lastPayload(topic: String): String? = published.lastOrNull { it.topic == topic }?.payload

    fun publishesTo(topic: String): List<Publish> = published.filter { it.topic == topic }

    fun clearRecords() {
        calls.clear()
        published.clear()
        connects.clear()
        subscribeCalls.clear()
    }

    companion object {
        /** MQTT topic filter matching with `+` and `#` wildcards. */
        fun matches(filter: String, topic: String): Boolean {
            val f = filter.split('/')
            val t = topic.split('/')
            for (i in f.indices) {
                when (f[i]) {
                    "#" -> return true
                    "+" -> if (i >= t.size) return false
                    else -> if (i >= t.size || f[i] != t[i]) return false
                }
            }
            return f.size == t.size
        }
    }
}
