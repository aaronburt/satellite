package uk.co.aaronburt.satellite.update

import android.content.Intent
import java.io.File
import java.util.concurrent.atomic.AtomicBoolean
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * Orchestrates the whole update flow: check → download → verify → install.
 *
 * The check is only ever started by an explicit user action (the app button or a
 * Home Assistant command), which keeps the app off GitHub's unauthenticated rate
 * limit and means it never phones home on its own.
 */
@Singleton
class UpdateManager @Inject constructor(
    private val installed: InstalledVersion,
    private val checker: ReleaseSource,
    private val downloader: ApkDownloader,
    private val verifier: ApkVerifier,
    private val installer: ApkInstaller
) {

    private val _state = MutableStateFlow<UpdateState>(UpdateState.Idle)
    val state: StateFlow<UpdateState> = _state.asStateFlow()

    private val downloadedApk = MutableStateFlow<File?>(null)

    private val inFlight = AtomicBoolean(false)

    val installedVersionName: String get() = installed.name

    val installedVersionCode: Long get() = installed.code

    /** False until the user allows Satellite to install unknown apps. */
    fun canInstall(): Boolean = installer.canInstall()

    fun unknownSourcesIntent(): Intent = installer.unknownSourcesIntent()

    /**
     * Checks for a newer release and, if there is one, downloads and verifies it
     * straight away so the user only has to confirm the install.
     */
    suspend fun checkAndDownload() {
        // Atomic, so two quick taps (or a tap plus a Home Assistant command)
        // cannot start two downloads of the same file.
        if (!inFlight.compareAndSet(false, true)) return

        _state.value = UpdateState.Checking
        downloadedApk.value = null
        try {
            val release = checker.latestRelease()
            val latestCode = release.versionCode

            if (latestCode == null || latestCode <= installedVersionCode) {
                _state.value = UpdateState.UpToDate(installedVersionName)
                return
            }

            val asset = release.apk ?: error("The latest release has no APK attached")

            _state.value = UpdateState.Downloading(release.versionName, 0)
            val apk = downloader.download(asset) { progress ->
                _state.value = UpdateState.Downloading(release.versionName, progress)
            }

            if (!verifier.verifySha256(apk, asset.sha256)) {
                apk.delete()
                error("The download did not match GitHub's checksum")
            }
            if (!verifier.signatureMatchesInstalledApp(apk)) {
                apk.delete()
                error("The update is not signed with the same key as this app")
            }

            downloadedApk.value = apk
            _state.value = UpdateState.Ready(
                versionName = release.versionName,
                versionCode = latestCode,
                notes = release.notes,
                releaseUrl = release.htmlUrl,
            )
        } catch (cancelled: CancellationException) {
            _state.value = UpdateState.Idle
            throw cancelled
        } catch (throwable: Throwable) {
            _state.value = UpdateState.Failed(
                throwable.message ?: throwable::class.simpleName ?: "Update check failed",
            )
        } finally {
            inFlight.set(false)
        }
    }

    /** Hands the verified APK to the platform installer. */
    fun install(): Result<Unit> {
        val apk = downloadedApk.value
            ?: return Result.failure(IllegalStateException("No update has been downloaded yet"))
        return installer.install(apk)
    }
}
