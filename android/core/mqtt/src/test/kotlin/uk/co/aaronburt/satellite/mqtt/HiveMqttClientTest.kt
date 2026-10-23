package uk.co.aaronburt.satellite.mqtt

import com.hivemq.client.mqtt.datatypes.MqttQos
import com.hivemq.client.mqtt.mqtt5.Mqtt5BlockingClient
import com.hivemq.client.mqtt.mqtt5.message.publish.Mqtt5Publish
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.testing.TestDispatchers
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.After
import org.junit.AfterClass
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.BeforeClass
import org.junit.Test
import java.util.UUID
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import com.hivemq.client.mqtt.MqttClient as HiveMqClient

/** Runs the real client against a real broker. */
class HiveMqttClientTest {

    private val clients = mutableListOf<HiveMqttClient>()
    private val observers = mutableListOf<Mqtt5BlockingClient>()

    private fun newClient(connectTimeoutSeconds: Long = 15) =
        HiveMqttClient(TestDispatchers(Dispatchers.IO), connectTimeoutSeconds).also { clients += it }

    private fun settings(port: Int = broker.port) = BrokerSettings(
        host = "127.0.0.1",
        port = port,
        username = null,
        password = null,
        useTls = false,
    )

    private fun uniqueId() = "test-" + UUID.randomUUID().toString().take(8)

    /** A second, independent client that records what the broker delivers. */
    private fun observe(filter: String): LinkedBlockingQueue<Mqtt5Publish> {
        val observer = HiveMqClient.builder()
            .useMqttVersion5()
            .identifier(uniqueId())
            .serverHost("127.0.0.1")
            .serverPort(broker.port)
            .buildBlocking()
        observers += observer
        observer.connect()
        val queue = LinkedBlockingQueue<Mqtt5Publish>()
        observer.toAsync().subscribeWith()
            .topicFilter(filter)
            .qos(MqttQos.AT_LEAST_ONCE)
            .callback { queue.add(it) }
            .send()
            .get(5, TimeUnit.SECONDS)
        return queue
    }

    private fun publishFromBroker(topic: String, payload: String) {
        val sender = HiveMqClient.builder()
            .useMqttVersion5()
            .identifier(uniqueId())
            .serverHost("127.0.0.1")
            .serverPort(broker.port)
            .buildBlocking()
        sender.connect()
        sender.publishWith().topic(topic).payload(payload.toByteArray()).qos(MqttQos.AT_LEAST_ONCE).send()
        sender.disconnect()
    }

    private suspend fun HiveMqttClient.awaitConnected(): ConnectionState.Connected =
        withTimeout(60_000) { connectionState.first { it is ConnectionState.Connected } } as ConnectionState.Connected

    @After
    fun tearDown() = runBlocking {
        clients.forEach { it.disconnect() }
        observers.forEach { runCatching { it.disconnect() } }
    }

    @Test
    fun `connects and reports the broker it is connected to`() = runBlocking {
        val client = newClient()
        val result = client.connect(settings(), uniqueId(), "satellite/t/availability")

        assertTrue(result.exceptionOrNull()?.toString(), result.isSuccess)
        val state = client.awaitConnected()
        assertEquals("tcp://127.0.0.1:${broker.port}", state.broker)
    }

    @Test
    fun `publishes reach other subscribers with retain and qos intact`() = runBlocking {
        val topic = "satellite/${uniqueId()}/state/battery"
        val received = observe(topic)
        val client = newClient()
        client.connect(settings(), uniqueId(), "satellite/t/availability")
        client.awaitConnected()

        val result = client.publish(topic, "87", qos = 1, retain = true, expirySeconds = 3600)

        assertTrue(result.isSuccess)
        val message = received.poll(5, TimeUnit.SECONDS)!!
        assertEquals("87", String(message.payloadAsBytes))
        assertEquals(MqttQos.AT_LEAST_ONCE, message.qos)
        assertTrue("retained messages must stay on the broker", client.retainedCheck(topic, "87"))
    }

    /** Subscribes with a fresh client: a retained message is delivered immediately. */
    private fun HiveMqttClient.retainedCheck(topic: String, expected: String): Boolean {
        val queue = observe(topic)
        return queue.poll(5, TimeUnit.SECONDS)?.let { String(it.payloadAsBytes) == expected && it.isRetain } ?: false
    }

    @Test
    fun `incoming messages are delivered to matching wildcard subscriptions`() = runBlocking {
        val base = "satellite/${uniqueId()}"
        val client = newClient()
        client.connect(settings(), uniqueId(), "$base/availability")
        client.awaitConnected()
        val received = LinkedBlockingQueue<MqttMessage>()

        client.subscribe("$base/cmd/#", qos = 1) { received.add(it) }.getOrThrow()
        publishFromBroker("$base/cmd/volume", "40")
        publishFromBroker("$base/other", "ignored")
        publishFromBroker("$base/cmd/mute", "ON")

        assertEquals(MqttMessage("$base/cmd/volume", "40"), received.poll(5, TimeUnit.SECONDS))
        assertEquals(MqttMessage("$base/cmd/mute", "ON"), received.poll(5, TimeUnit.SECONDS))
        assertNull(received.poll(300, TimeUnit.MILLISECONDS))
    }

