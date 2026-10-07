package uk.co.aaronburt.satellite.update

import org.json.JSONObject

data class ReleaseAsset(
    val name: String,
    val downloadUrl: String,
    val size: Long,
    /** SHA-256 of the asset as reported by GitHub, when available. */
    val sha256: String?,
)

data class GitHubRelease(
    val tag: String,
    val versionCode: Int?,
    val versionName: String,
    val title: String,
    val notes: String?,
    val htmlUrl: String,
    val apk: ReleaseAsset?,
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
 * `v1.2.3` -> `10203`. Mirrors the scheme the release workflow uses, so the
 * installed `versionCode` can be compared directly with a release tag.
 */
object VersionCodes {

    fun fromTag(tag: String): Int? {
        val parts = tag.trim().removePrefix("v").split(".")
        if (parts.size < 3) return null

        val major = parts[0].toIntOrNull() ?: return null
        val minor = parts[1].toIntOrNull() ?: return null
        val patch = parts[2].toIntOrNull() ?: return null
        return major * 10_000 + minor * 100 + patch
    }
}
