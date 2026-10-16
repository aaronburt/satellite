package uk.co.aaronburt.satellite.discovery

import org.json.JSONArray
import org.json.JSONObject

/**
 * Builds the Home Assistant MQTT **device discovery** payload (one message per
 * device, entities under `components`).
 *
 * Controllable entities ([EntitySpec.isControl]) are only included when they are
 * in [enabledControls] — they are opt-in. To un-expose one that Home Assistant
 * already knows about, Home Assistant's documented removal procedure is used:
 * first publish the component stub (`{ "platform": … }`), then publish the config
 * without it. [stubControls] drives the first half of that.
 *
 * Reference: https://www.home-assistant.io/integrations/mqtt/#device-discovery-payload
 */
object DiscoveryPayloadBuilder {

    const val ORIGIN_NAME = "Satellite"
    const val SUPPORT_URL = "https://github.com/aaronburt/satellite-android"

    fun build(
        deviceId: String,
        deviceName: String,
        manufacturer: String,
        model: String,
        appVersion: String,
        androidVersion: String,
        specs: List<EntitySpec> = EntityCatalog.all,
        enabledControls: Set<String> = emptySet(),
        stubControls: Set<String> = emptySet(),
    ): String {
        val topics = Topics(deviceId)

        val device = JSONObject().apply {
            put("identifiers", JSONArray().put("satellite_android_$deviceId"))
            put("name", deviceName)
            put("manufacturer", manufacturer)
            put("model", model)
            put("sw_version", appVersion)
            put("hw_version", androidVersion)
            put("configuration_url", SUPPORT_URL)
        }

        val origin = JSONObject().apply {
            put("name", ORIGIN_NAME)
            put("sw_version", appVersion)
            put("support_url", SUPPORT_URL)
        }

        val components = JSONObject().apply {
            specs.forEach { spec ->
                val stub = spec.key in stubControls
                if (!stub && spec.isControl && spec.key !in enabledControls) return@forEach
                put(spec.key, component(spec, deviceId, topics, stub))
            }
        }

        return JSONObject().apply {
            put("~", topics.base)
            put("device", device)
            put("origin", origin)
            put("availability_topic", "~/availability")
            put("qos", 1)
            put("components", components)
        }.toString()
    }

    private fun component(
        spec: EntitySpec,
        deviceId: String,
        topics: Topics,
        stub: Boolean,
    ): JSONObject {
        if (stub) {
            return JSONObject().put("platform", spec.platform)
        }

        return JSONObject().apply {
            put("platform", spec.platform)
            put("unique_id", "${deviceId}_${spec.key}")
            put("name", spec.name)
            put("state_topic", topics.state(spec.key).removePrefix("${topics.base}/").let { "~/$it" })
            spec.command?.let { put("command_topic", "~/cmd/${it.action}") }
            spec.deviceClass?.let { put("device_class", it) }
            spec.unit?.let { put("unit_of_measurement", it) }
            spec.stateClass?.let { put("state_class", it) }
            spec.entityCategory?.let { put("entity_category", it) }
            spec.icon?.let { put("icon", it) }
            spec.min?.let { put("min", it) }
            spec.max?.let { put("max", it) }
            spec.step?.let { put("step", it) }
            spec.options?.let { options ->
                put("options", JSONArray().apply { options.forEach { put(it) } })
            }
        }
    }
}