    @Test
    fun `subscribing to the same filter again delivers each message once, to the newest handler`() = runBlocking {
        // The reporter subscribes again after every reconnect. Before the fix
        // each subscription added a callback, so commands ran once per reconnect.
        val base = "satellite/${uniqueId()}"
        val client = newClient()
        client.connect(settings(), uniqueId(), "$base/availability")
        client.awaitConnected()
        val first = CopyOnWriteArrayList<String>()
        val second = CopyOnWriteArrayList<String>()

        client.subscribe("$base/cmd/#", qos = 1) { first += it.payload }.getOrThrow()
        client.subscribe("$base/cmd/#", qos = 1) { second += it.payload }.getOrThrow()
        publishFromBroker("$base/cmd/volume", "55")

        waitUntil { second.isNotEmpty() }
        Thread.sleep(300)
        assertEquals(listOf("55"), second)
        assertTrue(first.isEmpty())
    }

    @Test
    fun `payloads are read as UTF-8`() = runBlocking {
        val base = "satellite/${uniqueId()}"
        val client = newClient()
        client.connect(settings(), uniqueId(), "$base/availability")
        client.awaitConnected()
        val received = LinkedBlockingQueue<MqttMessage>()
        client.subscribe("$base/name", qos = 1) { received.add(it) }.getOrThrow()

        publishFromBroker("$base/name", "Küche ☕")

        assertEquals("Küche ☕", received.poll(5, TimeUnit.SECONDS)?.payload)
    }

    @Test
    fun `publish and subscribe fail cleanly when not connected`() = runBlocking {
        val client = newClient()

        val publish = client.publish("a/b", "x")
        val subscribe = client.subscribe("a/#") {}

        assertTrue(publish.isFailure)
        assertEquals("Not connected to a broker", publish.exceptionOrNull()?.message)
        assertTrue(subscribe.isFailure)
    }

    @Test
    fun `disconnect is reported as disconnected, not as an error`() = runBlocking {
        val client = newClient()
        client.connect(settings(), uniqueId(), "satellite/t/availability")
        client.awaitConnected()

        client.disconnect()
        Thread.sleep(500) // give a late listener callback the chance to misreport

        assertEquals(ConnectionState.Disconnected, client.connectionState.value)
        assertTrue(client.publish("a/b", "x").isFailure)
    }

    @Test
    fun `a clean disconnect does not trigger the offline will`() = runBlocking {
        val will = "satellite/${uniqueId()}/availability"
        val received = observe(will)
        val client = newClient()
        client.connect(settings(), uniqueId(), will)
        client.awaitConnected()

        client.disconnect()

        assertNull(received.poll(1, TimeUnit.SECONDS))
    }

    @Test
    fun `connecting again replaces the old connection without the old one reporting an error`() = runBlocking {
        val client = newClient()
        client.connect(settings(), uniqueId(), "satellite/t/availability")
        client.awaitConnected()

        client.connect(settings(), uniqueId(), "satellite/t/availability")
        client.awaitConnected()
        Thread.sleep(500) // the replaced client's disconnect callback fires late

        assertTrue(client.connectionState.value is ConnectionState.Connected)
        assertTrue(client.publish("satellite/t/state/x", "1").isSuccess)
    }

    @Test
    fun `an unreachable broker reports an error, then connects once the broker is up`() = runBlocking {
        // At boot the network is often not ready yet. The client used to give up
        // after the first failure and never connect until the settings changed.
        EmbeddedBroker().use { late ->
            val client = newClient(connectTimeoutSeconds = 1)
            val result = client.connect(settings(late.port), uniqueId(), "satellite/t/availability")

            assertFalse(result.isSuccess)
            val error = client.connectionState.value
            assertTrue("state was $error", error is ConnectionState.Error)
            assertTrue((error as ConnectionState.Error).message.isNotBlank())

            late.start()
            client.awaitConnected()
            // Connected must mean usable: the old code reported the orphaned
            // reconnecting client as connected while every publish failed.
            assertTrue(client.publish("satellite/t/state/battery", "50").isSuccess)
        }
    }

    private fun waitUntil(timeoutMillis: Long = 5_000, condition: () -> Boolean) {
        val deadline = System.currentTimeMillis() + timeoutMillis
        while (!condition()) {
            check(System.currentTimeMillis() < deadline) { "timed out" }
            Thread.sleep(20)
        }
    }

    companion object {
        private lateinit var broker: EmbeddedBroker

        @BeforeClass
        @JvmStatic
        fun startBroker() {
            broker = EmbeddedBroker().start()
        }

        @AfterClass
        @JvmStatic
        fun stopBroker() {
            broker.close()
        }
    }
}
