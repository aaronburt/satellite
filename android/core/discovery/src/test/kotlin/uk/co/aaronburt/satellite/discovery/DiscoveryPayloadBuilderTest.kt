package uk.co.aaronburt.satellite.discovery

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class DiscoveryPayloadBuilderTest {

    private val payload = DiscoveryPayloadBuilder.build(
        deviceId = "abcd1234",
        deviceName = "Satellite-1A2B",
        manufacturer = "Google",
        model = "Pixel 8",
        appVersion = "0.1.0",
        androidVersion = "16",
    )

    private val json = JSONObject(payload)

    @Test
    fun `uses the satellite base topic`() {
        assertEquals("satellite/abcd1234", json.getString("~"))
        assertEquals("~/availability", json.getString("availability_topic"))
    }

    @Test
    fun `includes required device and origin blocks`() {
        val device = json.getJSONObject("device")
        assertEquals("Satellite-1A2B", device.getString("name"))
        assertEquals("Google", device.getString("manufacturer"))
        assertEquals("Pixel 8", device.getString("model"))
        assertEquals("satellite_android_abcd1234", device.getJSONArray("identifiers").getString(0))

        assertEquals("Satellite", json.getJSONObject("origin").getString("name"))
    }

    @Test
    fun `emits every catalog entity as a component`() {
        val components = json.getJSONObject("components")
        assertEquals(EntityCatalog.entities.size, components.length())

        EntityCatalog.entities.forEach { spec ->
            val component = components.getJSONObject(spec.key)
            assertEquals(spec.platform, component.getString("platform"))
            assertEquals("abcd1234_${spec.key}", component.getString("unique_id"))
            assertEquals("~/state/${spec.key}", component.getString("state_topic"))
        }
    }

    @Test
    fun `enum entities carry their options`() {
        val chargerType = json.getJSONObject("components").getJSONObject("charger_type")
        assertEquals("enum", chargerType.getString("device_class"))
        assertTrue(chargerType.getJSONArray("options").length() > 0)
    }

    // --- Controllable entities are opt-in ------------------------------------

    @Test
    fun `controls are not exposed by default`() {
        val components = json.getJSONObject("components")
        EntityCatalog.controlEntities.forEach { spec ->
            assertTrue(
                "expected ${spec.key} to be hidden by default",
                !components.has(spec.key),
            )
        }
    }

    @Test
    fun `enabled control carries its command topic and range`() {
        val payload = DiscoveryPayloadBuilder.build(
            deviceId = "abcd1234",
            deviceName = "Satellite-1A2B",
            manufacturer = "Google",
            model = "Pixel 8",
            appVersion = "0.1.0",
            androidVersion = "16",
            enabledControls = setOf("volume_media", "mic_muted"),
        )
        val components = JSONObject(payload).getJSONObject("components")

        val volume = components.getJSONObject("volume_media")
        assertEquals("number", volume.getString("platform"))
        assertEquals("~/cmd/volume", volume.getString("command_topic"))
        assertEquals("~/state/volume_media", volume.getString("state_topic"))
        assertEquals(0, volume.getInt("min"))
        assertEquals(100, volume.getInt("max"))

        val mic = components.getJSONObject("mic_muted")
        assertEquals("switch", mic.getString("platform"))
        assertEquals("~/cmd/mute", mic.getString("command_topic"))
    }

    @Test
    fun `disabled control can be removed with a platform-only stub`() {
        val payload = DiscoveryPayloadBuilder.build(
            deviceId = "abcd1234",
            deviceName = "Satellite-1A2B",
            manufacturer = "Google",
            model = "Pixel 8",
            appVersion = "0.1.0",
            androidVersion = "16",
            stubControls = setOf("mic_muted"),
        )
        val components = JSONObject(payload).getJSONObject("components")

        val stub = components.getJSONObject("mic_muted")
        assertEquals("switch", stub.getString("platform"))
        assertEquals(1, stub.length())
    }
}
