package uk.co.aaronburt.satellite.feature.settings

import java.net.URI
import uk.co.aaronburt.satellite.model.Transport

private val validPorts = 1..65535
private val developmentHosts = setOf("localhost", "127.0.0.1", "10.0.2.2")

/** Validate the visible connection before any configuration is written. */
internal fun SettingsUiState.validationErrors(): Map<String, Int> = buildMap {
    if (deviceName.isBlank()) put("deviceName", R.string.error_device_name)
    if (transport == Transport.MQTT) {
        if (!validBrokerHost(host.trim())) put("host", R.string.error_host)
        if (port.trim().toIntOrNull() !in validPorts) put("port", R.string.error_port)
    } else {
        webhookError(webhookUrl.trim())?.let { put("webhookUrl", it) }
    }
}

private fun validBrokerHost(host: String): Boolean {
    val uri = parseUri("mqtt://$host") ?: return false
    val hasOnlyHost = uri.host == host && uri.rawUserInfo == null && uri.port == -1
    val hasNoSuffix = uri.rawPath.isNullOrEmpty() && uri.rawQuery == null && uri.rawFragment == null
    return hasOnlyHost && hasNoSuffix
}

private fun webhookError(url: String): Int? {
    val uri = parseUri(url) ?: return R.string.error_webhook
    val validEndpoint = !uri.host.isNullOrBlank() && uri.scheme?.lowercase() in setOf("http", "https")
    val validAuthority = uri.rawUserInfo == null && (uri.port == -1 || uri.port in validPorts)
    if (!validEndpoint || !validAuthority || uri.rawFragment != null) return R.string.error_webhook
    if (uri.scheme.equals("http", ignoreCase = true) && uri.host.lowercase() !in developmentHosts) {
        return R.string.error_webhook_https
    }
    return null
}

private fun parseUri(value: String): URI? = runCatching { URI(value) }.getOrNull()
