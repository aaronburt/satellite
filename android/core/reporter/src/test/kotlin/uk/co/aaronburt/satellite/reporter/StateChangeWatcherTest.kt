package uk.co.aaronburt.satellite.reporter

import android.content.Intent
import android.net.ConnectivityManager
import android.os.PowerManager
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import uk.co.aaronburt.satellite.testing.idleMainLooper
import uk.co.aaronburt.satellite.testing.settle
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest

@RunWith(RobolectricTestRunner::class)
class StateChangeWatcherTest {

    private val context = RuntimeEnvironment.getApplication()
    private var active = true
    private var changes = 0

    private fun TestScope.watcher() = StateChangeWatcher(
        context = context,
        scope = backgroundScope,
        isActive = { active },
        onChanged = { changes++ },
    )

    private fun send(action: String) {
        context.sendBroadcast(Intent(action))
        idleMainLooper()
    }

    @Test
    fun `reports a change two seconds after a relevant broadcast`() = runTest {
        watcher().register()

        send(Intent.ACTION_POWER_DISCONNECTED)
        settle(1_999)
        assertEquals(0, changes)

        settle(2)
        assertEquals(1, changes)
    }

    @Test
    fun `a burst of broadcasts produces one change`() = runTest {
        watcher().register()

        send(Intent.ACTION_SCREEN_ON)
        settle(500)
        send(Intent.ACTION_USER_PRESENT)
        settle(500)
        send(Intent.ACTION_BATTERY_CHANGED)
        settle(5_000)

        assertEquals(1, changes)
    }

    @Test
    fun `every watched action counts`() = runTest {
        watcher().register()
        val actions = listOf(
            Intent.ACTION_BATTERY_CHANGED,
            Intent.ACTION_POWER_CONNECTED,
            Intent.ACTION_POWER_DISCONNECTED,
            Intent.ACTION_SCREEN_ON,
            Intent.ACTION_SCREEN_OFF,
            Intent.ACTION_USER_PRESENT,
            PowerManager.ACTION_POWER_SAVE_MODE_CHANGED,
            PowerManager.ACTION_DEVICE_IDLE_MODE_CHANGED,
            @Suppress("DEPRECATION") ConnectivityManager.CONNECTIVITY_ACTION,
        )

        actions.forEach { action ->
            send(action)
            settle(2_500)
        }

        assertEquals(actions.size, changes)
    }

    @Test
    fun `unrelated broadcasts are ignored`() = runTest {
        watcher().register()

        send(Intent.ACTION_TIMEZONE_CHANGED)
        settle(5_000)

        assertEquals(0, changes)
    }

    @Test
    fun `broadcasts are ignored while inactive`() = runTest {
        active = false
        watcher().register()

        send(Intent.ACTION_POWER_CONNECTED)
        settle(5_000)

        assertEquals(0, changes)
    }

    @Test
    fun `registering twice registers one receiver`() = runTest {
        val watcher = watcher()
        watcher.register()
        watcher.register()

        assertEquals(1, shadowOf(context).registeredReceivers.size)
        send(Intent.ACTION_POWER_CONNECTED)
        settle(3_000)
        assertEquals(1, changes)
    }

    @Test
    fun `unregister stops reporting and cancels a pending change`() = runTest {
        val watcher = watcher()
        watcher.register()
        send(Intent.ACTION_POWER_CONNECTED)

        watcher.unregister()
        settle(5_000)
        send(Intent.ACTION_POWER_CONNECTED)
        settle(5_000)

        assertEquals(0, changes)
        assertEquals(0, shadowOf(context).registeredReceivers.size)
    }

    @Test
    fun `can register again after unregistering`() = runTest {
        val watcher = watcher()
        watcher.register()
        watcher.unregister()
        watcher.unregister() // harmless when already unregistered
        watcher.register()

        send(Intent.ACTION_POWER_CONNECTED)
        settle(3_000)

        assertEquals(1, changes)
    }
}
