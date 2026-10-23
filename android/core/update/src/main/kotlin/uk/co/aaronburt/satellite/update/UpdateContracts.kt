package uk.co.aaronburt.satellite.update

import android.content.Intent
import java.io.File

/** Where release metadata comes from (GitHub in the app). */
interface ReleaseSource {
    suspend fun latestRelease(): GitHubRelease
}

/** Fetches a release asset to local storage, reporting progress 0–100. */
interface ApkDownloader {
    suspend fun download(asset: ReleaseAsset, onProgress: (Int) -> Unit): File
}

/** Decides whether a downloaded APK can be trusted. */
interface ApkVerifier {
    fun verifySha256(file: File, expected: String?): Boolean
    fun signatureMatchesInstalledApp(apk: File): Boolean
}

/** Hands an APK to the platform installer. */
interface ApkInstaller {
    fun canInstall(): Boolean
    fun unknownSourcesIntent(): Intent
    fun install(apk: File): Result<Unit>
}

/** The version of the app that is running now. */
interface InstalledVersion {
    val name: String
    val code: Long
}
