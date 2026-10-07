package uk.co.aaronburt.satellite.model

/**
 * State of the MQTT connection as exposed to the UI and to publishers.
 */
sealed interface ConnectionState {

    data object Disconnected : ConnectionState

    data object Connecting : ConnectionState

    data class Connected(
        val broker: String,
        val connectedSinceEpochMillis: Long,
    ) : ConnectionState

    data class Error(
        val message: String,
    ) : ConnectionState
}
