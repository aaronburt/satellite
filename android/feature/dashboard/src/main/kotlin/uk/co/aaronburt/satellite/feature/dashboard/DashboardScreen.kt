package uk.co.aaronburt.satellite.feature.dashboard

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.co.aaronburt.satellite.designsystem.theme.SatelliteTheme
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DashboardScreen(
    state: ConnectionState,
    transport: Transport,
    onConfigure: () -> Unit,
    onPermissions: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier.fillMaxSize(),
        topBar = { TopAppBar(title = { Text("Satellite") }) },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(
                text = "Home Assistant companion over MQTT",
                style = MaterialTheme.typography.bodyMedium,
            )

            ElevatedCard {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text(
                        text = "Transport",
                        style = MaterialTheme.typography.titleMedium,
                    )
                    Text(
                        text = if (transport == Transport.WEBHOOK) {
                            "Webhook mode — posts state over HTTP"
                        } else {
                            state.describe()
                        },
                    )
                }
            }

            Button(
                onClick = onConfigure,
                modifier = Modifier
                    .fillMaxWidth()
                    .testTag("dashboard_configure"),
            ) {
                Text("Configure broker")
            }

            OutlinedButton(
                onClick = onPermissions,
                modifier = Modifier
                    .fillMaxWidth()
                    .testTag("dashboard_permissions"),
            ) {
                Text("Permissions")
            }
        }
    }
}

private fun ConnectionState.describe(): String = when (this) {
    ConnectionState.Disconnected -> "Not configured"
    ConnectionState.Connecting -> "Connecting…"
    is ConnectionState.Connected -> "Connected to $broker"
    is ConnectionState.Error -> "Error: $message"
}

@Preview(showBackground = true)
@Composable
private fun DashboardScreenPreview() {
    SatelliteTheme {
        DashboardScreen(
            state = ConnectionState.Disconnected,
            transport = Transport.MQTT,
            onConfigure = {},
            onPermissions = {},
        )
    }
}
