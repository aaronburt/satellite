package uk.co.aaronburt.satellite.reporter

import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ReporterStatusTest {

    @Test
    fun `starts empty, records the publish time, and resets`() {
        val status = ReporterStatus()
        assertNull(status.lastPublishedAtMillis.value)

        val before = System.currentTimeMillis()
        status.markPublished()
        val at = status.lastPublishedAtMillis.value!!
        assertTrue(at in before..System.currentTimeMillis())

        status.reset()
        assertNull(status.lastPublishedAtMillis.value)
    }
}
