package dev.satelliteandroid.discovery

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test

class WebhookPayloadBuilderTest {

    private val json = JSONObject(
        WebhookPayloadBuilder.build(
            deviceId = "abcd1234",
            deviceName = "Satellite-1A2B",
            manufacturer = "Google",
            model = "Pixel 8",
            appVersion = "0.1.0",
            androidVersion = "16",
            states = mapOf("battery" to "87", "charging" to "ON"),
            timestampEpochMillis = 1_700_000_000_000L,
        ),
    )

    @Test
    fun `includes device metadata`() {
        assertEquals("abcd1234", json.getString("device_id"))
        assertEquals("Satellite-1A2B", json.getString("device_name"))
        assertEquals("Google", json.getString("manufacturer"))
        assertEquals("Pixel 8", json.getString("model"))
        assertEquals("0.1.0", json.getString("app_version"))
        assertEquals("16", json.getString("android_version"))
    }

    @Test
    fun `nests the device state under state`() {
        val state = json.getJSONObject("state")
        assertEquals("87", state.getString("battery"))
        assertEquals("ON", state.getString("charging"))
    }

    @Test
    fun `carries a timestamp`() {
        assertEquals(1_700_000_000_000L, json.getLong("timestamp"))
    }
}
