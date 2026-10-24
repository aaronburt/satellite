package uk.co.aaronburt.satellite.reporter

import android.Manifest
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager
import android.telephony.TelephonyManager
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.shadows.ShadowWifiInfo
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.setBattery

/**
 * Every value the telemetry reader produces must have an entity declared in
 * the catalog; the reporters drop anything undeclared, so a missing entry
 * means a sensor that silently never reaches Home Assistant.
 */
@RunWith(RobolectricTestRunner::class)
class TelemetryContractTest {

    private val context = RuntimeEnvironment.getApplication()

    @Test
    fun `every telemetry key is a declared entity`() {
        context.setBattery(level = 50)
        shadowOf(context).grantPermissions(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.READ_PHONE_STATE,
        )
        val wifiInfo: WifiInfo = ShadowWifiInfo.newInstance()
        shadowOf(wifiInfo).setSSID("HomeNet")
        shadowOf(context.getSystemService(WifiManager::class.java)).setConnectionInfo(wifiInfo)
        shadowOf(context.getSystemService(TelephonyManager::class.java)).setNetworkOperatorName("Carrier")

        val keys = DeviceStateReader(context, AudioControls(context)).read().keys
        val declared = EntityCatalog.all.map { it.key }.toSet()

        assertTrue("expected a full reading, got $keys", keys.containsAll(listOf("battery", "wifi_ssid", "carrier")))
        assertTrue("undeclared: ${keys - declared}", declared.containsAll(keys))
    }

    @Test
    fun `entity keys are unique and safe to use as topic segments`() {
        val keys = EntityCatalog.all.map { it.key }
        assertTrue("duplicates in $keys", keys.size == keys.toSet().size)
        keys.forEach { key ->
            assertTrue("'$key' is not a plain topic segment", Regex("[a-z0-9_]+").matches(key))
        }
    }
}
