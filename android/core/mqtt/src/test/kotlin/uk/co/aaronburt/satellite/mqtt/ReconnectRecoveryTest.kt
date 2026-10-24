package uk.co.aaronburt.satellite.mqtt

import com.hivemq.client.mqtt.MqttClient as HiveMqClient
import com.hivemq.client.mqtt.datatypes.MqttQos
import com.hivemq.client.mqtt.mqtt5.Mqtt5BlockingClient
import java.util.UUID
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.TestDispatchers

/**
 * Issue #8: outage scenarios run against a real broker, with the real client
 * and connection manager. A Connected state alone is not proof of recovery, so
 * each test checks what another MQTT client (standing in for Home Assistant)
 * actually sees on the broker.
 */
class ReconnectRecoveryTest {

    private val broker = EmbeddedBroker().start()
    private val network = FlakyNetwork(broker.port)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val client = HiveMqttClient(TestDispatchers(Dispatchers.IO), 5)
    private val deviceId = "dev" + UUID.randomUUID().toString().take(6)
    private val topics = Topics(deviceId)
    private val observers = mutableListOf<Mqtt5BlockingClient>()

    private val settings = FakeSettingsRepository(
        broker = BrokerSettings(
            host = "127.0.0.1",
            port = network.port,
            username = null,
            password = null,
            useTls = false
        ),
        deviceIdValue = deviceId,
    )
    private val manager = MqttConnectionManager(client, settings, scope)

    @After
    fun tearDown() = runBlocking {
        scope.cancel()
        client.disconnect()
        observers.forEach { runCatching { it.disconnect() } }
        network.close()
        broker.close()
    }

    /** Records every availability payload the broker delivers, retained or live. */
    private fun watchAvailability(): CopyOnWriteArrayList<String> {
        val seen = CopyOnWriteArrayList<String>()
        val observer = connectObserver()
        observer.toAsync().subscribeWith()
            .topicFilter(topics.availability)
            .qos(MqttQos.AT_LEAST_ONCE)
            .callback { seen += String(it.payloadAsBytes) }
            .send()
            .get(5, TimeUnit.SECONDS)
        return seen
    }

    /** What a client subscribing now (e.g. Home Assistant after a restart) is told. */
    private fun retainedAvailability(): String? {
        val seen = CopyOnWriteArrayList<String>()
        val observer = connectObserver()
        observer.toAsync().subscribeWith()
            .topicFilter(topics.availability)
            .qos(MqttQos.AT_LEAST_ONCE)
            .callback { if (it.isRetain) seen += String(it.payloadAsBytes) }
            .send()
            .get(5, TimeUnit.SECONDS)
        waitUntil(2_000) { seen.isNotEmpty() }
        observer.disconnect()
        observers -= observer
        return seen.lastOrNull()
    }

    private fun connectObserver(): Mqtt5BlockingClient = HiveMqClient.builder()
        .useMqttVersion5()
        .identifier("obs-" + UUID.randomUUID().toString().take(8))
        .serverHost("127.0.0.1")
        .serverPort(broker.port) // straight to the broker, not through the flaky link
        .buildBlocking()
        .also {
            it.connect()
            observers += it
        }

    private fun sendCommand(topic: String, payload: String) {
        val sender = connectObserver()
        sender.publishWith().topic(topic).payload(payload.toByteArray()).qos(MqttQos.AT_LEAST_ONCE).send()
    }

    private suspend fun awaitState(predicate: (ConnectionState) -> Boolean) =
        withTimeout(60_000) { client.connectionState.first(predicate) }

    private suspend fun awaitConnected() = awaitState { it is ConnectionState.Connected }

    @Test
    fun `network loss publishes the offline will, then retained online replaces it without user action`() =
        runBlocking {
            val seen = watchAvailability()
            manager.start()
            awaitConnected()
            waitUntil { seen.lastOrNull() == "online" }

            network.cut()
            waitUntil { seen.lastOrNull() == "offline" }
            awaitConnected()
            waitUntil { seen.lastOrNull() == "online" }

            assertEquals("online", retainedAvailability())
        }

    @Test
    fun `availability recovers after the broker restarts and loses its retained messages`() = runBlocking {
        manager.start()
        awaitConnected()
        waitUntil { retainedAvailability() == "online" }

        broker.stop()
        awaitState { it is ConnectionState.Error }
        broker.start() // in-memory persistence: the broker comes back empty
        awaitConnected()

        waitUntil(10_000) { retainedAvailability() == "online" }
    }

    @Test
    fun `a command is handled exactly once after repeated reconnects`() = runBlocking {
        // Mirrors SatelliteReporter: subscribe again on every Connected.
        val handled = CopyOnWriteArrayList<String>()
        val subscriptions = CopyOnWriteArrayList<Unit>()
        scope.launch {
            client.connectionState.collect { state ->
                if (state is ConnectionState.Connected) {
                    client.subscribe(topics.commandWildcard, qos = 1) { handled += it.payload }
                        .onSuccess { subscriptions += Unit }
                }
            }
        }
        manager.start()
        awaitConnected()
        waitUntil { subscriptions.size == 1 }

        repeat(3) { round ->
            network.cut()
            awaitState { it !is ConnectionState.Connected }
            awaitConnected()
            waitUntil { subscriptions.size == round + 2 }
        }

        sendCommand(topics.command("volume"), "40")
        waitUntil { handled.isNotEmpty() }
        Thread.sleep(1_000) // leave time for a duplicate delivery to show up

        assertEquals(listOf("40"), handled)
    }

    @Test
    fun `stopping cleanly leaves offline retained, not online`() = runBlocking {
        manager.start()
        awaitConnected()
        waitUntil { retainedAvailability() == "online" }

        manager.stop()

        waitUntil { retainedAvailability() == "offline" }
        assertTrue(client.connectionState.value !is ConnectionState.Connected)
    }

    private fun waitUntil(timeoutMillis: Long = 30_000, condition: () -> Boolean) {
        val deadline = System.currentTimeMillis() + timeoutMillis
        while (!condition()) {
            check(System.currentTimeMillis() < deadline) { "timed out" }
            Thread.sleep(50)
        }
    }
}
