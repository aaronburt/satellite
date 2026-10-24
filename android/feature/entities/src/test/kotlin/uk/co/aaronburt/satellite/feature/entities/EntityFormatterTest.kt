package uk.co.aaronburt.satellite.feature.entities

import org.junit.Assert.assertEquals
import org.junit.Test

class EntityFormatterTest {

    @Test
    fun `on-off sensors read as On or Off`() {
        for (key in listOf("charging", "power_save", "doze", "interactive")) {
            assertEquals(key, "On", EntityFormatter.format(key, "ON"))
            assertEquals(key, "Off", EntityFormatter.format(key, "OFF"))
        }
    }

    @Test
    fun `units are added where the raw value has none`() {
        assertEquals("64%", EntityFormatter.format("battery", "64"))
        assertEquals("31.5 °C", EntityFormatter.format("battery_temperature", "31.5"))
    }

    @Test
    fun `uptime is shown in the largest sensible unit`() {
        assertEquals("45s", EntityFormatter.format("uptime", "45"))
        assertEquals("1m", EntityFormatter.format("uptime", "60"))
        assertEquals("59m", EntityFormatter.format("uptime", "3599"))
        assertEquals("1h 0m", EntityFormatter.format("uptime", "3600"))
        assertEquals("49h 1m", EntityFormatter.format("uptime", "176460"))
        assertEquals("unknown", EntityFormatter.format("uptime", "soon"))
    }

    @Test
    fun `free storage is shown in decimal gigabytes`() {
        assertEquals("12.3 GB", EntityFormatter.format("storage_free", "12345678901"))
        assertEquals("0.0 GB", EntityFormatter.format("storage_free", "0"))
        assertEquals("unknown", EntityFormatter.format("storage_free", ""))
    }

    @Test
    fun `everything else is capitalised`() {
        assertEquals("Wifi", EntityFormatter.format("network_transport", "wifi"))
        assertEquals("Vibrate", EntityFormatter.format("ringer_mode", "vibrate"))
        assertEquals("HomeNet", EntityFormatter.format("wifi_ssid", "HomeNet"))
        assertEquals("", EntityFormatter.format("carrier", ""))
    }

    @Test
    fun `control values`() {
        assertEquals("40%", EntityFormatter.controlValue("volume_media", 40, null))
        assertEquals("—", EntityFormatter.controlValue("volume_media", null, true))
        assertEquals("Muted", EntityFormatter.controlValue("mic_muted", null, true))
        assertEquals("Live", EntityFormatter.controlValue("mic_muted", 10, false))
        assertEquals("—", EntityFormatter.controlValue("mic_muted", 10, null))
        assertEquals("—", EntityFormatter.controlValue("unknown", 10, true))
    }

    @Test
    fun `network placeholders read as plain words`() {
        assertEquals("Not connected", EntityFormatter.format("wifi_ssid", "disconnected"))
        assertEquals("Permission needed", EntityFormatter.format("wifi_ssid", "permission_denied"))
        assertEquals("Unavailable", EntityFormatter.format("carrier", "unavailable"))
        assertEquals("Permission needed", EntityFormatter.format("carrier", "permission_denied"))
        assertEquals("eduroam", EntityFormatter.format("wifi_ssid", "eduroam"))
    }
}
