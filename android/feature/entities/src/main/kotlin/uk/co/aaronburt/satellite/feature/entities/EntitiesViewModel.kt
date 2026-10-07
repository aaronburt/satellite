package uk.co.aaronburt.satellite.feature.entities

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.EntitySpec
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.util.Locale
import javax.inject.Inject

/**
 * Renders the entity catalog with live values. Sensor values mirror exactly what
 * is published to Home Assistant; controls show their current state and whether
 * they are exposed, which is off by default.
 */
@HiltViewModel
class EntitiesViewModel @Inject constructor(
    private val telemetry: DeviceStateReader,
    private val audio: AudioControls,
    private val settingsRepository: SettingsRepository,
) : ViewModel() {

    private data class Readings(
        val states: Map<String, String>,
        val volumePercent: Int?,
        val microphoneMuted: Boolean?,
    )

    private val readings = flow {
        while (true) {
            emit(
                Readings(
                    states = runCatching { telemetry.read() }.getOrDefault(emptyMap()),
                    volumePercent = audio.mediaVolumePercent(),
                    microphoneMuted = audio.isMicrophoneMuted(),
                ),
            )
            delay(REFRESH_MILLIS)
        }
    }

    val uiState: StateFlow<EntitiesUiState> = combine(
        readings,
        settingsRepository.enabledControls,
        audio.microphoneMuteSupported,
    ) { snapshot, enabled, microphoneMuteSupported ->
        EntitiesUiState(
            sensors = EntityCatalog.entities.mapNotNull { it.toRow(snapshot.states) },
            controls = EntityCatalog.controlEntities.map { spec ->
                ControlRow(
                    key = spec.key,
                    label = spec.name,
                    value = controlValue(spec.key, snapshot),
                    enabled = spec.key in enabled,
                    available = spec.key != EntityCatalog.CONTROL_MIC_MUTE ||
                        microphoneMuteSupported != false,
                )
            },
        )
    }.stateIn(
        scope = viewModelScope,
        started = SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS),
        initialValue = EntitiesUiState(),
    )

    fun onControlToggled(key: String, enabled: Boolean) {
        viewModelScope.launch {
            // Turning a control on is the moment to prove the device can honour
            // it. If it can't, leave it off and let the row render as unavailable.
            if (enabled && key == EntityCatalog.CONTROL_MIC_MUTE) {
                val supported = withContext(Dispatchers.IO) { audio.probeMicrophoneMute() }
                if (!supported) return@launch
            }
            settingsRepository.setControlEnabled(key, enabled)
        }
    }

    private fun controlValue(key: String, snapshot: Readings): String = when (key) {
        "volume_media" -> snapshot.volumePercent?.let { "$it%" } ?: "\u2014"
        "mic_muted" -> when (snapshot.microphoneMuted) {
            true -> "Muted"
            false -> "Live"
            null -> "\u2014"
        }
        else -> "\u2014"
    }

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
        return String.format(Locale.UK, "%.1f GB", bytes / 1_000_000_000.0)
    }

    private companion object {
        const val STOP_TIMEOUT_MILLIS = 5_000L
        const val REFRESH_MILLIS = 15_000L
    }
}
