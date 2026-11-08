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
    fun `parses a two-part major-minor release tag`() {
        val release = GitHubReleaseParser.parse("""{"tag_name":"v0.28","assets":[]}""")

        assertEquals("v0.28", release.tag)
        assertEquals("0.28", release.versionName)
        assertEquals(2_800, release.versionCode)
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

    @Test
    fun `selects the highest Android version with an APK and skips other platforms`() {
        val releases = """[
            {"tag_name":"desktop-v9.99","assets":[{"name":"wrong.apk"}]},
            {"tag_name":"android-v8.99","draft":true,"assets":[{"name":"draft.apk"}]},
            {"tag_name":"android-v7.99","prerelease":true,"assets":[{"name":"preview.apk"}]},
            {"tag_name":"android-v6.99","assets":[]},
            {"tag_name":"v0.47","assets":[{"name":"legacy.apk"}]},
            {"tag_name":"android-v0.49","assets":[{"name":"current.apk"}]},
            {"tag_name":"android-v0.48","assets":[{"name":"older.apk"}]}
        ]"""
        val release = GitHubReleaseParser.latestAndroid(releases)
        assertEquals("android-v0.49", release?.tag)
        assertEquals("0.49", release?.versionName)
        assertEquals(4900, release?.versionCode)
        assertEquals("current.apk", release?.apk?.name)
    }

    @Test
    fun `falls back to legacy combined releases and keeps numeric version codes`() {
        val release = GitHubReleaseParser.latestAndroid("""[{"tag_name":"v0.47","assets":[{"name":"legacy.apk"}]}]""")
        assertEquals("v0.47", release?.tag)
        assertEquals(4700, release?.versionCode)
        assertNull(GitHubReleaseParser.latestAndroid("""[{"tag_name":"desktop-v0.99","assets":[]}]"""))
        assertEquals(1_000_100, VersionCodes.fromTag("android-v1.1"))
        assertNull(VersionCodes.fromTag("desktop-v1.1"))
    }
}
