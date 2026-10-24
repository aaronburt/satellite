package uk.co.aaronburt.satellite.update

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class UpdateEntityPayloadTest {

    private fun json(state: UpdateState) = JSONObject(UpdateEntityPayload.json("0.5.1", state))

    @Test
    fun `before any check only the installed version is known`() {
        listOf(UpdateState.Idle, UpdateState.Checking).forEach { state ->
            val json = json(state)
            assertEquals("0.5.1", json.getString("installed_version"))
            assertFalse("$state must not claim a latest version", json.has("latest_version"))
            assertFalse(json.has("in_progress"))
        }
    }

    @Test
    fun `up to date reports the installed version as the latest`() {
        val json = json(UpdateState.UpToDate("0.5.1"))
        assertEquals("0.5.1", json.getString("latest_version"))
    }

    @Test
    fun `downloading reports progress`() {
        val json = json(UpdateState.Downloading("0.6.0", 40))
        assertEquals("0.6.0", json.getString("latest_version"))
        assertTrue(json.getBoolean("in_progress"))
        assertEquals(40, json.getInt("update_percentage"))
    }

    @Test
    fun `ready reports the release but never an install in progress`() {
        val json = json(UpdateState.Ready("0.6.0", 600, "Fixes", "https://example.test/r"))
        assertEquals("0.6.0", json.getString("latest_version"))
        assertEquals("Satellite 0.6.0", json.getString("title"))
        assertEquals("https://example.test/r", json.getString("release_url"))
        assertEquals("Fixes", json.getString("release_summary"))
        assertFalse(json.has("in_progress"))
    }

    @Test
    fun `long release notes are cut to what Home Assistant accepts`() {
        val json = json(UpdateState.Ready("0.6.0", 600, "x".repeat(1_000), "https://example.test/r"))
        assertEquals(UpdateEntityPayload.SUMMARY_LIMIT, json.getString("release_summary").length)
    }

    @Test
    fun `missing or blank notes leave out the summary`() {
        assertFalse(json(UpdateState.Ready("0.6.0", 600, null, "u")).has("release_summary"))
        assertFalse(json(UpdateState.Ready("0.6.0", 600, "  \n", "u")).has("release_summary"))
    }

    @Test
    fun `a failed check keeps no latest version and explains itself`() {
        val json = json(UpdateState.Failed("rate limited"))
        assertFalse(json.has("latest_version"))
        assertEquals("Update check failed: rate limited", json.getString("release_summary"))
    }

    @Test
    fun `every state maps to a status the catalog declares`() {
        val statuses = listOf(
            UpdateState.Idle,
            UpdateState.Checking,
            UpdateState.UpToDate("1"),
            UpdateState.Downloading("2", 0),
            UpdateState.Ready("2", 2, null, "u"),
            UpdateState.Failed("x"),
        ).map(UpdateEntityPayload::status)

        assertEquals(UpdateEntityPayload.STATUSES, statuses)
    }

    @Test
    fun `states are keyed by the two entity keys`() {
        val states = UpdateEntityPayload.states("0.5.1", UpdateState.Idle)
        assertEquals(setOf("app_update", "update_status"), states.keys)
        assertEquals("idle", states.getValue("update_status"))
    }
}
