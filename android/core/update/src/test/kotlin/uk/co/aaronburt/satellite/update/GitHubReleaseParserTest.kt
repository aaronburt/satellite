package uk.co.aaronburt.satellite.update

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class GitHubReleaseParserTest {

    private val json = """
        {
          "tag_name": "v0.4.0",
          "name": "v0.4.0",
          "body": "Adds the update checker.",
          "html_url": "https://github.com/aaronburt/satellite/releases/tag/v0.4.0",
          "assets": [
            {
              "name": "satellite-v0.4.0.apk",
              "browser_download_url": "https://github.com/aaronburt/satellite/releases/download/v0.4.0/satellite-v0.4.0.apk",
              "size": 1941504,
              "digest": "sha256:abc123"
            },
            {
              "name": "checksums.txt",
              "browser_download_url": "https://example.invalid/checksums.txt",
              "size": 42
            }
          ]
        }
    """.trimIndent()

    @Test
    fun `parses tag, notes and version code`() {
        val release = GitHubReleaseParser.parse(json)

        assertEquals("v0.4.0", release.tag)
        assertEquals("0.4.0", release.versionName)
        assertEquals(400, release.versionCode)
        assertEquals("Adds the update checker.", release.notes)
    }

    @Test
    fun `picks the apk asset and reads its digest`() {
        val apk = GitHubReleaseParser.parse(json).apk

        assertEquals("satellite-v0.4.0.apk", apk?.name)
        assertEquals(1_941_504L, apk?.size)
        assertEquals("abc123", apk?.sha256)
        assertTrue(apk!!.downloadUrl.startsWith("https://github.com/"))
    }

    @Test
    fun `handles a release with no apk`() {
        val release = GitHubReleaseParser.parse(
            """{"tag_name":"v9.9.9","assets":[]}""",
        )
        assertNull(release.apk)
    }
}
