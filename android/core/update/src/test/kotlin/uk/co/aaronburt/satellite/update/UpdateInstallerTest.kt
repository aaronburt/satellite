package uk.co.aaronburt.satellite.update

import android.content.Intent
import android.net.Uri
import android.provider.Settings
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeFalse
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import java.io.File

@RunWith(RobolectricTestRunner::class)
class UpdateInstallerTest {

    private val context = RuntimeEnvironment.getApplication()
    private val authority = "${context.packageName}.fileprovider"

    // The seam keeps the intent assertions OS-independent: androidx FileProvider
    // assumes '/' separators and cannot resolve Windows paths.
    private val installer = UpdateInstaller(context) { apk ->
        Uri.parse("content://$authority/updates/${apk.name}")
    }
    private val updates = File(context.filesDir, "updates").apply { mkdirs() }

    private fun allowInstalls(allowed: Boolean) =
        shadowOf(context.packageManager).setCanRequestPackageInstalls(allowed)

    private fun nextActivity(): Intent? = shadowOf(context).nextStartedActivity

    @Test
    fun `install permission follows the system setting`() {
        allowInstalls(false)
        assertFalse(installer.canInstall())
        allowInstalls(true)
        assertTrue(installer.canInstall())
    }

    @Test
    fun `the settings intent opens this app's unknown-sources page`() {
        val intent = installer.unknownSourcesIntent()

        assertEquals(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, intent.action)
        assertEquals("package:${context.packageName}", intent.dataString)
    }

    @Test
    fun `refuses to install without permission`() {
        allowInstalls(false)
        val apk = File(updates, "satellite.apk").apply { writeText("apk") }

        val result = installer.install(apk)

        assertTrue(result.isFailure)
        assertNull(nextActivity())
    }

    @Test
    fun `refuses to install a missing file`() {
        allowInstalls(true)

        val result = installer.install(File(updates, "missing.apk"))

        assertEquals("The downloaded update is missing", result.exceptionOrNull()?.message)
        assertNull(nextActivity())
    }

    @Test
    fun `hands the APK to the system installer through the file provider`() {
        allowInstalls(true)
        val apk = File(updates, "satellite.apk").apply { writeText("apk") }

        val result = installer.install(apk)

        assertTrue(result.exceptionOrNull()?.toString(), result.isSuccess)
        val intent = nextActivity()!!
        assertEquals(Intent.ACTION_VIEW, intent.action)
        assertEquals("application/vnd.android.package-archive", intent.type)
        assertEquals("content", intent.data?.scheme)
        assertEquals(authority, intent.data?.authority)
        assertTrue(intent.flags and Intent.FLAG_GRANT_READ_URI_PERMISSION != 0)
        assertTrue(intent.flags and Intent.FLAG_ACTIVITY_NEW_TASK != 0)
    }

    @Test
    fun `the real file provider exposes files under the updates directory`() {
        assumeFalse(
            "androidx FileProvider cannot resolve Windows paths",
            System.getProperty("os.name").startsWith("Windows"),
        )
        allowInstalls(true)
        val apk = File(updates, "satellite.apk").apply { writeText("apk") }

        val result = UpdateInstaller(context).install(apk)

        assertTrue(result.exceptionOrNull()?.toString(), result.isSuccess)
        val uri = nextActivity()!!.data!!
        assertEquals(authority, uri.authority)
        assertEquals("/updates/satellite.apk", uri.path)
    }
}
