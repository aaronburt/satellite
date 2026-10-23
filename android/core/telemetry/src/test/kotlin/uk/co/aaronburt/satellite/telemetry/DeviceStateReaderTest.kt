package uk.co.aaronburt.satellite.telemetry

import android.Manifest
import android.media.AudioManager
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.wifi.WifiManager
import android.os.BatteryManager
import android.os.Build
import android.os.Environment
import android.os.PowerManager
import android.telephony.TelephonyManager
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.shadows.ShadowNetworkCapabilities
import org.robolectric.shadows.ShadowStatFs
import org.robolectric.shadows.ShadowWifiInfo
import uk.co.aaronburt.satellite.testing.setBattery

@RunWith(RobolectricTestRunner::class)
class DeviceStateReaderTest {

    private val context = RuntimeEnvironment.getApplication()
    private val reader = DeviceStateReader(context, AudioControls(context))

    private fun read() = reader.read()

    @Test
    fun `battery level is a percentage of the scale`() {
        context.setBattery(level = 50, scale = 200)
        assertEquals("25", read()["battery"])

        context.setBattery(level = 1, scale = 3)
        assertEquals("33", read()["battery"])
    }

    @Test
    fun `an invalid battery reading is left out`() {
        context.setBattery(level = -1)
        assertNull(read()["battery"])

        context.setBattery(level = 10, scale = 0)
        assertNull(read()["battery"])
    }

    @Test
    fun `no battery broadcast means no battery values`() {
        val values = read()
        assertFalse(values.containsKey("battery"))
        assertFalse(values.containsKey("charging"))
    }

    @Test
    fun `charger type follows what is plugged in`() {
        val cases = mapOf(
            BatteryManager.BATTERY_PLUGGED_AC to "ac",
            BatteryManager.BATTERY_PLUGGED_USB to "usb",
            BatteryManager.BATTERY_PLUGGED_WIRELESS to "wireless",
            8 to "dock",
            0 to "none",
        )
        cases.forEach { (plugged, expected) ->
            context.setBattery(level = 50, plugged = plugged)
            assertEquals("plugged=$plugged", expected, read()["charger_type"])
        }
    }

    @Test
    fun `charging is on when plugged in, charging, or full`() {
        context.setBattery(level = 50, plugged = BatteryManager.BATTERY_PLUGGED_USB)
        assertEquals("ON", read()["charging"])

        context.setBattery(level = 50, status = BatteryManager.BATTERY_STATUS_CHARGING)
        assertEquals("ON", read()["charging"])

        context.setBattery(level = 100, status = BatteryManager.BATTERY_STATUS_FULL)
        assertEquals("ON", read()["charging"])

        context.setBattery(level = 50, status = BatteryManager.BATTERY_STATUS_DISCHARGING)
        assertEquals("OFF", read()["charging"])
    }

    @Test
    fun `battery temperature is reported in degrees`() {
        context.setBattery(level = 50, temperatureTenths = 253)
        assertEquals("25.3", read()["battery_temperature"])

        context.setBattery(level = 50, temperatureTenths = null)
        assertNull(read()["battery_temperature"])
    }

    @Test
    fun `power state flags`() {
        val power = shadowOf(context.getSystemService(PowerManager::class.java))
        power.setIsPowerSaveMode(true)
        power.setIsInteractive(false)
        power.setIsDeviceIdleMode(true)

        val values = read()
        assertEquals("ON", values["power_save"])
        assertEquals("OFF", values["interactive"])
        assertEquals("ON", values["doze"])

        power.setIsPowerSaveMode(false)
        power.setIsInteractive(true)
        power.setIsDeviceIdleMode(false)
        val after = read()
        assertEquals("OFF", after["power_save"])
        assertEquals("ON", after["interactive"])
        assertEquals("OFF", after["doze"])
    }

