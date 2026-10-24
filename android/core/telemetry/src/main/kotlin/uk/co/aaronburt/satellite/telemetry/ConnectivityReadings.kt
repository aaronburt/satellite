package uk.co.aaronburt.satellite.telemetry

/**
 * Decides the published value of the permission-dependent network sensors.
 *
 * These sensors are always published so Home Assistant never keeps a stale
 * retained value: a real reading, or one of the placeholder states below that
 * say *why* there is no reading.
 */
object ConnectivityReadings {

    /** Not on Wi-Fi (mobile data, ethernet, or offline). */
    const val DISCONNECTED = "disconnected"

    /** The user has not granted (or has revoked) the permission this reading needs. */
    const val PERMISSION_DENIED = "permission_denied"

    /** Permission is granted but Android or the OEM hid the value (or there is no SIM). */
    const val UNAVAILABLE = "unavailable"

    private val placeholders = setOf(DISCONNECTED, PERMISSION_DENIED, UNAVAILABLE)

    /** True when [value] is one of the placeholder states rather than a real reading. */
    fun isPlaceholder(value: String?): Boolean = value == null || value in placeholders

    /**
     * The Wi-Fi network name. Checks the active network first, because
     * WifiManager keeps reporting the last SSID for a while after Wi-Fi drops.
     */
    fun wifiSsid(onWifi: Boolean, permitted: Boolean, rawSsid: String?): String = when {
        !onWifi -> DISCONNECTED
        !permitted -> PERMISSION_DENIED
        else -> cleanSsid(rawSsid) ?: UNAVAILABLE
    }

    /** The mobile carrier name. */
    fun carrier(permitted: Boolean, operatorName: String?): String = when {
        !permitted -> PERMISSION_DENIED
        operatorName.isNullOrBlank() -> UNAVAILABLE
        else -> operatorName
    }

    /** Strips Android's quotes and drops the redacted "<unknown ssid>" value. */
    internal fun cleanSsid(raw: String?): String? {
        val ssid = raw?.trim()?.removeSurrounding("\"") ?: return null
        return ssid.takeUnless { it.isBlank() || it == UNKNOWN_SSID }
    }

    private const val UNKNOWN_SSID = "<unknown ssid>"
}
