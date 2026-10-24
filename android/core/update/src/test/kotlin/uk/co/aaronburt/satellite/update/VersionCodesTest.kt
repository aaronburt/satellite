package uk.co.aaronburt.satellite.update

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class VersionCodesTest {

    @Test
    fun `encodes a semver tag the same way the release workflow does`() {
        assertEquals(10_203, VersionCodes.fromTag("v1.2.3"))
        assertEquals(300, VersionCodes.fromTag("v0.3.0"))
        assertEquals(400, VersionCodes.fromTag("0.4.0"))
        assertEquals(10_203, VersionCodes.fromTag("  v1.2.3 "))
    }

    @Test
    fun `newer versions always get bigger codes within the scheme`() {
        val ordered = listOf("v0.0.1", "v0.0.99", "v0.1.0", "v0.99.99", "v1.0.0", "v2.3.4")
        val codes = ordered.map { VersionCodes.fromTag(it)!! }
        assertTrue(codes.toString(), codes.zipWithNext().all { (a, b) -> a < b })
    }

    @Test
    fun `tags that are not plain three-part versions have no code`() {
        for (tag in listOf("", "v1", "v1.2", "nightly", "v1.x.3", "v1.2.3-beta", "vv1.2.3")) {
            assertNull(tag, VersionCodes.fromTag(tag))
        }
    }

    @Test
    fun `extra parts after the patch are ignored`() {
        assertEquals(10203, VersionCodes.fromTag("v1.2.3.4"))
    }
}
