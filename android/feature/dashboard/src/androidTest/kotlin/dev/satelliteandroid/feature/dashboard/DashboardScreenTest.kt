package dev.satelliteandroid.feature.dashboard

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.test.ext.junit.runners.AndroidJUnit4
import dev.satelliteandroid.model.ConnectionState
import dev.satelliteandroid.model.Transport
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class DashboardScreenTest {

    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun showsDisconnectedState() {
        composeRule.setContent {
            DashboardScreen(
                state = ConnectionState.Disconnected,
                transport = Transport.MQTT,
                onConfigure = {},
                onPermissions = {},
            )
        }

        composeRule.onNodeWithText("Satellite").assertIsDisplayed()
        composeRule.onNodeWithText("Not configured").assertIsDisplayed()
    }

    @Test
    fun showsConnectedBroker() {
        composeRule.setContent {
            DashboardScreen(
                state = ConnectionState.Connected(
                    broker = "tcp://10.0.2.2:1883",
                    connectedSinceEpochMillis = 0L,
                ),
                transport = Transport.MQTT,
                onConfigure = {},
                onPermissions = {},
            )
        }

        composeRule.onNodeWithText("Connected to tcp://10.0.2.2:1883").assertIsDisplayed()
    }
}
