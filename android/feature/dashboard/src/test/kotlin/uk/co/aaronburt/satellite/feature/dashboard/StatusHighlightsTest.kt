package uk.co.aaronburt.satellite.feature.dashboard

import org.junit.Assert.assertEquals
import org.junit.Test

class StatusHighlightsTest {

    @Test
    fun `battery, charging and Wi-Fi name when known`() {
        val highlights = statusHighlights(
            mapOf("battery" to "81", "charging" to "ON", "wifi_ssid" to "HomeNet", "network_transport" to "wifi"),
        )

        assertEquals(
            listOf(
                StatusHighlight("battery", "Battery", "81%"),
                StatusHighlight("charging", "Charging", "On"),
                StatusHighlight("wifi_ssid", "Wi-Fi", "HomeNet"),
            ),
            highlights,
        )
    }

    @Test
    fun `falls back to the network type without a Wi-Fi name`() {
        val highlights = statusHighlights(mapOf("charging" to "OFF", "network_transport" to "cellular"))

        assertEquals(
            listOf(
                StatusHighlight("charging", "Charging", "Off"),
                StatusHighlight("network_transport", "Network", "Cellular"),
            ),
            highlights,
        )
    }

    @Test
    fun `nothing known, nothing shown`() {
        assertEquals(emptyList<StatusHighlight>(), statusHighlights(emptyMap()))
    }

    @Test
    fun `a Wi-Fi placeholder falls back to the network type`() {
        listOf("disconnected", "permission_denied", "unavailable").forEach { placeholder ->
            val highlights = statusHighlights(
                mapOf("wifi_ssid" to placeholder, "network_transport" to "cellular"),
            )
            assertEquals(placeholder, listOf(StatusHighlight("network_transport", "Network", "Cellular")), highlights)
        }
    }
}
