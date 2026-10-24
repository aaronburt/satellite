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

    private companion object {
        const val DEFAULT_PORT = 1883
        const val ALT_PORT = 1884
    }

    @Test
    fun noExtrasMeansNoChanges() {
        assertTrue(DeviceCheckCommand.parse(emptyMap()).isEmpty())
    }

    @Test
    fun fullSetupIsAppliedInOrderWithReportingLast() {
        val commands = DeviceCheckCommand.parse(
            linkedMapOf(
                "reporting" to "true",
                "transport" to "MQTT",
                "interval" to "30",
                "host" to "10.0.2.2",
                "port" to ALT_PORT.toString(),
                "username" to "satellite",
                "password" to "secret",
            ),
        )

        assertEquals(
            listOf(
                DeviceCheckCommand.ConfigureBroker(BrokerSettings("10.0.2.2", ALT_PORT, "satellite", "secret", false)),
                DeviceCheckCommand.SetInterval(UpdateInterval.THIRTY_SECONDS),
                DeviceCheckCommand.SetTransport(Transport.MQTT),
                DeviceCheckCommand.SetReporting(true),
            ),
            commands,
        )
    }

    @Test
    fun brokerDefaultsToPort1883WithoutCredentialsOrTLS() {
        assertEquals(
            listOf(DeviceCheckCommand.ConfigureBroker(BrokerSettings("broker", DEFAULT_PORT, null, null, false))),
            DeviceCheckCommand.parse(mapOf("host" to "broker", "username" to "")),
        )
    }

    @Test
    fun webhookUrlAndBearerTokenBecomeWebhookSettings() {
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
    fun controlTogglesAcceptOnAndOffSpellings() {
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
    fun malformedValuesAreRejectedInsteadOfIgnored() {
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
