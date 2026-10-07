package uk.co.aaronburt.satellite.discovery

/**
 * Topic layout for one satellite device.
 *
 * Base:       `satellite/<deviceId>`
 * Availability: `<base>/availability`        (online/offline, retained, LWT)
 * State:        `<base>/state/<key>`         (retained)
 * Commands:     `<base>/cmd/<action>`        (never retained)
 * Discovery:    `homeassistant/device/satellite_<deviceId>/config` (retained)
 * HA birth:     `homeassistant/status`
 */
class Topics(private val deviceId: String) {

    val base: String = "satellite/$deviceId"

    val availability: String = "$base/availability"

    val discovery: String = "$DISCOVERY_PREFIX/device/satellite_$deviceId/config"

    fun state(key: String): String = "$base/state/$key"

    val commandPrefix: String = "$base/$COMMAND_SEGMENT/"

    fun command(action: String): String = commandPrefix + action

    /** Matches every command topic for this device. */
    val commandWildcard: String = "${commandPrefix}#"

    companion object {
        const val DISCOVERY_PREFIX = "homeassistant"
        const val BIRTH_TOPIC = "homeassistant/status"
        const val COMMAND_SEGMENT = "cmd"
    }
}
