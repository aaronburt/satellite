package uk.co.aaronburt.satellite.update

import android.content.Intent
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.async
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.io.File

@RunWith(RobolectricTestRunner::class)
class UpdateManagerTest {

    @get:Rule
    val folder = TemporaryFolder()

    private val installed = object : InstalledVersion {
        override val name = "0.5.1"
        override val code = 501L
    }

    private fun release(tag: String = "v0.6.0", apk: ReleaseAsset? = asset()) = GitHubRelease(
        tag = tag,
        versionCode = VersionCodes.fromTag(tag),
        versionName = tag.removePrefix("v"),
        title = tag,
        notes = "Changes",
        htmlUrl = "https://github.com/aaronburt/satellite/releases/tag/$tag",
        apk = apk,
    )

    private fun asset(sha: String? = "abc") = ReleaseAsset(
        name = "satellite.apk",
        downloadUrl = "https://github.com/aaronburt/satellite/releases/download/v0.6.0/satellite.apk",
        size = 100,
        sha256 = sha,
    )

    private inner class FakeSource(var result: () -> GitHubRelease) : ReleaseSource {
        var calls = 0
        override suspend fun latestRelease(): GitHubRelease {
            calls++
            return result()
        }
    }

    private inner class FakeDownloader : ApkDownloader {
        var calls = 0
        var gate: CompletableDeferred<Unit>? = null
        val progressSeen = mutableListOf<UpdateState>()
        lateinit var manager: UpdateManager

        override suspend fun download(asset: ReleaseAsset, onProgress: (Int) -> Unit): File {
            calls++
            gate?.await()
            listOf(0, 50, 100).forEach {
                onProgress(it)
                progressSeen += manager.state.value
            }
            return folder.newFile("download-$calls.apk")
        }
    }

    private class FakeVerifier(var checksumOk: Boolean = true, var signatureOk: Boolean = true) : ApkVerifier {
        val checked = mutableListOf<String?>()
        override fun verifySha256(file: File, expected: String?): Boolean {
            checked += expected
            return checksumOk
        }
        override fun signatureMatchesInstalledApp(apk: File) = signatureOk
    }

    private class FakeInstaller : ApkInstaller {
        var allowed = true
        val installed = mutableListOf<File>()
        var result: Result<Unit> = Result.success(Unit)
        override fun canInstall() = allowed
        override fun unknownSourcesIntent() = Intent("settings")
        override fun install(apk: File): Result<Unit> {
            installed += apk
            return result
        }
    }

    private val source = FakeSource { release() }
    private val downloader = FakeDownloader()
    private val verifier = FakeVerifier()
    private val installer = FakeInstaller()
    private val manager = UpdateManager(installed, source, downloader, verifier, installer)
        .also { downloader.manager = it }

    @Test
    fun `starts idle and exposes the installed version`() {
        assertEquals(UpdateState.Idle, manager.state.value)
        assertEquals("0.5.1", manager.installedVersionName)
        assertEquals(501L, manager.installedVersionCode)
    }

    @Test
    fun `a newer release is downloaded, verified and made ready`() = runTest {
        manager.checkAndDownload()

        assertEquals(
            UpdateState.Ready(
                versionName = "0.6.0",
                versionCode = 600,
                notes = "Changes",
                releaseUrl = "https://github.com/aaronburt/satellite/releases/tag/v0.6.0",
            ),
            manager.state.value,
        )
        assertEquals(listOf<String?>("abc"), verifier.checked)
    }

    @Test
    fun `download progress is reported`() = runTest {
        manager.checkAndDownload()

        assertEquals(
            listOf(0, 50, 100).map { UpdateState.Downloading("0.6.0", it) },
            downloader.progressSeen,
        )
    }

    @Test
    fun `install hands the verified download to the installer`() = runTest {
        manager.checkAndDownload()

        assertTrue(manager.install().isSuccess)
        assertEquals("download-1.apk", installer.installed.single().name)
    }

    @Test
    fun `install before a download fails without calling the installer`() {
        val result = manager.install()

        assertTrue(result.isFailure)
        assertTrue(installer.installed.isEmpty())
    }

