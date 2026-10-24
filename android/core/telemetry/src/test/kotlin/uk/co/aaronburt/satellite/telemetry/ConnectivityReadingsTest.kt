package uk.co.aaronburt.satellite.telemetry

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import uk.co.aaronburt.satellite.telemetry.ConnectivityReadings.DISCONNECTED
import uk.co.aaronburt.satellite.telemetry.ConnectivityReadings.PERMISSION_DENIED
import uk.co.aaronburt.satellite.telemetry.ConnectivityReadings.UNAVAILABLE

class ConnectivityReadingsTest {

    @Test
    fun `off Wi-Fi is disconnected even when a stale SSID is still reported`() {
        assertEquals(DISCONNECTED, ConnectivityReadings.wifiSsid(onWifi = false, permitted = true, rawSsid = "\"Old\""))
        assertEquals(DISCONNECTED, ConnectivityReadings.wifiSsid(onWifi = false, permitted = false, rawSsid = null))
    }

    @Test
    fun `on Wi-Fi without permission is denied, not disconnected`() {
        assertEquals(
            PERMISSION_DENIED,
            ConnectivityReadings.wifiSsid(onWifi = true, permitted = false, rawSsid = "\"Home\"")
        )
    }

    @Test
    fun `on Wi-Fi with permission reports the name or unavailable when redacted`() {
        assertEquals("Home", ConnectivityReadings.wifiSsid(true, true, "\"Home\""))
        assertEquals("Home Net", ConnectivityReadings.wifiSsid(true, true, " \"Home Net\" "))
        assertEquals(UNAVAILABLE, ConnectivityReadings.wifiSsid(true, true, "<unknown ssid>"))
        assertEquals(UNAVAILABLE, ConnectivityReadings.wifiSsid(true, true, "\"\""))
        assertEquals(UNAVAILABLE, ConnectivityReadings.wifiSsid(true, true, null))
    }

    @Test
    fun `cleanSsid keeps unquoted names and inner quotes`() {
        assertEquals("Cafe", ConnectivityReadings.cleanSsid("Cafe"))
        assertEquals("Bob's \"5G\"", ConnectivityReadings.cleanSsid("\"Bob's \"5G\"\""))
        assertNull(ConnectivityReadings.cleanSsid("   "))
        assertEquals("Home", ConnectivityReadings.cleanSsid("\"\"Home\"\""))
        assertNull(ConnectivityReadings.cleanSsid("\"\"\"\""))
    }

    @Test
    fun `carrier distinguishes denied, unavailable and a real name`() {
        assertEquals(PERMISSION_DENIED, ConnectivityReadings.carrier(permitted = false, operatorName = "Vodafone"))
        assertEquals(UNAVAILABLE, ConnectivityReadings.carrier(permitted = true, operatorName = ""))
        assertEquals(UNAVAILABLE, ConnectivityReadings.carrier(permitted = true, operatorName = null))
        assertEquals("Vodafone", ConnectivityReadings.carrier(permitted = true, operatorName = "Vodafone"))
    }

    @Test
    fun `placeholders are recognised so the UI does not show them as names`() {
        listOf(DISCONNECTED, PERMISSION_DENIED, UNAVAILABLE, null).forEach {
            assertTrue("$it", ConnectivityReadings.isPlaceholder(it))
        }
        assertFalse(ConnectivityReadings.isPlaceholder("HomeNet"))
    }
}
