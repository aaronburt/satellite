package uk.co.aaronburt.satellite.feature.entities

import java.util.Locale
import uk.co.aaronburt.satellite.discovery.EntityCatalog

/** Turns raw published values into what the Entities screen shows. */
internal object EntityFormatter {

    private const val NO_VALUE = "\u2014"

    fun format(key: String, raw: String): String = when (key) {
        "charging", "power_save", "doze", "interactive" ->
            if (raw == "ON") "On" else "Off"
        "battery" -> "$raw%"
        "battery_temperature" -> "$raw \u00b0C"
        "uptime" -> formatDuration(raw.toLongOrNull())
        "storage_free" -> formatBytes(raw.toLongOrNull())
        else -> raw.replaceFirstChar { it.uppercase() }
    }

    fun controlValue(key: String, volumePercent: Int?, microphoneMuted: Boolean?): String = when (key) {
        EntityCatalog.CONTROL_VOLUME -> volumePercent?.let { "$it%" } ?: NO_VALUE
        EntityCatalog.CONTROL_MIC_MUTE -> when (microphoneMuted) {
            true -> "Muted"
            false -> "Live"
            null -> NO_VALUE
        }
        else -> NO_VALUE
    }

    fun formatDuration(seconds: Long?): String {
        if (seconds == null) return "unknown"
        val hours = seconds / 3_600
        val minutes = (seconds % 3_600) / 60
        return when {
            hours > 0 -> "${hours}h ${minutes}m"
            minutes > 0 -> "${minutes}m"
            else -> "${seconds}s"
        }
    }

    fun formatBytes(bytes: Long?): String {
        if (bytes == null) return "unknown"
        return String.format(Locale.UK, "%.1f GB", bytes / 1_000_000_000.0)
    }
}
