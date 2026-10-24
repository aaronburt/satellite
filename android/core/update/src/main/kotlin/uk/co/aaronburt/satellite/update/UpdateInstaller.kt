package uk.co.aaronburt.satellite.update

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.core.content.FileProvider
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Hands a verified APK to the platform installer.
 *
 * This deliberately uses the `ACTION_VIEW` hand-off to the system package
 * installer rather than `PackageInstaller` sessions: the session route fails on
 * some system images (the emulator's API 36 image crashes inside
 * `PackageInstallerSession.commit`), and the intent route is the widely
 * compatible one. Either way Android shows its own confirmation, so this is a
 * one-tap upgrade rather than a silent one.
 */
@Singleton
class UpdateInstaller internal constructor(private val context: Context, private val uriFor: (File) -> Uri) :
    ApkInstaller {

    @Inject
    constructor(@ApplicationContext context: Context) : this(
        context,
        { apk -> FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", apk) },
    )

    /** False until the user allows Satellite to install unknown apps. */
    override fun canInstall(): Boolean = context.packageManager.canRequestPackageInstalls()

    override fun unknownSourcesIntent(): Intent = Intent(
        Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
        Uri.parse("package:${context.packageName}"),
    )

    override fun install(apk: File): Result<Unit> = runCatching {
        check(canInstall()) { "Satellite is not allowed to install apps yet" }
        check(apk.exists()) { "The downloaded update is missing" }

        val uri = uriFor(apk)

        val intent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, APK_MIME_TYPE)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(intent)
    }

    private companion object {
        const val APK_MIME_TYPE = "application/vnd.android.package-archive"
    }
}
