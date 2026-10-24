package uk.co.aaronburt.satellite.update

import android.content.Context
import android.content.pm.PackageInfo
import android.os.Build
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

/** Reads the running app's version from the package manager. */
class PackageInstalledVersion @Inject constructor(
    @ApplicationContext private val context: Context,
) : InstalledVersion {

    override val name: String get() = packageInfo()?.versionName ?: "unknown"

    override val code: Long
        get() = packageInfo()?.let { info ->
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                info.longVersionCode
            } else {
                @Suppress("DEPRECATION") info.versionCode.toLong()
            }
        } ?: 0L

    private fun packageInfo(): PackageInfo? = runCatching {
        context.packageManager.getPackageInfo(context.packageName, 0)
    }.getOrNull()
}