    @Test
    fun `install failures are passed back`() = runTest {
        installer.result = Result.failure(IllegalStateException("not allowed"))
        manager.checkAndDownload()

        assertEquals("not allowed", manager.install().exceptionOrNull()?.message)
    }

    @Test
    fun `same or older release is up to date and nothing is downloaded`() = runTest {
        for (tag in listOf("v0.5.1", "v0.5.0", "v0.4.9")) {
            source.result = { release(tag) }
            manager.checkAndDownload()
            assertEquals(tag, UpdateState.UpToDate("0.5.1"), manager.state.value)
        }
        assertEquals(0, downloader.calls)
    }

    @Test
    fun `a release tag without a version is treated as up to date`() = runTest {
        source.result = { release("nightly") }
        manager.checkAndDownload()

        assertEquals(UpdateState.UpToDate("0.5.1"), manager.state.value)
        assertEquals(0, downloader.calls)
    }

    @Test
    fun `a release without an APK fails`() = runTest {
        source.result = { release(apk = null) }
        manager.checkAndDownload()

        assertEquals(UpdateState.Failed("The latest release has no APK attached"), manager.state.value)
    }

    @Test
    fun `a checksum mismatch deletes the download and fails`() = runTest {
        verifier.checksumOk = false
        manager.checkAndDownload()

        assertEquals(UpdateState.Failed("The download did not match GitHub's checksum"), manager.state.value)
        assertFalse(File(folder.root, "download-1.apk").exists())
        assertTrue(manager.install().isFailure)
    }

    @Test
    fun `a different signing key deletes the download and fails`() = runTest {
        verifier.signatureOk = false
        manager.checkAndDownload()

        assertEquals(
            UpdateState.Failed("The update is not signed with the same key as this app"),
            manager.state.value,
        )
        assertFalse(File(folder.root, "download-1.apk").exists())
        assertTrue(manager.install().isFailure)
    }

    @Test
    fun `a failing check is reported with its message`() = runTest {
        source.result = { error("GitHub returned HTTP 403") }
        manager.checkAndDownload()

        assertEquals(UpdateState.Failed("GitHub returned HTTP 403"), manager.state.value)
    }

    @Test
    fun `an error without a message is still reported`() = runTest {
        source.result = { throw java.io.IOException() }
        manager.checkAndDownload()

        assertEquals(UpdateState.Failed("IOException"), manager.state.value)
    }

    @Test
    fun `a second check while one is running is ignored`() = runTest {
        downloader.gate = CompletableDeferred()
        val first = launch { manager.checkAndDownload() }
        runCurrent()
        assertTrue(manager.state.value is UpdateState.Downloading)

        manager.checkAndDownload()
        downloader.gate!!.complete(Unit)
        first.join()

        assertEquals(1, source.calls)
        assertEquals(1, downloader.calls)
        assertTrue(manager.state.value is UpdateState.Ready)
    }

    @Test
    fun `a cancelled check goes back to idle and can run again`() = runTest {
        downloader.gate = CompletableDeferred()
        val running = async { manager.checkAndDownload() }
        runCurrent()

        running.cancel()
        runCurrent()
        assertEquals(UpdateState.Idle, manager.state.value)

        downloader.gate = null
        manager.checkAndDownload()
        assertTrue(manager.state.value is UpdateState.Ready)
    }

    @Test
    fun `a later failed check forgets the earlier download`() = runTest {
        manager.checkAndDownload()
        verifier.signatureOk = false
        source.result = { release("v0.7.0") }

        manager.checkAndDownload()

        assertTrue(manager.state.value is UpdateState.Failed)
        assertTrue("must not install the stale 0.6.0 download", manager.install().isFailure)
    }

    @Test
    fun `can check again after finishing`() = runTest {
        source.result = { release("v0.5.1") }
        manager.checkAndDownload()
        source.result = { release("v0.6.0") }
        manager.checkAndDownload()

        assertEquals(2, source.calls)
        assertTrue(manager.state.value is UpdateState.Ready)
    }

    @Test
    fun `install permission and settings intent come from the installer`() {
        installer.allowed = false
        assertFalse(manager.canInstall())
        installer.allowed = true
        assertTrue(manager.canInstall())
        assertEquals("settings", manager.unknownSourcesIntent().action)
    }
}
