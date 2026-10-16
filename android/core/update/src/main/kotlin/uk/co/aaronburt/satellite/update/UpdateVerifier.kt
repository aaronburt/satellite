package uk.co.aaronburt.satellite.update

import android.content.Context
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.content.pm.Signature
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import java.security.MessageDigest
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Checks a downloaded APK before it is offered for install:
 *
 * 1. the SHA-256 matches what GitHub reported for the asset, and
 * 2. it is signed by the **same key** as the installed app.
 *
 * The second check is the important one — Android would refuse the install
 * anyway, but verifying first means we never hand the user an APK we cannot trust.
 */
@Singleton
class UpdateVerifier @Inject constructor(
    @ApplicationContext private val context: Context,
) {

    fun verifySha256(file: File, expected: String?): Boolean {
        if (expected.isNullOrBlank()) return true
        val actual = sha256(file) ?: return false
        return actual.equals(expected, ignoreCase = true)
    }

    fun signatureMatchesInstalledApp(apk: File): Boolean = runCatching {
        val packageManager = context.packageManager
        val flags = signatureFlags()

        val installed = packageManager.getPackageInfo(context.packageName, flags)
        val archive = packageManager.getPackageArchiveInfo(apk.absolutePath, flags) ?: return false

        val installedCerts = certDigests(installed)
        val archiveCerts = certDigests(archive)

        installedCerts.isNotEmpty() && installedCerts == archiveCerts
    }.getOrDefault(false)

    @Suppress("DEPRECATION")
    private fun signatureFlags(): Int =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            PackageManager.GET_SIGNING_CERTIFICATES
        } else {
            PackageManager.GET_SIGNATURES
        }

    @Suppress("DEPRECATION")
    private fun certDigests(info: PackageInfo): Set<String> {
        val signatures: Array<Signature> = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            info.signingInfo?.apkContentsSigners ?: return emptySet()
        } else {
            info.signatures ?: return emptySet()
        }
        return signatures.mapTo(mutableSetOf()) { sha256(it.toByteArray()) ?: "" }
    }

    private fun sha256(file: File): String? = runCatching {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(64 * 1024)
            while (true) {
                val read = input.read(buffer)
                if (read <= 0) break
                digest.update(buffer, 0, read)
            }
        }
        digest.digest().toHex()
    }.getOrNull()

    private fun sha256(bytes: ByteArray): String? =
        runCatching { MessageDigest.getInstance("SHA-256").digest(bytes).toHex() }.getOrNull()

    private fun ByteArray.toHex(): String =
        joinToString(separator = "") { byte -> "%02x".format(byte) }
}
