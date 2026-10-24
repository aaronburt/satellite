package uk.co.aaronburt.satellite.feature.entities

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import uk.co.aaronburt.satellite.common.coroutines.DispatchersProvider
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.EntitySpec
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader

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
    private val dispatchers: DispatchersProvider
) : ViewModel() {

    private data class Readings(val states: Map<String, String>, val volumePercent: Int?, val microphoneMuted: Boolean?)

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
                    value = EntityFormatter.controlValue(
                        spec.key,
                        snapshot.volumePercent,
                        snapshot.microphoneMuted,
                    ),
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
            if (enabled && key == EntityCatalog.CONTROL_MIC_MUTE) {
                val supported = withContext(dispatchers.io) { audio.probeMicrophoneMute() }
                if (!supported) return@launch
            }
            settingsRepository.setControlEnabled(key, enabled)
        }
    }

    private fun EntitySpec.toRow(values: Map<String, String>): EntityRow? {
        val raw = values[key] ?: return null
        return EntityRow(key = key, label = name, value = EntityFormatter.format(key, raw))
    }

    private companion object {
        const val STOP_TIMEOUT_MILLIS = 5_000L
        const val REFRESH_MILLIS = 15_000L
    }
}
