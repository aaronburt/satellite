package uk.co.aaronburt.satellite.update

import android.content.Context
import android.content.Intent
import android.content.pm.PackageInfo
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Orchestrates the whole update flow: check → download → verify → install.
 *
 * The check is only ever started by an explicit user action (the app button or a
 * Home Assistant command), which keeps the app off GitHub's unauthenticated rate
 * limit and means it never phones home on its own.
 */
@Singleton
class UpdateManager @Inject constructor(
    @ApplicationContext private val context: Context,
    private val checker: UpdateChecker,
    private val downloader: UpdateDownloader,
    private val verifier: UpdateVerifier,
    private val installer: UpdateInstaller,
) {

    private val _state = MutableStateFlow<UpdateState>(UpdateState.Idle)
    val state: StateFlow<UpdateState> = _state.asStateFlow()

    private val downloadedApk = MutableStateFlow<File?>(null)

    val installedVersionName: String get() = packageInfo()?.versionName ?: "unknown"

    val installedVersionCode: Long
        get() = packageInfo()?.let { info ->
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                info.longVersionCode
            } else {
                @Suppress("DEPRECATION") info.versionCode.toLong()
            }
        } ?: 0L

    /** False until the user allows Satellite to install unknown apps. */
    fun canInstall(): Boolean = installer.canInstall()

    fun unknownSourcesIntent(): Intent = installer.unknownSourcesIntent()

    /**
     * Checks for a newer release and, if there is one, downloads and verifies it
     * straight away so the user only has to confirm the install.
     */
    suspend fun checkAndDownload() {
        if (_state.value is UpdateState.Checking || _state.value is UpdateState.Downloading) return

        _state.value = UpdateState.Checking
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
        } catch (throwable: Throwable) {
            _state.value = UpdateState.Failed(
                throwable.message ?: throwable::class.simpleName ?: "Update check failed",
            )
        }
    }

    /** Hands the verified APK to the platform installer. */
    fun install(): Result<Unit> {
        val apk = downloadedApk.value
            ?: return Result.failure(IllegalStateException("No update has been downloaded yet"))
        return installer.install(apk)
    }

    private fun packageInfo(): PackageInfo? = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0)
    }.getOrNull()
}
