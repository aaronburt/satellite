package uk.co.aaronburt.satellite.reporter

import uk.co.aaronburt.satellite.datastore.SettingsRepository
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
 */
@Singleton
class CommandRouter @Inject constructor(
    private val settingsRepository: SettingsRepository,
    private val audio: AudioControls,
) {

    /**
     * @return true when the command was accepted (regardless of whether the
     * device change succeeded) so the caller can publish a confirming state.
     */
    suspend fun handle(topic: String, payload: String, topics: Topics): Boolean {
        val command = CommandParser.parse(topic, payload, topics) ?: return false
        val enabled = settingsRepository.enabledControls.first()

        return when (command) {
            is SatelliteCommand.SetVolume ->
                if (VOLUME_KEY in enabled) {
                    audio.setMediaVolumePercent(command.percent)
                    true
                } else {
                    false
                }

            is SatelliteCommand.SetMicrophoneMuted ->
                if (MIC_KEY in enabled) {
                    audio.setMicrophoneMuted(command.muted)
                    true
                } else {
                    false
                }
        }
    }

    private companion object {
        const val VOLUME_KEY = "volume_media"
        const val MIC_KEY = "mic_muted"
    }
}
