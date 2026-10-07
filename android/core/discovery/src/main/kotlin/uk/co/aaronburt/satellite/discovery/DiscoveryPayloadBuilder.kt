package uk.co.aaronburt.satellite.discovery

import org.json.JSONArray
import org.json.JSONObject

/**
 * Builds the Home Assistant MQTT **device discovery** payload (one message per
 * device, entities under `components`).
 *
 * Reference: https://www.home-assistant.io/integrations/mqtt/#device-discovery-payload
 */
object DiscoveryPayloadBuilder {

    const val ORIGIN_NAME = "Satellite"
    const val SUPPORT_URL = "https://github.com/"

    fun build(
        deviceId: String,
        deviceName: String,
        manufacturer: String,
        model: String,
        appVersion: String,
        androidVersion: String,
        specs: List<EntitySpec> = EntityCatalog.entities,
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
                put(spec.key, component(spec, deviceId, topics))
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

    private fun component(spec: EntitySpec, deviceId: String, topics: Topics): JSONObject =
        JSONObject().apply {
            put("platform", spec.platform)
            put("unique_id", "${deviceId}_${spec.key}")
            put("name", spec.name)
            put("state_topic", topics.state(spec.key).removePrefix("${topics.base}/").let { "~/$it" })
            spec.deviceClass?.let { put("device_class", it) }
            spec.unit?.let { put("unit_of_measurement", it) }
            spec.stateClass?.let { put("state_class", it) }
            spec.entityCategory?.let { put("entity_category", it) }
            spec.icon?.let { put("icon", it) }
            spec.options?.let { options ->
                put("options", JSONArray().apply { options.forEach { put(it) } })
            }
        }
}
