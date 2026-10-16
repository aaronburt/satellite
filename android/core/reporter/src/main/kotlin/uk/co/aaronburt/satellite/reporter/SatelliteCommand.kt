package uk.co.aaronburt.satellite.reporter

import uk.co.aaronburt.satellite.discovery.Topics

/** A command Home Assistant can send to this satellite. */
sealed interface SatelliteCommand {
    data class SetVolume(val percent: Int) : SatelliteCommand
    data class SetMicrophoneMuted(val muted: Boolean) : SatelliteCommand
}

/**
 * Parses `<base>/cmd/<action>` messages. Pure and side-effect free so the
 * payload contract can be unit tested.
 */
object CommandParser {

    fun parse(topic: String, payload: String, topics: Topics): SatelliteCommand? {
        if (!topic.startsWith(topics.commandPrefix)) return null

        val action = topic.removePrefix(topics.commandPrefix).trim()
        val value = payload.trim()

        return when (action) {
            "volume" -> value
                .toIntOrNull()
                ?.takeIf { it in VOLUME_RANGE }
                ?.let { SatelliteCommand.SetVolume(it) }

            "mute" -> when (value.uppercase()) {
                "ON" -> SatelliteCommand.SetMicrophoneMuted(true)
                "OFF" -> SatelliteCommand.SetMicrophoneMuted(false)
                else -> null
            }

            else -> null
        }
    }

    private val VOLUME_RANGE = 0..100
}
