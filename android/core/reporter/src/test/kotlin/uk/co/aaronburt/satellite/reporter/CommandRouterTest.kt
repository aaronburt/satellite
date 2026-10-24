package uk.co.aaronburt.satellite.reporter

import android.media.AudioManager
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_MIC_MUTE
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_REFRESH
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_VOLUME
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.RefusingMicrophoneShadow

@RunWith(RobolectricTestRunner::class)
class CommandRouterTest {

    private val context = RuntimeEnvironment.getApplication()
    private val audioManager = context.getSystemService(AudioManager::class.java)
    private val audio = AudioControls(context)
    private val topics = Topics("abcd1234")

    private fun router(vararg enabled: String) =
        CommandRouter(FakeSettingsRepository(enabledControls = enabled.toSet()), audio)

    private val maxVolume get() = audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
    private val volume get() = audioManager.getStreamVolume(AudioManager.STREAM_MUSIC)

    @Test
    fun `sets the media volume when the volume control is enabled`() = runTest {
        val accepted = router(CONTROL_VOLUME).handle(topics.command("volume"), "100", topics)

        assertTrue(accepted)
        assertEquals(maxVolume, volume)
    }

    @Test
    fun `volume percent is scaled to the stream's steps`() = runTest {
        router(CONTROL_VOLUME).handle(topics.command("volume"), "0", topics)
        assertEquals(0, volume)

        router(CONTROL_VOLUME).handle(topics.command("volume"), "50", topics)
        assertEquals(Math.round(maxVolume * 0.5f), volume)
    }

    @Test
    fun `ignores volume when the control is not enabled`() = runTest {
        audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, 3, 0)

        val accepted = router().handle(topics.command("volume"), "100", topics)

        assertFalse(accepted)
        assertEquals(3, volume)
    }

    @Test
    fun `enabling one control does not enable the other`() = runTest {
        assertFalse(router(CONTROL_VOLUME).handle(topics.command("mute"), "ON", topics))
        assertFalse(audioManager.isMicrophoneMute)

        assertFalse(router(CONTROL_MIC_MUTE).handle(topics.command("volume"), "100", topics))
    }

    @Test
    fun `mutes and unmutes the microphone when enabled`() = runTest {
        val router = router(CONTROL_MIC_MUTE)

        assertTrue(router.handle(topics.command("mute"), "ON", topics))
        assertTrue(audioManager.isMicrophoneMute)

        assertTrue(router.handle(topics.command("mute"), "off", topics))
        assertFalse(audioManager.isMicrophoneMute)
    }

    @Test
    @Config(shadows = [RefusingMicrophoneShadow::class])
    fun `a refused microphone change is still accepted but marks the control unsupported`() = runTest {
        val accepted = router(CONTROL_MIC_MUTE).handle(topics.command("mute"), "ON", topics)

        // Accepted, so the caller republishes the real (unchanged) state.
        assertTrue(accepted)
        assertFalse(audioManager.isMicrophoneMute)
        assertEquals(false, audio.microphoneMuteSupported.value)
    }

    @Test
    fun `malformed or foreign commands are rejected without touching audio`() = runTest {
        val router = router(CONTROL_VOLUME, CONTROL_MIC_MUTE)
        audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, 2, 0)

        assertFalse(router.handle(topics.command("volume"), "loud", topics))
        assertFalse(router.handle(topics.command("volume"), "101", topics))
        assertFalse(router.handle(topics.command("mute"), "maybe", topics))
        assertFalse(router.handle(topics.command("reboot"), "now", topics))
        assertFalse(router.handle(Topics("someoneelse").command("volume"), "100", topics))

        assertEquals(2, volume)
        assertFalse(audioManager.isMicrophoneMute)
        assertNull(audio.microphoneMuteSupported.value)
    }

    @Test
    fun `accepts a refresh press only when the refresh control is enabled`() = runTest {
        assertTrue(router(CONTROL_REFRESH).handle(topics.command("refresh"), "PRESS", topics))
        assertFalse(router().handle(topics.command("refresh"), "PRESS", topics))
        assertFalse(router(CONTROL_VOLUME, CONTROL_MIC_MUTE).handle(topics.command("refresh"), "PRESS", topics))
    }

    @Test
    fun `enabling refresh does not enable the audio controls`() = runTest {
        assertFalse(router(CONTROL_REFRESH).handle(topics.command("volume"), "100", topics))
        assertFalse(router(CONTROL_REFRESH).handle(topics.command("mute"), "ON", topics))
    }
}
