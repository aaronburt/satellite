package uk.co.aaronburt.satellite.app.devicecheck

import uk.co.aaronburt.satellite.model.Transport

/**
 * State returned to the device-check script as `key=value;key=value`, which a
 * shell can split without a JSON parser. Unknown audio values are `unknown`.
 */
data class DeviceCheckReport(
    val deviceId: String,
    val reporting: Boolean,
    val transport: Transport,
    val enabledControls: Set<String>,
    val micMuted: Boolean?,
    val volumePercent: Int?
) {
    fun format(): String = listOf(
        "device_id" to deviceId,
        "reporting" to reporting.toString(),
        "transport" to transport.name.lowercase(),
        "controls" to enabledControls.sorted().joinToString(","),
        "mic_muted" to (micMuted?.toString() ?: "unknown"),
        "volume" to (volumePercent?.toString() ?: "unknown"),
    ).joinToString(";") { (key, value) -> "$key=$value" }
}
