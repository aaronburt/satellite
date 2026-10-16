package uk.co.aaronburt.satellite.model

/**
 * Connection settings for the MQTT broker, as entered by the user.
 */
data class BrokerSettings(
    val host: String,
    val port: Int,
    val username: String?,
    val password: String?,
    val useTls: Boolean,
) {
    val serverUri: String
        get() = "${if (useTls) "ssl" else "tcp"}://$host:$port"
}
