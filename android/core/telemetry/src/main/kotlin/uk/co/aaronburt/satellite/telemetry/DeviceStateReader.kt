package uk.co.aaronburt.satellite.telemetry

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.media.AudioManager
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.wifi.WifiManager
import android.os.BatteryManager
import android.os.Build
import android.os.Environment
import android.os.PowerManager
import android.os.StatFs
import android.os.SystemClock
import android.telephony.TelephonyManager
import androidx.core.content.ContextCompat
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlin.math.roundToInt
import javax.inject.Inject

/**
 * Reads the current device state as a map of entity key → MQTT payload.
 *
 * Only the no-permission set is read unconditionally; permission-gated values
 * are included only when the user has granted the relevant permission.
 */
class DeviceStateReader @Inject constructor(
    @ApplicationContext private val context: Context,
) {

    @Suppress("InlinedApi")
    fun read(): Map<String, String> {
        val values = mutableMapOf<String, String>()
        // Each section is isolated so a single unavailable API can never crash
        // the reporter; failures simply omit that value.
        runCatching { readBattery(values) }
        runCatching { readPower(values) }
        runCatching { readApp(values) }
        runCatching { readNetwork(values) }
        runCatching { readStorage(values) }
        runCatching { readAudio(values) }
        runCatching { readPermissionGated(values) }
        return values
    }

    private fun readBattery(values: MutableMap<String, String>) {
        val intent = context.registerReceiver(null, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
            ?: return

        val level = intent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1)
        val scale = intent.getIntExtra(BatteryManager.EXTRA_SCALE, 100)
        if (level >= 0 && scale > 0) {
            values["battery"] = ((level * 100f) / scale).roundToInt().toString()
        }

        val status = intent.getIntExtra(BatteryManager.EXTRA_STATUS, -1)
        val plugged = intent.getIntExtra(BatteryManager.EXTRA_PLUGGED, 0)
        val charging = plugged != 0 ||
            status == BatteryManager.BATTERY_STATUS_CHARGING ||
            status == BatteryManager.BATTERY_STATUS_FULL
        values["charging"] = if (charging) "ON" else "OFF"

        values["charger_type"] = when (plugged) {
            BatteryManager.BATTERY_PLUGGED_AC -> "ac"
            BatteryManager.BATTERY_PLUGGED_USB -> "usb"
            BatteryManager.BATTERY_PLUGGED_WIRELESS -> "wireless"
            BATTERY_PLUGGED_DOCK -> "dock"
            else -> "none"
        }

        val temperature = intent.getIntExtra(BatteryManager.EXTRA_TEMPERATURE, Int.MIN_VALUE)
        if (temperature != Int.MIN_VALUE) {
            values["battery_temperature"] = (temperature / 10.0).toString()
        }
    }

    private fun readPower(values: MutableMap<String, String>) {
        val powerManager = context.getSystemService(PowerManager::class.java) ?: return
        values["power_save"] = if (powerManager.isPowerSaveMode) "ON" else "OFF"
        values["interactive"] = if (powerManager.isInteractive) "ON" else "OFF"
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            values["doze"] = if (powerManager.isDeviceIdleMode) "ON" else "OFF"
        }
    }

    private fun readApp(values: MutableMap<String, String>) {
        val versionName = runCatching {
            context.packageManager.getPackageInfo(context.packageName, 0).versionName
        }.getOrNull()
        values["app_version"] = versionName ?: "unknown"
        values["android_version"] = Build.VERSION.RELEASE ?: "unknown"
        values["uptime"] = (SystemClock.elapsedRealtime() / 1000).toString()
    }

    private fun readNetwork(values: MutableMap<String, String>) {
        val connectivityManager = context.getSystemService(ConnectivityManager::class.java)
            ?: return
        val capabilities = connectivityManager.activeNetwork
            ?.let { connectivityManager.getNetworkCapabilities(it) }

        values["network_transport"] = when {
            capabilities == null -> "none"
            capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
            capabilities.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "cellular"
            capabilities.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "ethernet"
            capabilities.hasTransport(NetworkCapabilities.TRANSPORT_VPN) -> "vpn"
            else -> "none"
        }
    }

    private fun readStorage(values: MutableMap<String, String>) {
        runCatching {
            val statFs = StatFs(Environment.getDataDirectory().path)
            values["storage_free"] = statFs.availableBytes.toString()
        }
    }

    private fun readAudio(values: MutableMap<String, String>) {
        val audioManager = context.getSystemService(AudioManager::class.java) ?: return
        values["ringer_mode"] = when (audioManager.ringerMode) {
            AudioManager.RINGER_MODE_SILENT -> "silent"
            AudioManager.RINGER_MODE_VIBRATE -> "vibrate"
            else -> "normal"
        }
    }

    @Suppress("InlinedApi", "DEPRECATION")
    private fun readPermissionGated(values: MutableMap<String, String>) {
        val wifiGranted = hasPermission(Manifest.permission.ACCESS_FINE_LOCATION) ||
            hasPermission(Manifest.permission.NEARBY_WIFI_DEVICES)
        if (wifiGranted) {
            val wifiManager = context.applicationContext.getSystemService(WifiManager::class.java)
            val ssid = wifiManager?.connectionInfo?.ssid?.trim('"')
            if (!ssid.isNullOrBlank() && ssid != "<unknown ssid>") {
                values["wifi_ssid"] = ssid
            }
        }

        if (hasPermission(Manifest.permission.READ_PHONE_STATE)) {
            val telephonyManager = context.getSystemService(TelephonyManager::class.java)
            val carrier = telephonyManager?.networkOperatorName
            if (!carrier.isNullOrBlank()) {
                values["carrier"] = carrier
            }
        }
    }

    private fun hasPermission(permission: String): Boolean =
        ContextCompat.checkSelfPermission(context, permission) == PackageManager.PERMISSION_GRANTED

    private companion object {
        const val BATTERY_PLUGGED_DOCK = 8 // BatteryManager.BATTERY_PLUGGED_DOCK, API 33
    }
}
