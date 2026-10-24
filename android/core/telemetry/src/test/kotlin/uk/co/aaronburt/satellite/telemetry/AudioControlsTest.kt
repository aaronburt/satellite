package uk.co.aaronburt.satellite.telemetry

import android.media.AudioManager
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import uk.co.aaronburt.satellite.testing.RefusingMicrophoneShadow

@RunWith(RobolectricTestRunner::class)
class AudioControlsTest {

    private val context = RuntimeEnvironment.getApplication()
    private val audioManager = context.getSystemService(AudioManager::class.java)
    private val audio = AudioControls(context)

    private fun useSteps(max: Int) = shadowOf(audioManager).setStreamMaxVolume(max)
    private val musicVolume get() = audioManager.getStreamVolume(AudioManager.STREAM_MUSIC)

    @Test
    fun `reports media volume as a percentage of the stream's steps`() {
        useSteps(15)
        audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, 0, 0)
        assertEquals(0, audio.mediaVolumePercent())

        audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, 5, 0)
        assertEquals(33, audio.mediaVolumePercent())

        audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, 15, 0)
        assertEquals(100, audio.mediaVolumePercent())
    }

    @Test
    fun `media volume is unknown when the stream has no steps`() {
        useSteps(0)
        assertNull(audio.mediaVolumePercent())
    }

    @Test
    fun `sets media volume to the nearest step`() {
        useSteps(15)

        assertTrue(audio.setMediaVolumePercent(50))
        assertEquals(8, musicVolume) // 7.5 rounds up

        assertTrue(audio.setMediaVolumePercent(10))
        assertEquals(2, musicVolume) // 1.5 rounds up

        assertTrue(audio.setMediaVolumePercent(100))
        assertEquals(15, musicVolume)
    }

    @Test
    fun `out-of-range volume is clamped`() {
        useSteps(15)

        audio.setMediaVolumePercent(150)
        assertEquals(15, musicVolume)

        audio.setMediaVolumePercent(-20)
        assertEquals(0, musicVolume)
    }

    @Test
    fun `set then read gives back roughly the same percentage`() {
        useSteps(15)
        for (percent in 0..100 step 5) {
            audio.setMediaVolumePercent(percent)
            val readBack = audio.mediaVolumePercent()!!
            assertTrue("$percent -> $readBack", kotlin.math.abs(readBack - percent) <= 100 / 15 / 2 + 1)
        }
    }

    @Test
    fun `mutes and unmutes the microphone`() {
        assertTrue(audio.setMicrophoneMuted(true))
        assertEquals(true, audio.isMicrophoneMuted())

        assertTrue(audio.setMicrophoneMuted(false))
        assertEquals(false, audio.isMicrophoneMuted())
    }

    @Test
    fun `probing a device that allows mic mute reports supported and leaves the mic as it was`() {
        audioManager.isMicrophoneMute = true

        assertTrue(audio.probeMicrophoneMute())

        assertEquals(true, audio.microphoneMuteSupported.value)
        assertTrue(audioManager.isMicrophoneMute)
    }

    @Test
    fun `support is unknown until probed`() {
        assertNull(audio.microphoneMuteSupported.value)
    }

    @Test
    @Config(shadows = [RefusingMicrophoneShadow::class])
    fun `a device that refuses mic mute is detected`() {
        assertFalse(audio.setMicrophoneMuted(true))
        assertEquals(false, audio.isMicrophoneMuted())

        assertFalse(audio.probeMicrophoneMute())
        assertEquals(false, audio.microphoneMuteSupported.value)
    }

    @Test
    fun `a refusal seen later marks the control unsupported`() {
        audio.probeMicrophoneMute()
        audio.markMicrophoneMuteUnsupported()
        assertEquals(false, audio.microphoneMuteSupported.value)
    }
}
