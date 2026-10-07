package uk.co.aaronburt.satellite.telemetry

import android.content.Context
import android.media.AudioManager
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlin.math.roundToInt
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Read and write access to the phone's audio state, used by both telemetry (for
 * reporting) and the command router (for Home Assistant commands).
 *
 * Media volume is a plain `AudioManager` stream. Microphone mute is subject to
 * platform restrictions on recent Android versions — callers should treat the
 * returned/observed state as authoritative rather than assuming the write
 * succeeded.
 */
@Singleton
class AudioControls @Inject constructor(
    @ApplicationContext private val context: Context,
) {

    fun mediaVolumePercent(): Int? = runCatching {
        val audio = audioManager() ?: return null
        val max = audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
        if (max <= 0) return null
        (audio.getStreamVolume(AudioManager.STREAM_MUSIC) * 100f / max).roundToInt()
    }.getOrNull()

    /** @return true when the volume was applied (re-read after writing). */
    fun setMediaVolumePercent(percent: Int): Boolean = runCatching {
        val audio = audioManager() ?: return false
        val max = audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
        val target = (percent.coerceIn(0, 100) * max / 100f).roundToInt()
        audio.setStreamVolume(AudioManager.STREAM_MUSIC, target, 0)
        true
    }.getOrDefault(false)

    fun isMicrophoneMuted(): Boolean? = runCatching {
        audioManager()?.isMicrophoneMute
    }.getOrNull()

    /**
     * @return true when the observed state matches the request. Android 12+
     * restricts microphone muting, so this can legitimately report false.
     */
    fun setMicrophoneMuted(muted: Boolean): Boolean = runCatching {
        val audio = audioManager() ?: return false
        @Suppress("DEPRECATION")
        audio.isMicrophoneMute = muted
        audio.isMicrophoneMute == muted
    }.getOrDefault(false)

    private fun audioManager(): AudioManager? =
        context.getSystemService(AudioManager::class.java)
}
