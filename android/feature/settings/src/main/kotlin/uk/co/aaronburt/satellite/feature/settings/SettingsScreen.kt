package uk.co.aaronburt.satellite.feature.settings

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.co.aaronburt.satellite.designsystem.theme.SatelliteTheme
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    state: SettingsUiState,
    onDeviceNameChange: (String) -> Unit,
    onThemeChange: (ThemePreference) -> Unit,
    onUpdateModeChange: (UpdateMode) -> Unit,
    onUpdateIntervalChange: (UpdateInterval) -> Unit,
    onTransportChange: (Transport) -> Unit,
    onHostChange: (String) -> Unit,
    onPortChange: (String) -> Unit,
    onUsernameChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onUseTlsChange: (Boolean) -> Unit,
    onWebhookUrlChange: (String) -> Unit,
    onBearerTokenChange: (String) -> Unit,
    onSave: () -> Unit,
    onTest: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier.fillMaxSize(),
        topBar = {
            TopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            OutlinedTextField(
                value = state.deviceName,
                onValueChange = onDeviceNameChange,
                label = { Text("Device name") },
                singleLine = true,
                modifier = Modifier
                    .fillMaxWidth()
                    .testTag("settings_device_name"),
            )

            Text("Theme", style = MaterialTheme.typography.titleMedium)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                ThemePreference.entries.forEach { preference ->
                    FilterChip(
                        selected = state.themePreference == preference,
                        onClick = { onThemeChange(preference) },
                        label = { Text(preference.label()) },
                    )
                }
            }

            Text("Updates", style = MaterialTheme.typography.titleMedium)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                UpdateMode.entries.forEach { mode ->
                    FilterChip(
                        selected = state.updateMode == mode,
                        onClick = { onUpdateModeChange(mode) },
                        label = { Text(mode.label()) },
                    )
                }
            }
            Text("Publish interval", style = MaterialTheme.typography.bodyMedium)
            Row(
                modifier = Modifier.horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                UpdateInterval.entries.forEach { interval ->
                    FilterChip(
                        selected = state.updateInterval == interval,
                        onClick = { onUpdateIntervalChange(interval) },
                        label = { Text(interval.label()) },
                    )
                }
            }

            Text("Transport", style = MaterialTheme.typography.titleMedium)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Transport.entries.forEach { transport ->
                    FilterChip(
                        selected = state.transport == transport,
                        onClick = { onTransportChange(transport) },
                        label = { Text(transport.label()) },
                    )
                }
            }

            if (state.transport == Transport.MQTT) {
                OutlinedTextField(
                    value = state.host,
                    onValueChange = onHostChange,
                    label = { Text("Host") },
                    singleLine = true,
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_host"),
                )

                OutlinedTextField(
                    value = state.port,
                    onValueChange = onPortChange,
                    label = { Text("Port") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_port"),
                )

                OutlinedTextField(
                    value = state.username,
                    onValueChange = onUsernameChange,
                    label = { Text("Username (optional)") },
                    singleLine = true,
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_username"),
                )

                OutlinedTextField(
                    value = state.password,
                    onValueChange = onPasswordChange,
                    label = { Text("Password (optional)") },
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_password"),
                )

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text("Use TLS")
                    Switch(checked = state.useTls, onCheckedChange = onUseTlsChange)
                }
            } else {
                OutlinedTextField(
                    value = state.webhookUrl,
                    onValueChange = onWebhookUrlChange,
                    label = { Text("Webhook URL") },
                    singleLine = true,
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_webhook_url"),
                )
                OutlinedTextField(
                    value = state.bearerToken,
                    onValueChange = onBearerTokenChange,
                    label = { Text("Bearer token (optional)") },
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_bearer_token"),
                )
                Text(
                    text = "POSTs a JSON snapshot, e.g. " +
                        "http://<home-assistant>:8123/api/webhook/<webhook-id>. " +
                        "The bearer token is sent as an Authorization header.",
                    style = MaterialTheme.typography.bodySmall,
                )
            }

            Button(
                onClick = onSave,
                modifier = Modifier
                    .fillMaxWidth()
                    .testTag("settings_save"),
            ) {
                Text("Save and connect")
            }

            OutlinedButton(
                onClick = onTest,
                modifier = Modifier
                    .fillMaxWidth()
                    .testTag("settings_test"),
            ) {
                Text("Send test now")
            }

            state.testStatus?.let { status ->
                Text(text = status, style = MaterialTheme.typography.bodyMedium)
            }

            if (state.saved) {
                Text(
                    text = "Saved.",
                    style = MaterialTheme.typography.bodyMedium,
                )
            }

            Text(
                text = "Dev environment: MQTT at 10.0.2.2:1883 (satellite / satellite), " +
                    "or a webhook on http://10.0.2.2:8123. On a physical device use the PC's LAN IP.",
                style = MaterialTheme.typography.bodySmall,
            )
        }
    }
}

private fun ThemePreference.label(): String = when (this) {
    ThemePreference.SYSTEM -> "System"
    ThemePreference.LIGHT -> "Light"
    ThemePreference.DARK -> "Dark"
}

private fun UpdateMode.label(): String = when (this) {
    UpdateMode.PERIODIC -> "Periodic"
    UpdateMode.EVENT_DRIVEN -> "Event-driven"
}

private fun UpdateInterval.label(): String = when (this) {
    UpdateInterval.THIRTY_SECONDS -> "30 s"
    UpdateInterval.ONE_MINUTE -> "1 min"
    UpdateInterval.FIVE_MINUTES -> "5 min"
    UpdateInterval.FIFTEEN_MINUTES -> "15 min"
}

private fun Transport.label(): String = when (this) {
    Transport.MQTT -> "MQTT"
    Transport.WEBHOOK -> "Webhook"
}

@Preview(showBackground = true)
@Composable
private fun SettingsScreenPreview() {
    SatelliteTheme {
        SettingsScreen(
            state = SettingsUiState(),
            onDeviceNameChange = {},
            onThemeChange = {},
            onUpdateModeChange = {},
            onUpdateIntervalChange = {},
            onTransportChange = {},
            onHostChange = {},
            onPortChange = {},
            onUsernameChange = {},
            onPasswordChange = {},
            onUseTlsChange = {},
            onWebhookUrlChange = {},
            onBearerTokenChange = {},
            onSave = {},
            onTest = {},
            onBack = {},
        )
    }
}
