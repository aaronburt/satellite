package uk.co.aaronburt.satellite.feature.settings

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.clickable
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
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material3.Button
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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

@Composable
fun SettingsScreen(
    state: SettingsUiState,
    onDeviceNameChange: (String) -> Unit,
    onThemeChange: (ThemePreference) -> Unit,
    onReportingChange: (Boolean) -> Unit,
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
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        SectionLabel("Identity")
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
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
            }
        }

        SectionLabel("Appearance")
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text("Theme", style = MaterialTheme.typography.bodyMedium)
                ChoiceRow(
                    options = ThemePreference.entries,
                    selected = state.themePreference,
                    label = { it.label() },
                    onSelect = onThemeChange,
                )
            }
        }

        SectionLabel("Reporting")
        ElevatedCard {
            Column(modifier = Modifier.fillMaxWidth()) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "Send to Home Assistant",
                            style = MaterialTheme.typography.bodyLarge,
                        )
                        Text(
                            text = "Turn off to stop publishing without removing the app",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    Switch(
                        checked = state.reportingEnabled,
                        onCheckedChange = onReportingChange,
                        modifier = Modifier.testTag("settings_reporting"),
                    )
                }

                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)

                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text("Update mode", style = MaterialTheme.typography.bodySmall)
                    ChoiceRow(
                        options = UpdateMode.entries,
                        selected = state.updateMode,
                        label = { it.label() },
                        onSelect = onUpdateModeChange,
                        enabled = state.reportingEnabled,
                    )

                    Text(
                        text = "Publish interval",
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                    ChoiceRow(
                        options = UpdateInterval.entries,
                        selected = state.updateInterval,
                        label = { it.label() },
                        onSelect = onUpdateIntervalChange,
                        enabled = state.reportingEnabled,
                    )
                }
            }
        }

        SectionLabel("Connection")
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Text("Transport", style = MaterialTheme.typography.bodySmall)
                ChoiceRow(
                    options = Transport.entries,
                    selected = state.transport,
                    label = { it.label() },
                    onSelect = onTransportChange,
                )

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
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("Use TLS", modifier = Modifier.weight(1f))
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
                }

                Button(
                    onClick = onSave,
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_save"),
                ) {
                    Text("Save & reconnect")
                }

                OutlinedButton(
                    onClick = onTest,
                    enabled = state.reportingEnabled,
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_test"),
                ) {
                    Text("Send test now")
                }

                state.testStatus?.let {
                    Text(text = it, style = MaterialTheme.typography.bodySmall)
                }
                if (state.saved) {
                    Text(
                        text = "Saved.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.primary,
                    )
                }
            }
        }

        SectionLabel("App")
        ElevatedCard {
            Column(modifier = Modifier.fillMaxWidth()) {
                DeveloperOptions()
            }
        }

        Text(
            text = "Satellite ${state.appVersion}",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(start = 4.dp),
        )
    }
}

@Composable
private fun DeveloperOptions() {
    var expanded by remember { mutableStateOf(false) }

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { expanded = !expanded }
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text("Developer options", style = MaterialTheme.typography.bodyLarge)
            Text(
                text = "Test broker hints",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Icon(
            imageVector = if (expanded) Icons.Filled.KeyboardArrowUp else Icons.Filled.KeyboardArrowDown,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }

    AnimatedVisibility(visible = expanded) {
        Text(
            text = "Dev environment: MQTT at 10.0.2.2:1883 (satellite / satellite), " +
                "or a webhook on http://10.0.2.2:8123. On a physical device use the PC's LAN IP.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(start = 16.dp, end = 16.dp, bottom = 16.dp),
        )
    }
}

@Composable
private fun SectionLabel(text: String) {
    Text(
        text = text.uppercase(),
        style = MaterialTheme.typography.labelMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier.padding(start = 4.dp),
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun <T> ChoiceRow(
    options: List<T>,
    selected: T,
    label: (T) -> String,
    onSelect: (T) -> Unit,
    enabled: Boolean = true,
) {
    SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
        options.forEachIndexed { index, option ->
            SegmentedButton(
                selected = option == selected,
                onClick = { onSelect(option) },
                enabled = enabled,
                shape = SegmentedButtonDefaults.itemShape(index = index, count = options.size),
                label = { Text(label(option)) },
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
    UpdateInterval.THIRTY_SECONDS -> "30s"
    UpdateInterval.ONE_MINUTE -> "1m"
    UpdateInterval.FIVE_MINUTES -> "5m"
    UpdateInterval.FIFTEEN_MINUTES -> "15m"
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
            state = SettingsUiState(appVersion = "0.2.0"),
            onDeviceNameChange = {},
            onThemeChange = {},
            onReportingChange = {},
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
        )
    }
}
