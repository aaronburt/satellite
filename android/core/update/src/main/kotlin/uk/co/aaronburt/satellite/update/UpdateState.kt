package uk.co.aaronburt.satellite.update

/** Where the update flow currently is, for the UI and the Home Assistant entity. */
sealed interface UpdateState {

    data object Idle : UpdateState

    data object Checking : UpdateState

    data class UpToDate(val installedVersion: String) : UpdateState

    data class Downloading(val versionName: String, val progress: Int) : UpdateState

    /** Downloaded and verified; waiting for the user to confirm the install. */
    data class Ready(
        val versionName: String,
        val versionCode: Int,
        val notes: String?,
        val releaseUrl: String,
    ) : UpdateState

    data class Failed(val message: String) : UpdateState
}
