package uk.co.aaronburt.satellite.reporter

import uk.co.aaronburt.satellite.discovery.Topics
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class CommandParserTest {

    private val topics = Topics("abcd1234")

    @Test
    fun `parses an in-range volume`() {
        assertEquals(
            SatelliteCommand.SetVolume(70),
            CommandParser.parse(topics.command("volume"), "70", topics),
        )
    }

    @Test
    fun `rejects volume outside 0-100`() {
        assertNull(CommandParser.parse(topics.command("volume"), "101", topics))
        assertNull(CommandParser.parse(topics.command("volume"), "-1", topics))
    }

    @Test
    fun `rejects non-numeric volume`() {
        assertNull(CommandParser.parse(topics.command("volume"), "loud", topics))
    }

    @Test
    fun `parses mute payloads case-insensitively`() {
        assertEquals(
            SatelliteCommand.SetMicrophoneMuted(true),
            CommandParser.parse(topics.command("mute"), "ON", topics),
        )
        assertEquals(
            SatelliteCommand.SetMicrophoneMuted(false),
            CommandParser.parse(topics.command("mute"), "off", topics),
        )
    }

    @Test
    fun `rejects unknown actions`() {
        assertNull(CommandParser.parse(topics.command("reboot"), "ON", topics))
    }

    @Test
    fun `rejects commands for another device`() {
        val other = Topics("99999999")
        assertNull(CommandParser.parse(other.command("volume"), "70", topics))
    }
}
