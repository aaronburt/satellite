package uk.co.aaronburt.satellite.reporter

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.ConnectivityManager
import android.os.Build
import android.os.PowerManager
import androidx.core.content.ContextCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Watches system broadcasts that indicate device state changed and invokes
 * [onChanged] after a short debounce. Shared by the MQTT and webhook reporters.
 */
internal class StateChangeWatcher(
    private val context: Context,
    private val scope: CoroutineScope,
    private val isActive: () -> Boolean,
    private val onChanged: suspend () -> Unit,
) {

    private var receiver: BroadcastReceiver? = null
    private var debounceJob: Job? = null

    fun register() {
        if (receiver != null) return

        val newReceiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context?, intent: Intent?) {
                if (!isActive()) return
                debounceJob?.cancel()
                debounceJob = scope.launch {
                    delay(DEBOUNCE_MILLIS)
                    onChanged()
                }
            }
        }

        val filter = IntentFilter().apply {
            addAction(Intent.ACTION_BATTERY_CHANGED)
            addAction(Intent.ACTION_POWER_CONNECTED)
            addAction(Intent.ACTION_POWER_DISCONNECTED)
            addAction(Intent.ACTION_SCREEN_ON)
            addAction(Intent.ACTION_SCREEN_OFF)
            addAction(Intent.ACTION_USER_PRESENT)
            addAction(PowerManager.ACTION_POWER_SAVE_MODE_CHANGED)
            addAction(ConnectivityManager.CONNECTIVITY_ACTION)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                addAction(PowerManager.ACTION_DEVICE_IDLE_MODE_CHANGED)
            }
        }

        ContextCompat.registerReceiver(
            context,
            newReceiver,
            filter,
            ContextCompat.RECEIVER_NOT_EXPORTED,
        )
        receiver = newReceiver
    }

    fun unregister() {
        debounceJob?.cancel()
        debounceJob = null
        receiver?.let { runCatching { context.unregisterReceiver(it) } }
        receiver = null
    }

    private companion object {
        const val DEBOUNCE_MILLIS = 2_000L
    }
}
