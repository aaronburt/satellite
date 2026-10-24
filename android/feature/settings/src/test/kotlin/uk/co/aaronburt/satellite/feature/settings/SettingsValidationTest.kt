package uk.co.aaronburt.satellite.feature.settings

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import uk.co.aaronburt.satellite.model.Transport

class SettingsValidationTest {
    private val valid = SettingsUiState(deviceName = "Kitchen")

    @Test
    fun `ports must be numeric and inside the TCP range`() {
        listOf("", " ", "eighty", "0", "-1", "65536", "999999999999").forEach { port ->
            assertEquals(port, R.string.error_port, valid.copy(port = port).validationErrors()["port"])
        }
        listOf("1", "1883", " 8883 ", "65535").forEach { port ->
            assertTrue(port, valid.copy(port = port).validationErrors().isEmpty())
        }
    }

    @Test
    fun `broker fields accept hosts rather than connection URLs`() {
        listOf(
            "",
            "a b",
            "mqtt://broker.local",
            "broker.local:1883",
            "broker.local/path",
            "user@host"
        ).forEach { host ->
            assertEquals(host, R.string.error_host, valid.copy(host = host).validationErrors()["host"])
        }
        listOf(" broker.local ", "localhost", "10.0.2.2", "[2001:db8::1]").forEach { host ->
            assertTrue(host, valid.copy(host = host).validationErrors().isEmpty())
        }
    }

    @Test
    fun `webhooks require complete supported URLs without embedded secrets`() {
        listOf(
            "",
            "ha.local/hook",
            "ftp://ha.local/hook",
            "https://",
            "https://ha.local:65536/hook",
            "https://user:secret@ha.local/hook",
            "https://ha.local/hook#fragment",
            "https://ha.local/a b",
        ).forEach { url ->
            val errors = valid.copy(transport = Transport.WEBHOOK, webhookUrl = url).validationErrors()
            assertEquals(url, R.string.error_webhook, errors["webhookUrl"])
        }
    }

    @Test
    fun `HTTP follows the application's development host exceptions`() {
        listOf("localhost", "127.0.0.1", "10.0.2.2").forEach { host ->
            val state = valid.copy(transport = Transport.WEBHOOK, webhookUrl = "http://$host:8123/hook")
            assertTrue(host, state.validationErrors().isEmpty())
        }
        listOf("ha.local", "192.168.1.2", "localhost.example.org").forEach { host ->
            val state = valid.copy(transport = Transport.WEBHOOK, webhookUrl = "http://$host/hook")
            assertEquals(host, R.string.error_webhook_https, state.validationErrors()["webhookUrl"])
        }
    }

    @Test
    fun `only the selected transport is validated`() {
        assertTrue(valid.copy(webhookUrl = "invalid hidden URL").validationErrors().isEmpty())
        val webhook = valid.copy(
            transport = Transport.WEBHOOK,
            webhookUrl = " https://ha.local:8123/hook?key=value ",
            host = "",
            port = "invalid",
        )
        assertTrue(webhook.validationErrors().isEmpty())
    }

    @Test
    fun `blank names are actionable errors for both transports`() {
        Transport.entries.forEach { transport ->
            val errors = valid.copy(deviceName = "  ", transport = transport).validationErrors()
            assertEquals(R.string.error_device_name, errors["deviceName"])
            assertFalse(errors.isEmpty())
        }
    }
}