    @Test
    fun `network transport reflects the active network`() {
        val connectivity = context.getSystemService(ConnectivityManager::class.java)
        val network = connectivity.activeNetwork!!
        val cases = mapOf(
            NetworkCapabilities.TRANSPORT_WIFI to "wifi",
            NetworkCapabilities.TRANSPORT_CELLULAR to "cellular",
            NetworkCapabilities.TRANSPORT_ETHERNET to "ethernet",
            NetworkCapabilities.TRANSPORT_VPN to "vpn",
            NetworkCapabilities.TRANSPORT_BLUETOOTH to "none",
        )
        cases.forEach { (transport, expected) ->
            val capabilities = ShadowNetworkCapabilities.newInstance()
            shadowOf(capabilities).addTransportType(transport)
            shadowOf(connectivity).setNetworkCapabilities(network, capabilities)
            assertEquals("transport=$transport", expected, read()["network_transport"])
        }
    }

    @Test
    fun `no active network reads as none`() {
        val connectivity = context.getSystemService(ConnectivityManager::class.java)
        shadowOf(connectivity).setActiveNetworkInfo(null)

        assertEquals("none", read()["network_transport"])
    }

    @Test
    fun `ringer mode`() {
        val audio = context.getSystemService(AudioManager::class.java)
        mapOf(
            AudioManager.RINGER_MODE_SILENT to "silent",
            AudioManager.RINGER_MODE_VIBRATE to "vibrate",
            AudioManager.RINGER_MODE_NORMAL to "normal",
        ).forEach { (mode, expected) ->
            audio.ringerMode = mode
            assertEquals(expected, read()["ringer_mode"])
        }
    }

    @Test
    fun `audio controls are read through`() {
        val audio = context.getSystemService(AudioManager::class.java)
        shadowOf(audio).setStreamMaxVolume(10)
        audio.setStreamVolume(AudioManager.STREAM_MUSIC, 4, 0)
        audio.isMicrophoneMute = true

        val values = read()
        assertEquals("40", values["volume_media"])
        assertEquals("ON", values["mic_muted"])
    }

    @Test
    fun `app, system and storage values`() {
        ShadowStatFs.registerStats(Environment.getDataDirectory(), 1_000, 500, 250)

        val values = read()
        assertEquals(Build.VERSION.RELEASE, values["android_version"])
        assertTrue(values["app_version"]!!.isNotBlank())
        assertTrue(values["uptime"]!!.toLong() >= 0)
        assertEquals((250L * ShadowStatFs.BLOCK_SIZE).toString(), values["storage_free"])
    }

    @Test
    fun `Wi-Fi name and carrier need their permissions`() {
        setWifi("\"HomeNet\"")
        setCarrier("Carrier")

        val values = read()
        assertFalse(values.containsKey("wifi_ssid"))
        assertFalse(values.containsKey("carrier"))
    }

    @Test
    fun `Wi-Fi name is read with location permission, without the quotes`() {
        shadowOf(context).grantPermissions(Manifest.permission.ACCESS_FINE_LOCATION)
        setWifi("\"HomeNet\"")

        assertEquals("HomeNet", read()["wifi_ssid"])
    }

    @Test
    fun `Wi-Fi name is read with the nearby devices permission`() {
        shadowOf(context).grantPermissions(Manifest.permission.NEARBY_WIFI_DEVICES)
        setWifi("\"Cafe\"")

        assertEquals("Cafe", read()["wifi_ssid"])
    }

    @Test
    fun `an unknown or empty Wi-Fi name is left out`() {
        shadowOf(context).grantPermissions(Manifest.permission.ACCESS_FINE_LOCATION)

        setWifi("<unknown ssid>")
        assertNull(read()["wifi_ssid"])

        setWifi("\"\"")
        assertNull(read()["wifi_ssid"])
    }

    @Test
    fun `carrier is read with the phone state permission`() {
        shadowOf(context).grantPermissions(Manifest.permission.READ_PHONE_STATE)
        setCarrier("Vodafone UK")
        assertEquals("Vodafone UK", read()["carrier"])

        setCarrier("")
        assertNull(read()["carrier"])
    }

    private fun setWifi(ssid: String) {
        val info = ShadowWifiInfo.newInstance()
        shadowOf(info).setSSID(ssid)
        shadowOf(context.getSystemService(WifiManager::class.java)).setConnectionInfo(info)
    }

    private fun setCarrier(name: String) {
        shadowOf(context.getSystemService(TelephonyManager::class.java)).setNetworkOperatorName(name)
    }
}
