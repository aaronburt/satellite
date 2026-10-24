package uk.co.aaronburt.satellite.update

import android.content.Intent
import java.io.File
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class UpdateEntityStatesTest {

    private val release = GitHubRelease(
        tag = "v0.6.0",
        versionCode = 600,
        versionName = "0.6.0",
        title = "v0.6.0",
        notes = null,
        htmlUrl = "https://example.test/r",
        apk = null,
    )

    private fun source(result: () -> GitHubRelease) = object : ReleaseSource {
        override suspend fun latestRelease(): GitHubRelease = result()
    }

    private fun manager(source: ReleaseSource = source { release }) = UpdateManager(
        installed = object : InstalledVersion {
            override val name = "0.5.1"
            override val code = 501L
        },
        checker = source,
        downloader = object : ApkDownloader {
            override suspend fun download(asset: ReleaseAsset, onProgress: (Int) -> Unit): File = error("unused")
        },
        verifier = object : ApkVerifier {
            override fun verifySha256(file: File, expected: String?) = true
            override fun signatureMatchesInstalledApp(apk: File) = true
        },
        installer = object : ApkInstaller {
            override fun canInstall() = true
            override fun unknownSourcesIntent() = Intent()
            override fun install(apk: File) = Result.success(Unit)
        },
    )

    @Test
    fun `reads the live state of the manager`() = runTest {
        val manager = manager(source { throw IllegalStateException("offline") })
        val states = UpdateEntityStates(manager)

        assertEquals("idle", states.read().getValue("update_status"))

        manager.checkAndDownload()

        assertEquals("failed", states.read().getValue("update_status"))
    }

    @Test
    fun `changes emits for each distinct state and not for repeats`() = runTest {
        val manager = manager(source { throw IllegalStateException("offline") })
        val states = UpdateEntityStates(manager)
        val seen = mutableListOf<String>()
        val job = launch(UnconfinedTestDispatcher(testScheduler)) {
            states.changes.collect { seen += states.read().getValue("update_status") }
        }

        manager.checkAndDownload()
        manager.checkAndDownload()
        job.cancel()

        // Each run walks idle/failed -> checking -> failed.
        assertEquals(listOf("idle", "checking", "failed", "checking", "failed"), seen)
    }

    @Test
    fun `no-op source reports nothing`() = runTest {
        assertEquals(emptyMap<String, String>(), NoUpdateStates.read())
        assertEquals(emptyList<Unit>(), NoUpdateStates.changes.toList())
    }
}
