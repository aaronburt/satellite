package uk.co.aaronburt.satellite.update

import org.json.JSONObject

data class ReleaseAsset(
    val name: String,
    val downloadUrl: String,
    val size: Long,
    /** SHA-256 of the asset as reported by GitHub, when available. */
    val sha256: String?
)

data class GitHubRelease(
    val tag: String,
    val versionCode: Int?,
    val versionName: String,
    val title: String,
    val notes: String?,
    val htmlUrl: String,
    val apk: ReleaseAsset?
)

/**
 * Parses a GitHub `releases/latest` payload. Kept pure so it can be unit tested
 * without touching the network.
 */
object GitHubReleaseParser {

    fun parse(body: String): GitHubRelease {
        val json = JSONObject(body)
        val tag = json.optString("tag_name")

        val assets = json.optJSONArray("assets")
        var apk: ReleaseAsset? = null
        if (assets != null) {
            for (index in 0 until assets.length()) {
                val asset = assets.getJSONObject(index)
                val name = asset.optString("name")
                if (!name.endsWith(".apk", ignoreCase = true)) continue
                apk = ReleaseAsset(
                    name = name,
                    downloadUrl = asset.optString("browser_download_url"),
                    size = asset.optLong("size"),
                    sha256 = asset.optString("digest")
                        .takeIf { it.startsWith(SHA256_PREFIX) }
                        ?.removePrefix(SHA256_PREFIX),
                )
                break
            }
        }

        return GitHubRelease(
            tag = tag,
            versionCode = VersionCodes.fromTag(tag),
            versionName = tag.removePrefix("v"),
            title = json.optString("name").ifBlank { tag },
            notes = json.optString("body").takeIf { it.isNotBlank() },
            htmlUrl = json.optString("html_url"),
            apk = apk,
        )
    }

    private const val SHA256_PREFIX = "sha256:"
}

/**
 * Turns a release tag into a `versionCode`, using the same encoding as the
 * release workflow so the installed code can be compared with a tag.
 *
 * Releases are `vMAJOR.MINOR` (`v0.28` -> `2800`, `v1.0` -> `1000000`). Older
 * releases were `vMAJOR.MINOR.PATCH` (`v0.27.3` -> `2703`); they still parse,
 * and every 0.x code is unchanged, so the codes keep increasing across the
 * switch. Minor numbers can go up to 9999.
 */
object VersionCodes {

    private const val MAJOR_STEP = 1_000_000
    private const val MINOR_STEP = 100
    private const val MAX_MAJOR = 2_099

    /** Major, minor up to 9999, then an optional patch up to 99 and any extra parts. */
    private val TAG = Regex("""v?(\d{1,4})\.(\d{1,4})(?:\.(\d{1,2})(?:\.\d+)*)?""")

    fun fromTag(tag: String): Int? {
        val match = TAG.matchEntire(tag.trim()) ?: return null
        val (majorText, minorText, patchText) = match.destructured
        val major = majorText.toInt()
        if (major > MAX_MAJOR) return null
        val patch = patchText.ifEmpty { "0" }.toInt()
        return major * MAJOR_STEP + minorText.toInt() * MINOR_STEP + patch
    }
}
