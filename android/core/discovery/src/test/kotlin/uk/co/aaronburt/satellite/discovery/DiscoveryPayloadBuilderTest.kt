package uk.co.aaronburt.satellite.discovery

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
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
    fun `writes topics out in full, without the root abbreviation Home Assistant rejects`() {
        assertFalse(json.has("~"))
        assertEquals("satellite/abcd1234/availability", json.getString("availability_topic"))
        assertFalse("no abbreviated topics", payload.contains("\"~/"))
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
            assertEquals("satellite/abcd1234/state/${spec.key}", component.getString("state_topic"))
        }
    }

    @Test
    fun `enum entities carry their options`() {
        val chargerType = json.getJSONObject("components").getJSONObject("charger_type")
        assertEquals("enum", chargerType.getString("device_class"))
        assertTrue(chargerType.getJSONArray("options").length() > 0)
    }

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
        assertEquals("satellite/abcd1234/cmd/volume", volume.getString("command_topic"))
        assertEquals("satellite/abcd1234/state/volume_media", volume.getString("state_topic"))
        assertEquals(0, volume.getInt("min"))
        assertEquals(100, volume.getInt("max"))

        val mic = components.getJSONObject("mic_muted")
        assertEquals("switch", mic.getString("platform"))
        assertEquals("satellite/abcd1234/cmd/mute", mic.getString("command_topic"))
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

    private fun build(enabled: Set<String> = emptySet(), stubs: Set<String> = emptySet()) = JSONObject(
        DiscoveryPayloadBuilder.build(
            deviceId = "abcd1234",
            deviceName = "Satellite-1A2B",
            manufacturer = "Google",
            model = "Pixel",
            appVersion = "1.0",
            androidVersion = "15",
            enabledControls = enabled,
            stubControls = stubs,
        ),
    ).getJSONObject("components")

    @Test
    fun `refresh button has a command topic and no state topic`() {
        val button = build(setOf(EntityCatalog.CONTROL_REFRESH)).getJSONObject(EntityCatalog.CONTROL_REFRESH)

        assertEquals("button", button.getString("platform"))
        assertEquals("satellite/abcd1234/cmd/refresh", button.getString("command_topic"))
        assertFalse("Home Assistant rejects a state topic on a button", button.has("state_topic"))
    }

    @Test
    fun `refresh result sensor appears and disappears with the refresh button`() {
        val enabled = build(setOf(EntityCatalog.CONTROL_REFRESH))
        val result = enabled.getJSONObject(EntityCatalog.REFRESH_RESULT)
        assertEquals("satellite/abcd1234/state/refresh_result", result.getString("state_topic"))
        assertEquals(EntityCatalog.refreshOutcomes.size, result.getJSONArray("options").length())

        val disabled = build()
        assertFalse(disabled.has(EntityCatalog.CONTROL_REFRESH))
        assertFalse(disabled.has(EntityCatalog.REFRESH_RESULT))
    }

    @Test
    fun `refresh entities can be removed with platform-only stubs`() {
        val components = build(
            stubs = setOf(EntityCatalog.CONTROL_REFRESH, EntityCatalog.REFRESH_RESULT),
        )

        assertEquals(1, components.getJSONObject(EntityCatalog.CONTROL_REFRESH).length())
        assertEquals(1, components.getJSONObject(EntityCatalog.REFRESH_RESULT).length())
    }
}
