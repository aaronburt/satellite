package uk.co.aaronburt.satellite.app.service

import android.app.Application
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport

// A plain Application keeps Hilt's generated app class out of these tests.
@RunWith(RobolectricTestRunner::class)
@Config(application = Application::class)
class ServiceNotificationTest {

    private val context = RuntimeEnvironment.getApplication()

    @Test
    fun `describes the MQTT connection`() {
        assertEquals("Not connected", context.serviceNotificationText(ConnectionState.Disconnected, Transport.MQTT))
        assertEquals("Connecting…", context.serviceNotificationText(ConnectionState.Connecting, Transport.MQTT))
        assertEquals(
            "Connected to ssl://broker.local:8883",
            context.serviceNotificationText(ConnectionState.Connected("ssl://broker.local:8883", 0), Transport.MQTT),
        )
        assertEquals(
            "Error: Connection refused",
            context.serviceNotificationText(ConnectionState.Error("Connection refused"), Transport.MQTT),
        )
    }

    @Test
    fun `webhook mode ignores the MQTT connection`() {
        for (state in listOf(ConnectionState.Disconnected, ConnectionState.Error("x"))) {
            assertEquals("Webhook mode", context.serviceNotificationText(state, Transport.WEBHOOK))
        }
    }
}
