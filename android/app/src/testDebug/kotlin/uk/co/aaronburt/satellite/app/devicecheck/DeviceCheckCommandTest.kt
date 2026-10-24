package uk.co.aaronburt.satellite.app.devicecheck

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.WebhookSettings

class DeviceCheckCommandTest {

    @Test
    fun `no extras means no changes`() {
        assertTrue(DeviceCheckCommand.parse(emptyMap()).isEmpty())
    }

    @Test
    fun `full setup is applied in order with reporting last`() {
        val commands = DeviceCheckCommand.parse(
            linkedMapOf(
                "reporting" to "true",
                "transport" to "MQTT",
                "interval" to "30",
                "host" to "10.0.2.2",
                "port" to "1884",
                "username" to "satellite",
                "password" to "secret",
            ),
        )

        assertEquals(
            listOf(
                DeviceCheckCommand.ConfigureBroker(BrokerSettings("10.0.2.2", 1884, "satellite", "secret", false)),
                DeviceCheckCommand.SetInterval(UpdateInterval.THIRTY_SECONDS),
                DeviceCheckCommand.SetTransport(Transport.MQTT),
                DeviceCheckCommand.SetReporting(true),
            ),
            commands,
        )
    }

    @Test
    fun `broker defaults to port 1883 without credentials or TLS`() {
        assertEquals(
            listOf(DeviceCheckCommand.ConfigureBroker(BrokerSettings("broker", 1883, null, null, false))),
            DeviceCheckCommand.parse(mapOf("host" to "broker", "username" to "")),
        )
    }

    @Test
    fun `webhook url and bearer token become webhook settings`() {
        assertEquals(
            listOf(
                DeviceCheckCommand.SetWebhook(WebhookSettings("http://10.0.2.2:8080/hook", "t0ken")),
                DeviceCheckCommand.SetTransport(Transport.WEBHOOK),
            ),
            DeviceCheckCommand.parse(
                mapOf("transport" to "webhook", "webhook_url" to "http://10.0.2.2:8080/hook", "bearer" to "t0ken"),
            ),
        )
    }

    @Test
    fun `control toggles accept on and off spellings`() {
        assertEquals(
            listOf(DeviceCheckCommand.SetControl("mic_muted", false)),
            DeviceCheckCommand.parse(mapOf("control" to "mic_muted", "control_enabled" to "off")),
        )
        assertEquals(
            listOf(DeviceCheckCommand.SetControl("volume_media", true)),
            DeviceCheckCommand.parse(mapOf("control" to "volume_media", "control_enabled" to "1")),
        )
    }

    @Test
    fun `malformed values are rejected instead of ignored`() {
        val bad = listOf(
            mapOf("reporting" to "yes please"),
            mapOf("transport" to "carrier-pigeon"),
            mapOf("interval" to "45"),
            mapOf("host" to "broker", "port" to "abc"),
            mapOf("host" to "broker", "port" to "70000"),
            mapOf("host" to " "),
            mapOf("webhook_url" to "ftp://x"),
            mapOf("control" to "mic_muted"),
        )
        bad.forEach { extras ->
            assertThrows("expected $extras to be rejected", IllegalArgumentException::class.java) {
                DeviceCheckCommand.parse(extras)
            }
        }
    }
}
