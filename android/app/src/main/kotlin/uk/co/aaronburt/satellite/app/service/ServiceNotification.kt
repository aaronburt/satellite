package uk.co.aaronburt.satellite.app.service

import android.content.Context
import uk.co.aaronburt.satellite.app.R
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport

/** The foreground notification's text for the current transport and connection. */
internal fun Context.serviceNotificationText(state: ConnectionState, transport: Transport): String =
    if (transport == Transport.WEBHOOK) {
        getString(R.string.notification_state_webhook)
    } else {
        when (state) {
            ConnectionState.Disconnected -> getString(R.string.notification_state_disconnected)
            ConnectionState.Connecting -> getString(R.string.notification_state_connecting)
            is ConnectionState.Connected -> getString(R.string.notification_state_connected, state.broker)
            is ConnectionState.Error -> getString(R.string.notification_state_error, state.message)
        }
    }
