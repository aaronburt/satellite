package uk.co.aaronburt.satellite.feature.entities

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.EntitySpec
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import java.util.Locale
import javax.inject.Inject

/**
 * Renders the entity catalog with live values. Sensor values mirror exactly what
 * is published to Home Assistant; the controls are read-only for now.
 */
@HiltViewModel
class EntitiesViewModel @Inject constructor(
    private val telemetry: DeviceStateReader,
) : ViewModel() {

    private val snapshot = flow {
        while (true) {
            emit(runCatching { telemetry.read() }.getOrDefault(emptyMap()))
            delay(REFRESH_MILLIS)
        }
    }

    val uiState: StateFlow<EntitiesUiState> = snapshot
        .map { values ->
            EntitiesUiState(
                sensors = EntityCatalog.entities.mapNotNull { spec -> spec.toRow(values) },
                mediaVolumePercent = telemetry.mediaVolumePercent(),
                microphoneMuted = telemetry.isMicrophoneMuted(),
            )
        }
        .stateIn(
            scope = viewModelScope,
            started = SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS),
            initialValue = EntitiesUiState(),
        )

    private fun EntitySpec.toRow(values: Map<String, String>): EntityRow? {
        val raw = values[key] ?: return null
        return EntityRow(key = key, label = name, value = format(key, raw))
    }

    private fun format(key: String, raw: String): String = when (key) {
        "charging", "power_save", "doze", "interactive" ->
            if (raw == "ON") "On" else "Off"
        "battery" -> "$raw%"
        "battery_temperature" -> "$raw \u00b0C"
        "uptime" -> formatDuration(raw.toLongOrNull())
        "storage_free" -> formatBytes(raw.toLongOrNull())
        else -> raw.replaceFirstChar { it.uppercase() }
    }

    private fun formatDuration(seconds: Long?): String {
        if (seconds == null) return "unknown"
        val hours = seconds / 3_600
        val minutes = (seconds % 3_600) / 60
        return when {
            hours > 0 -> "${hours}h ${minutes}m"
            minutes > 0 -> "${minutes}m"
            else -> "${seconds}s"
        }
    }

    private fun formatBytes(bytes: Long?): String {
        if (bytes == null) return "unknown"
        val gigabytes = bytes / 1_000_000_000.0
        return String.format(Locale.UK, "%.1f GB", gigabytes)
    }

    private companion object {
        const val STOP_TIMEOUT_MILLIS = 5_000L
        const val REFRESH_MILLIS = 15_000L
    }
}
