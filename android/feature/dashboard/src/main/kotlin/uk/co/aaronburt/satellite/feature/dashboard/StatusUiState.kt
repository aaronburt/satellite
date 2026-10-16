package uk.co.aaronburt.satellite.feature.dashboard

import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport

/** One at-a-glance value shown on the Status screen. */
data class StatusHighlight(
    val key: String,
    val label: String,
    val value: String,
)

data class StatusUiState(
    val deviceName: String = "",
    val transport: Transport = Transport.MQTT,
    val connectionState: ConnectionState = ConnectionState.Disconnected,
    val reportingEnabled: Boolean = true,
    val lastPublishedAtMillis: Long? = null,
    val entityCount: Int = 0,
    val configured: Boolean = true,
    val highlights: List<StatusHighlight> = emptyList(),
)
