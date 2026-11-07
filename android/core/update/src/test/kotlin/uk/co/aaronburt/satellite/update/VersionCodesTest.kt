package uk.co.aaronburt.satellite.update

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class VersionCodesTest {

    @Test
    fun `encodes a major-minor tag the same way the release workflow does`() {
        assertEquals(2_800, VersionCodes.fromTag("v0.28"))
        assertEquals(2_900, VersionCodes.fromTag("0.29"))
        assertEquals(10_000, VersionCodes.fromTag("v0.100"))
        assertEquals(1_000_000, VersionCodes.fromTag("v1.0"))
        assertEquals(2_000_300, VersionCodes.fromTag("  v2.3 "))
    }

    @Test
    fun `old three-part tags keep the codes they were released with`() {
        assertEquals(2_703, VersionCodes.fromTag("v0.27.3"))
        assertEquals(300, VersionCodes.fromTag("v0.3.0"))
        assertEquals(2_601, VersionCodes.fromTag("v0.26.1"))
    }

    @Test
    fun `the first two-part release is newer than the last three-part one`() {
        assertTrue(VersionCodes.fromTag("v0.28")!! > VersionCodes.fromTag("v0.27.3")!!)
    }

    @Test
    fun `newer versions always get bigger codes`() {
        val ordered = listOf(
            "v0.0.1", "v0.1.0", "v0.27.3", "v0.28", "v0.29", "v0.99", "v0.100",
            "v0.9999", "v1.0", "v1.1", "v2.0",
        )
        val codes = ordered.map { VersionCodes.fromTag(it)!! }
        assertTrue(codes.toString(), codes.zipWithNext().all { (a, b) -> a < b })
    }

    @Test
    fun `tags that are not plain versions have no code`() {
        val bad = listOf(
            "", "v1", "nightly", "v1.x", "v1.x.3", "v1.2-beta", "v1.2.3-beta", "vv1.2",
            "v-1.2", "v0.10000", "v0.1.100", "v9999.0",
        )
        for (tag in bad) {
            assertNull(tag, VersionCodes.fromTag(tag))
        }
    }

    @Test
    fun `extra parts after the patch are ignored`() {
        assertEquals(2_703, VersionCodes.fromTag("v0.27.3.4"))
    }
}
