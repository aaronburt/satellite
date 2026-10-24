package uk.co.aaronburt.satellite.update

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.flow.map
import org.json.JSONObject

/** Supplies the update-related entity states to the reporters. */
interface UpdateStatesSource {

    /** Entity key → payload, for the keys in [UpdateEntityPayload]. */
    fun read(): Map<String, String>

    /** Emits each time [read] would return something worth republishing. */
    val changes: Flow<Unit>
}

/** Reports nothing; for reporters that have no update flow behind them. */
object NoUpdateStates : UpdateStatesSource {
    override fun read(): Map<String, String> = emptyMap()
    override val changes: Flow<Unit> = emptyFlow()
}

@Singleton
class UpdateEntityStates @Inject constructor(private val manager: UpdateManager) : UpdateStatesSource {

    override fun read(): Map<String, String> =
        UpdateEntityPayload.states(manager.installedVersionName, manager.state.value)

    // Download progress is reported in coarse steps so a 0-100 download is a
    // handful of publishes rather than a hundred.
    override val changes: Flow<Unit> = manager.state
        .map { UpdateEntityPayload.states(manager.installedVersionName, it.coarse()) }
        .distinctUntilChanged()
        .map { }

    private fun UpdateState.coarse(): UpdateState = when (this) {
        is UpdateState.Downloading -> copy(progress = progress / PROGRESS_STEP * PROGRESS_STEP)
        else -> this
    }

    private companion object {
        const val PROGRESS_STEP = 10
    }
}

/**
 * Maps [UpdateState] onto the two Home Assistant entities that describe it.
 *
 * `app_update` is an MQTT `update` entity whose state topic carries JSON.
 * `update_status` is an enum sensor that spells out where the flow is, which the
 * update entity cannot (it has no "check failed" state). See
 * `android/docs/update-entity.md`.
 *
 * `latest_version` is only sent when a check has actually produced one, so the
 * entity never claims to be up to date when it does not know.
 */
object UpdateEntityPayload {

    const val UPDATE_KEY = "app_update"
    const val STATUS_KEY = "update_status"

    const val STATUS_IDLE = "idle"
    const val STATUS_CHECKING = "checking"
    const val STATUS_UP_TO_DATE = "up_to_date"
    const val STATUS_DOWNLOADING = "downloading"
    const val STATUS_READY = "ready"
    const val STATUS_FAILED = "failed"

    val STATUSES = listOf(
        STATUS_IDLE,
        STATUS_CHECKING,
        STATUS_UP_TO_DATE,
        STATUS_DOWNLOADING,
        STATUS_READY,
        STATUS_FAILED,
    )

    /** Home Assistant truncates `release_summary` at 255 characters. */
    const val SUMMARY_LIMIT = 255

    private const val MAX_PERCENT = 100

    fun states(installedVersion: String, state: UpdateState): Map<String, String> = mapOf(
        UPDATE_KEY to json(installedVersion, state),
        STATUS_KEY to status(state),
    )

    fun status(state: UpdateState): String = when (state) {
        UpdateState.Idle -> STATUS_IDLE
        UpdateState.Checking -> STATUS_CHECKING
        is UpdateState.UpToDate -> STATUS_UP_TO_DATE
        is UpdateState.Downloading -> STATUS_DOWNLOADING
        is UpdateState.Ready -> STATUS_READY
        is UpdateState.Failed -> STATUS_FAILED
    }

    fun json(installedVersion: String, state: UpdateState): String {
        val json = JSONObject().put("installed_version", installedVersion)
        when (state) {
            UpdateState.Idle, UpdateState.Checking -> Unit

            is UpdateState.UpToDate -> json.put("latest_version", state.installedVersion)

            is UpdateState.Downloading -> {
                json.put("latest_version", state.versionName)
                    .put("in_progress", true)
                    .put("update_percentage", state.progress.coerceIn(0, MAX_PERCENT))
            }

            is UpdateState.Ready -> {
                json.put("latest_version", state.versionName)
                    .put("title", "Satellite ${state.versionName}")
                    .put("release_url", state.releaseUrl)
                state.notes?.takeIf { it.isNotBlank() }?.let {
                    json.put("release_summary", it.trim().take(SUMMARY_LIMIT))
                }
            }

            is UpdateState.Failed -> json.put(
                "release_summary",
                "Update check failed: ${state.message}".take(SUMMARY_LIMIT),
            )
        }
        return json.toString()
    }
}
