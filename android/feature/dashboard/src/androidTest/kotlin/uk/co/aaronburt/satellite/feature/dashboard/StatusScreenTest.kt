package uk.co.aaronburt.satellite.feature.dashboard

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.test.ext.junit.runners.AndroidJUnit4
import uk.co.aaronburt.satellite.model.ConnectionState
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class StatusScreenTest {

    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun showsConnectedDevice() {
        composeRule.setContent {
            StatusScreen(
                state = StatusUiState(
                    deviceName = "Satellite-02A1",
                    connectionState = ConnectionState.Connected("mqtt://10.0.2.2:1883", 0L),
                    entityCount = 15,
                ),
                testStatus = null,
                onReportingChange = {},
                onTest = {},
                onSetUpBroker = {},
                onUseWebhook = {},
            )
        }

        composeRule.onNodeWithText("Satellite-02A1").assertIsDisplayed()
        composeRule.onNodeWithText("Connected").assertIsDisplayed()
        composeRule.onNodeWithText("mqtt://10.0.2.2:1883").assertIsDisplayed()
    }

    @Test
    fun pausedStateIsShown() {
        composeRule.setContent {
            StatusScreen(
                state = StatusUiState(
                    deviceName = "Satellite-02A1",
                    reportingEnabled = false,
                ),
                testStatus = null,
                onReportingChange = {},
                onTest = {},
                onSetUpBroker = {},
                onUseWebhook = {},
            )
        }

        composeRule.onNodeWithText("Paused").assertIsDisplayed()
        composeRule.onNodeWithText("Not reporting to Home Assistant").assertIsDisplayed()
    }

    @Test
    fun firstRunShowsOnboarding() {
        composeRule.setContent {
            StatusScreen(
                state = StatusUiState(configured = false),
                testStatus = null,
                onReportingChange = {},
                onTest = {},
                onSetUpBroker = {},
                onUseWebhook = {},
            )
        }

        composeRule.onNodeWithText("Let's connect your satellite").assertIsDisplayed()
        composeRule.onNodeWithText("Set up broker").assertIsDisplayed()
    }
}
