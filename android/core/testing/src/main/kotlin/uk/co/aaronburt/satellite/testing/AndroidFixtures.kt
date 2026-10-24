package uk.co.aaronburt.satellite.testing

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.media.AudioManager
import android.os.BatteryManager
import android.os.Looper
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Implementation
import org.robolectric.annotation.Implements
import org.robolectric.shadows.ShadowAudioManager

/** Publishes a sticky battery broadcast, which is what the app reads battery state from. */
// This test-only helper runs against Robolectric's shadow context, not a device.
@SuppressLint("MissingPermission")
@Suppress("DEPRECATION")
fun Context.setBattery(
    level: Int,
    scale: Int = 100,
    plugged: Int = 0,
    status: Int = BatteryManager.BATTERY_STATUS_DISCHARGING,
    temperatureTenths: Int? = 250
) {
    val intent = Intent(Intent.ACTION_BATTERY_CHANGED)
        .putExtra(BatteryManager.EXTRA_LEVEL, level)
        .putExtra(BatteryManager.EXTRA_SCALE, scale)
        .putExtra(BatteryManager.EXTRA_PLUGGED, plugged)
        .putExtra(BatteryManager.EXTRA_STATUS, status)
    temperatureTenths?.let { intent.putExtra(BatteryManager.EXTRA_TEMPERATURE, it) }
    sendStickyBroadcast(intent)
}

/** Runs everything queued on the main looper (broadcast delivery, for example). */
fun idleMainLooper() {
    shadowOf(Looper.getMainLooper()).idle()
}

/**
 * An [AudioManager] that ignores microphone-mute writes, like Android 12+
 * does for most apps. Use with `@Config(shadows = [RefusingMicrophoneShadow::class])`.
 */
@Implements(AudioManager::class)
class RefusingMicrophoneShadow : ShadowAudioManager() {
    @Implementation
    override fun setMicrophoneMute(on: Boolean) {
        // Refused: the observed state does not change.
    }
}
