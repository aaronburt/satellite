package uk.co.aaronburt.satellite.reporter

import android.util.Log
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.telemetry.AudioControls
import kotlinx.coroutines.flow.first
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Turns parsed [SatelliteCommand]s into device changes.
 *
 * Commands are only accepted for controllable entities the user has explicitly
 * enabled; anything else is ignored. That makes "off by default" a real
 * guarantee rather than just a discovery-level one.
 *
 * A returned `true` means the command was *accepted*, not that the platform
 * applied it — some Android versions and OEM builds restrict audio changes, so
 * the caller republishes the observed state either way.
 */
@Singleton
class CommandRouter @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val audio: AudioControls,
) {

    suspend fun handle(topic: String, payload: String, topics: Topics): Boolean {
        val command = CommandParser.parse(topic, payload, topics) ?: return false
        val enabled = settingsRepository.enabledControls.first()

        return when (command) {
            is SatelliteCommand.SetVolume -> {
                if (EntityCatalog.CONTROL_VOLUME !in enabled) {
                    false
                } else {
                    if (!audio.setMediaVolumePercent(command.percent)) {
                        Log.w(TAG, "Media volume command was refused by the platform")
                    }
                    true
                }
            }

            is SatelliteCommand.SetMicrophoneMuted -> {
                if (EntityCatalog.CONTROL_MIC_MUTE !in enabled) {
                    false
                } else {
                    if (!audio.setMicrophoneMuted(command.muted)) {
                        Log.w(
                            TAG,
                            "Microphone mute was not applied; the platform refused it " +
                                "(observed state differs from the request)",
                        )
                        audio.markMicrophoneMuteUnsupported()
                    }
                    true
                }
            }
        }
    }

    private companion object {
        const val TAG = "CommandRouter"
    }
}
