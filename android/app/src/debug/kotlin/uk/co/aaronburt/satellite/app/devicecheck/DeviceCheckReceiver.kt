package uk.co.aaronburt.satellite.app.devicecheck

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.telemetry.AudioControls

/**
 * Debug-only hook for `scripts/device-checks.sh`.
 *
 * Writes settings exactly as the settings screen would, so a running
 * MainActivity/service reacts to them live, and reports the current state back
 * as the broadcast's result data:
 *
 * ```
 * adb shell am broadcast -n uk.co.aaronburt.satellite/.app.devicecheck.DeviceCheckReceiver --es reporting false
 * ```
 */
@AndroidEntryPoint
class DeviceCheckReceiver : BroadcastReceiver() {

    @Inject lateinit var settings: SettingsRepository

    @Inject lateinit var audio: AudioControls

    override fun onReceive(context: Context, intent: Intent) {
        val extras = intent.extras?.keySet().orEmpty()
            .associateWith { intent.extras?.getString(it).orEmpty() }
        val pending = goAsync()
        scope.launch {
            try {
                DeviceCheckCommand.parse(extras).forEach { apply(it) }
                pending.setResult(RESULT_OK, report(), null)
            } catch (e: IllegalArgumentException) {
                pending.setResult(RESULT_BAD_REQUEST, "error=${e.message}", null)
            } finally {
                pending.finish()
            }
        }
    }

    private suspend fun apply(command: DeviceCheckCommand) {
        when (command) {
            is DeviceCheckCommand.ConfigureBroker -> settings.saveBrokerSettings(command.settings)
            is DeviceCheckCommand.SetInterval -> settings.saveUpdateInterval(command.interval)
            is DeviceCheckCommand.SetWebhook -> settings.saveWebhookSettings(command.settings)
            is DeviceCheckCommand.SetTransport -> settings.saveTransport(command.transport)
            is DeviceCheckCommand.SetControl -> settings.setControlEnabled(command.key, command.enabled)
            is DeviceCheckCommand.SetReporting -> settings.saveReportingEnabled(command.enabled)
        }
    }

    private suspend fun report(): String = DeviceCheckReport(
        deviceId = settings.deviceId(),
        reporting = settings.reportingEnabled.first(),
        transport = settings.transport.first(),
        enabledControls = settings.enabledControls.first(),
        micMuted = audio.isMicrophoneMuted(),
        volumePercent = audio.mediaVolumePercent(),
    ).format()

    private companion object {
        const val RESULT_OK = -1
        const val RESULT_BAD_REQUEST = 2
        val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    }
}
