package uk.co.aaronburt.satellite.app.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import dagger.hilt.android.AndroidEntryPoint
import uk.co.aaronburt.satellite.app.MainActivity
import uk.co.aaronburt.satellite.app.R
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.mqtt.MqttConnectionManager
import uk.co.aaronburt.satellite.reporter.ReporterCoordinator
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * Keeps the MQTT connection / webhook reporter alive while the app is in the
 * background. Runs as a foreground service of type `specialUse` (a persistent
 * connection to the user's own broker cannot be expressed with the other
 * foreground types).
 */
@AndroidEntryPoint
class SatelliteService : Service() {

    @Inject
    lateinit var connectionManager: MqttConnectionManager

    @Inject
    lateinit var coordinator: ReporterCoordinator

    @Inject
    lateinit var settingsRepository: SettingsRepository

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    @Volatile
    private var transport: Transport = Transport.MQTT

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        startForeground(connectionManager.connectionState.value, transport)

        serviceScope.launch {
            // Belt and braces: never run while the user has paused reporting.
            if (!settingsRepository.reportingEnabled.first()) {
                stopSelf()
                return@launch
            }

            coordinator.start()

            combine(connectionManager.connectionState, settingsRepository.transport) { state, current ->
                state to current
            }.collect { (state, current) ->
                transport = current
                // Re-post through startForeground so the notification stays owned
                // by the service (and is removed with it when we stop).
                startForeground(state, current)
            }
        }

        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        coordinator.stop()
        // Belt and braces: make sure nothing is left in the shade when we go away.
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        notificationManager().cancel(NOTIFICATION_ID)
        serviceScope.cancel()
        super.onDestroy()
    }

    private fun startForeground(state: ConnectionState, current: Transport) {
        val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE
        } else {
            0
        }
        ServiceCompat.startForeground(
            this,
            NOTIFICATION_ID,
            buildNotification(state, current),
            type,
        )
    }

    private fun buildNotification(state: ConnectionState, transport: Transport): Notification {
        val openApp = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )

        val text = if (transport == Transport.WEBHOOK) {
            getString(R.string.notification_state_webhook)
        } else {
            state.notificationText()
        }

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_satellite)
            .setContentTitle(getString(R.string.notification_title))
            .setContentText(text)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setContentIntent(openApp)
            .build()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.notification_channel_name),
            NotificationManager.IMPORTANCE_LOW,
        )
        notificationManager().createNotificationChannel(channel)
    }

    private fun notificationManager(): NotificationManager =
        getSystemService(NotificationManager::class.java)

    private fun ConnectionState.notificationText(): String = when (this) {
        ConnectionState.Disconnected -> getString(R.string.notification_state_disconnected)
        ConnectionState.Connecting -> getString(R.string.notification_state_connecting)
        is ConnectionState.Connected -> getString(R.string.notification_state_connected, broker)
        is ConnectionState.Error -> getString(R.string.notification_state_error, message)
    }

    companion object {
        const val CHANNEL_ID = "mqtt_connection"
        const val NOTIFICATION_ID = 1001
    }
}
