package dev.satelliteandroid.app

import android.Manifest
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.rule.GrantPermissionRule
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Drives the real settings screen and connects to the Docker Mosquitto broker.
 *
 * Requires the local test environment to be running (`docker compose up -d`) and
 * uses the emulator host alias `10.0.2.2`. This is an integration test, not a
 * hermetic unit test.
 */
@RunWith(AndroidJUnit4::class)
class BrokerConfigurationE2ETest {

    @get:Rule(order = 0)
    val notificationPermissionRule: GrantPermissionRule =
        GrantPermissionRule.grant(Manifest.permission.POST_NOTIFICATIONS)

    @get:Rule(order = 1)
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Test
    fun configuresBrokerAndConnects() {
        composeRule.onNodeWithTag("dashboard_configure").performClick()

        composeRule.onNodeWithTag("settings_host").performTextClearance()
        composeRule.onNodeWithTag("settings_host").performTextInput("10.0.2.2")
        composeRule.onNodeWithTag("settings_port").performTextClearance()
        composeRule.onNodeWithTag("settings_port").performTextInput("1883")
        composeRule.onNodeWithTag("settings_username").performTextClearance()
        composeRule.onNodeWithTag("settings_username").performTextInput("satellite")
        composeRule.onNodeWithTag("settings_password").performTextClearance()
        composeRule.onNodeWithTag("settings_password").performTextInput("satellite")

        composeRule.onNodeWithTag("settings_save").performClick()
        composeRule.onNodeWithText("Back").performClick()

        composeRule.waitUntil(timeoutMillis = 20_000) {
            composeRule
                .onAllNodesWithText("Connected to tcp://10.0.2.2:1883")
                .fetchSemanticsNodes()
                .isNotEmpty()
        }

        composeRule.onNodeWithText("Connected to tcp://10.0.2.2:1883").assertIsDisplayed()
    }
}
