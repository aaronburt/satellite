package uk.co.aaronburt.satellite.update

import android.annotation.SuppressLint
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.net.Uri
import android.os.Build
import android.provider.Settings
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Hands a verified APK to the platform installer. Android always shows its own
 * confirmation, so this is a one-tap upgrade rather than a silent one.
 */
@Singleton
class UpdateInstaller @Inject constructor(
    @ApplicationContext private val context: Context,
) {

    /** False until the user allows Satellite to install unknown apps. */
    fun canInstall(): Boolean = context.packageManager.canRequestPackageInstalls()

    fun unknownSourcesIntent(): Intent = Intent(
        Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
        Uri.parse("package:${context.packageName}"),
    )

    // An app may always install an update for itself; the lint annotation on
    // createSession assumes a generic installer.
    @SuppressLint("MissingPermission")
    fun install(apk: File): Result<Unit> = runCatching {
        check(canInstall()) { "Satellite is not allowed to install apps yet" }

        val installer = context.packageManager.packageInstaller
        val params = PackageInstaller.SessionParams(
            PackageInstaller.SessionParams.MODE_FULL_INSTALL,
        ).apply {
            setAppPackageName(context.packageName)
        }

        val sessionId = installer.createSession(params)
        installer.openSession(sessionId).use { session ->
            session.openWrite(SESSION_NAME, 0, apk.length()).use { output ->
                apk.inputStream().use { input -> input.copyTo(output) }
                session.fsync(output)
            }

            val launch = context.packageManager.getLaunchIntentForPackage(context.packageName)
                ?: Intent(Intent.ACTION_MAIN)

            // commit() hands the result back through this PendingIntent, so on
            // Android 12+ it must be mutable for the system to fill it in.
            val flags = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE
            } else {
                PendingIntent.FLAG_UPDATE_CURRENT
            }
            val pendingIntent = PendingIntent.getActivity(context, sessionId, launch, flags)
            session.commit(pendingIntent.intentSender)
        }
    }

    private companion object {
        const val SESSION_NAME = "satellite-update.apk"
    }
}
