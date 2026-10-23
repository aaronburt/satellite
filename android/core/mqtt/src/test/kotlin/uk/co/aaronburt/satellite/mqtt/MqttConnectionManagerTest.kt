package uk.co.aaronburt.satellite.mqtt

import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.testing.FakeMqttClient
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.BROKER
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.DEVICE_ID
import kotlinx.coroutines.test.TestScope
import uk.co.aaronburt.satellite.testing.settle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class MqttConnectionManagerTest {

    private val client = FakeMqttClient()
    private val availability = Topics(DEVICE_ID).availability

    private fun TestScope.manager(settings: FakeSettingsRepository) =
        MqttConnectionManager(client, settings, backgroundScope)

    @Test
    fun `connects with the saved broker and registers the offline will on the availability topic`() = runTest {
        manager(FakeSettingsRepository(broker = BROKER)).start()
        settle()

        val connect = client.connects.single()
        assertEquals(BROKER, connect.settings)
        assertEquals("satelliteandroid-$DEVICE_ID", connect.clientId)
        assertEquals("satellite/$DEVICE_ID/availability", connect.willTopic)
        assertEquals(availability, connect.willTopic)
    }

    @Test
    fun `announces online, retained, once connected`() = runTest {
        manager(FakeSettingsRepository(broker = BROKER)).start()
        settle()

        val online = client.publishesTo(availability).single()
        assertEquals("online", online.payload)
        assertTrue("availability must be retained", online.retain)
        assertEquals(1, online.qos)
    }

    @Test
    fun `announces online again after the client reconnects on its own`() = runTest {
        // The broker publishes our retained "offline" will when the link drops.
        // Without a fresh "online" Home Assistant keeps the device unavailable.
        manager(FakeSettingsRepository(broker = BROKER)).start()
        settle()

        client.dropConnection()
        settle()
        client.setConnected()
        settle()

        assertEquals(listOf("online", "online"), client.publishesTo(availability).map { it.payload })
        assertEquals("a reconnect is not a new connect call", 1, client.connects.size)
    }

    @Test
    fun `start is idempotent`() = runTest {
        val manager = manager(FakeSettingsRepository(broker = BROKER))
        manager.start()
        manager.start()
        settle()
        manager.start()
        settle()

        assertEquals(1, client.connects.size)
        assertEquals(1, client.publishesTo(availability).size)
    }

    @Test
    fun `nothing happens until a broker is configured, then it connects`() = runTest {
        val settings = FakeSettingsRepository(broker = null)
        manager(settings).start()
        settle()
        assertTrue(client.connects.isEmpty())

        settings.saveBrokerSettings(BROKER)
        settle()
        assertEquals(BROKER, client.connects.single().settings)
    }

    @Test
    fun `changing the broker settings reconnects with the new ones`() = runTest {
        val settings = FakeSettingsRepository(broker = BROKER)
        manager(settings).start()
        settle()

        val moved = BROKER.copy(host = "new.local", port = 8883, useTls = true)
        settings.saveBrokerSettings(moved)
        settle()

        assertEquals(listOf(BROKER, moved), client.connects.map { it.settings })
    }

    @Test
    fun `clearing the broker settings disconnects`() = runTest {
        val settings = FakeSettingsRepository(broker = BROKER)
        manager(settings).start()
        settle()

        settings.brokerState.value = null
        settle()

        assertEquals("disconnect", client.calls.last())
        assertEquals(ConnectionState.Disconnected, client.connectionState.value)
    }

    @Test
    fun `stop publishes offline before disconnecting and stops announcing`() = runTest {
        val manager = manager(FakeSettingsRepository(broker = BROKER))
        manager.start()
        settle()
        client.calls.clear()

        manager.stop()
        settle()

        assertEquals(listOf("publish $availability offline", "disconnect"), client.calls)
        assertTrue(client.publishesTo(availability).last().retain)

        // A late reconnect event after stop must not mark the device online.
        client.setConnected()
        settle()
        assertEquals("offline", client.lastPayload(availability))
    }

    @Test
    fun `stop without a configured broker only disconnects`() = runTest {
        val manager = manager(FakeSettingsRepository(broker = null))
        manager.start()
        settle()
        client.calls.clear()

        manager.stop()
        settle()

        assertEquals(listOf("disconnect"), client.calls)
    }

    @Test
    fun `a quick stop then start ends up connected`() = runTest {
        // Toggling reporting off and on (or switching transport away and back)
        // calls stop() then start() back to back. The stop's network calls take
        // time; the restart must not be undone by them finishing afterwards.
        client.latencyMillis = 50
        val manager = manager(FakeSettingsRepository(broker = BROKER))
        manager.start()
        settle()

        manager.stop()
        manager.start()
        settle()

        assertTrue(client.connectionState.value is ConnectionState.Connected)
        assertEquals("connect broker.local", client.calls.last { it.startsWith("connect") || it == "disconnect" })
        assertEquals("online", client.lastPayload(availability))
    }

    @Test
    fun `stop twice is harmless`() = runTest {
        val manager = manager(FakeSettingsRepository(broker = BROKER))
        manager.start()
        settle()

        manager.stop()
        manager.stop()
        settle()

        assertEquals(ConnectionState.Disconnected, client.connectionState.value)
        assertEquals(1, client.publishesTo(availability).count { it.payload == "offline" })
    }

    @Test
    fun `a failed connect does not announce online`() = runTest {
        client.connectSucceeds = false
        manager(FakeSettingsRepository(broker = BROKER)).start()
        settle()

        assertTrue(client.connectionState.value is ConnectionState.Error)
        assertTrue(client.publishesTo(availability).isEmpty())
    }

    @Test
    fun `reconnect uses the current settings`() = runTest {
        val manager = manager(FakeSettingsRepository(broker = BROKER))
        manager.reconnect()
        assertTrue("nothing to reconnect to before start", client.connects.isEmpty())

        manager.start()
        settle()
        manager.reconnect()

        assertEquals(listOf(BROKER, BROKER), client.connects.map { it.settings })
    }

    @Test
    fun `exposes the client's connection state`() = runTest {
        val manager = manager(FakeSettingsRepository(broker = BROKER))
        assertEquals(ConnectionState.Disconnected, manager.connectionState.value)
        manager.start()
        settle()
        assertTrue(manager.connectionState.value is ConnectionState.Connected)
    }
}
