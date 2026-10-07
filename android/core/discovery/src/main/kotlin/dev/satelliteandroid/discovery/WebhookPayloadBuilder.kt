package dev.satelliteandroid.discovery

import org.json.JSONObject

/**
 * JSON snapshot posted to a webhook endpoint. Designed to be consumed by a Home
 * Assistant webhook trigger or any custom endpoint:
 *
 * ```yaml
 * trigger:
 *   - platform: webhook
 *     webhook_id: <id>
 * # use trigger.json.state.battery etc.
 * ```
 */
object WebhookPayloadBuilder {

    fun build(
        deviceId: String,
        deviceName: String,
        manufacturer: String,
        model: String,
        appVersion: String,
        androidVersion: String,
        states: Map<String, String>,
        timestampEpochMillis: Long,
    ): String {
        val state = JSONObject().apply {
            states.forEach { (key, value) -> put(key, value) }
        }

        return JSONObject().apply {
            put("device_id", deviceId)
            put("device_name", deviceName)
            put("manufacturer", manufacturer)
            put("model", model)
            put("app_version", appVersion)
            put("android_version", androidVersion)
            put("timestamp", timestampEpochMillis)
            put("state", state)
        }.toString()
    }
}
