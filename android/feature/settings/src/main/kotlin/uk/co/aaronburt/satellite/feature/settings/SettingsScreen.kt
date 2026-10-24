package uk.co.aaronburt.satellite.feature.settings

import android.content.res.Configuration
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.co.aaronburt.satellite.designsystem.theme.SatelliteTheme
import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.update.UpdateState

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
    onCheckForUpdates: () -> Unit,
    onInstallUpdate: () -> Unit,
    onOpenInstallSettings: () -> Unit,
    modifier: Modifier = Modifier
) {
    val reportingLabel = stringResource(R.string.send_to_home_assistant)
    val tlsLabel = stringResource(R.string.use_tls)
    var confirmDisableTls by remember { mutableStateOf(false) }
    val fieldFocus = remember {
        listOf("deviceName", "host", "port", "webhookUrl").associateWith { FocusRequester() }
    }
    LaunchedEffect(state.errors) {
        state.errors.keys.firstOrNull()?.let { fieldFocus[it]?.requestFocus() }
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        SectionLabel(stringResource(R.string.settings_identity))
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                OutlinedTextField(
                    value = state.deviceName,
                    isError = state.errors.containsKey("deviceName"),
                    supportingText = { state.errors["deviceName"]?.let { Text(stringResource(it)) } },
                    onValueChange = onDeviceNameChange,
                    enabled = !state.busy,
                    label = { Text(stringResource(R.string.settings_device_name)) },
                    singleLine = true,
                    modifier = Modifier
                        .fillMaxWidth()
                        .focusRequester(fieldFocus.getValue("deviceName"))
                        .testTag("settings_device_name"),
                )
            }
        }

        SectionLabel(stringResource(R.string.settings_appearance))
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text(stringResource(R.string.settings_theme), style = MaterialTheme.typography.bodyMedium)
                ChoiceRow(
                    options = ThemePreference.entries,
                    selected = state.themePreference,
                    label = { it.label() },
                    onSelect = onThemeChange,
                )
            }
        }

        SectionLabel(stringResource(R.string.settings_reporting))
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
                            text = stringResource(R.string.send_to_home_assistant),
                            style = MaterialTheme.typography.bodyLarge,
                        )
                        Text(
                            text = stringResource(R.string.reporting_description),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    Switch(
                        checked = state.reportingEnabled,
                        onCheckedChange = onReportingChange,
                        modifier = Modifier.testTag("settings_reporting").semantics {
                            contentDescription =
                                reportingLabel
                        },
                    )
                }

                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)

                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text(stringResource(R.string.settings_update_mode), style = MaterialTheme.typography.bodySmall)
                    ChoiceRow(
                        options = UpdateMode.entries,
                        selected = state.updateMode,
                        label = { it.label() },
                        onSelect = onUpdateModeChange,
                        enabled = state.reportingEnabled,
                    )

                    Text(
                        text = stringResource(R.string.settings_publish_interval),
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

        SectionLabel(stringResource(R.string.settings_connection))
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Text(stringResource(R.string.settings_transport), style = MaterialTheme.typography.bodySmall)
                ChoiceRow(
                    options = Transport.entries,
                    selected = state.transport,
                    label = { it.label() },
                    onSelect = onTransportChange,
                    enabled = !state.busy,
                )

                if (state.transport == Transport.MQTT) {
                    OutlinedTextField(
                        value = state.host,
                        isError = state.errors.containsKey("host"),
                        supportingText = { state.errors["host"]?.let { Text(stringResource(it)) } },
                        onValueChange = onHostChange,
                        enabled = !state.busy,
                        label = { Text(stringResource(R.string.settings_host)) },
                        singleLine = true,
                        modifier = Modifier
                            .fillMaxWidth()
                            .focusRequester(fieldFocus.getValue("host"))
                            .testTag("settings_host"),
                    )
                    OutlinedTextField(
                        value = state.port,
                        isError = state.errors.containsKey("port"),
                        supportingText = { state.errors["port"]?.let { Text(stringResource(it)) } },
                        onValueChange = onPortChange,
                        enabled = !state.busy,
                        label = { Text(stringResource(R.string.settings_port)) },
                        singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                        modifier = Modifier
                            .fillMaxWidth()
                            .focusRequester(fieldFocus.getValue("port"))
                            .testTag("settings_port"),
                    )
                    OutlinedTextField(
                        value = state.username,
                        onValueChange = onUsernameChange,
                        enabled = !state.busy,
                        label = { Text(stringResource(R.string.settings_username_optional)) },
                        singleLine = true,
                        modifier = Modifier
                            .fillMaxWidth()
                            .testTag("settings_username"),
                    )
                    CredentialField(
                        value = state.password,
                        onValueChange = onPasswordChange,
                        isToken = false,
                        enabled = !state.busy,
                        modifier = Modifier.fillMaxWidth().testTag("settings_password"),
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(stringResource(R.string.use_tls), modifier = Modifier.weight(1f))
                        Switch(
                            checked = state.useTls,
                            enabled = !state.busy,
                            modifier = Modifier.semantics { contentDescription = tlsLabel },
                            onCheckedChange = { enabled ->
                                if (enabled) onUseTlsChange(true) else confirmDisableTls = true
                            },
                        )
                    }
                    if (!state.useTls) {
                        Text(
                            text = stringResource(R.string.tls_disabled_warning),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                } else {
                    OutlinedTextField(
                        value = state.webhookUrl,
                        isError = state.errors.containsKey("webhookUrl"),
                        supportingText = { state.errors["webhookUrl"]?.let { Text(stringResource(it)) } },
                        onValueChange = onWebhookUrlChange,
                        enabled = !state.busy,
                        label = { Text(stringResource(R.string.settings_webhook_url)) },
                        singleLine = true,
                        modifier = Modifier
                            .fillMaxWidth()
                            .focusRequester(fieldFocus.getValue("webhookUrl"))
                            .testTag("settings_webhook_url"),
                    )
                    CredentialField(
                        value = state.bearerToken,
                        onValueChange = onBearerTokenChange,
                        isToken = true,
                        enabled = !state.busy,
                        modifier = Modifier.fillMaxWidth().testTag("settings_bearer_token"),
                    )
                    if (state.webhookUrl.trim().startsWith("http://", ignoreCase = true)) {
                        Text(
                            text = stringResource(R.string.http_warning),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }

                Button(
                    onClick = onSave,
                    enabled = !state.busy,
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_save"),
                ) {
                    Text(stringResource(R.string.settings_save_reconnect))
                }

                OutlinedButton(
                    onClick = onTest,
                    enabled = state.reportingEnabled && !state.busy,
                    modifier = Modifier
                        .fillMaxWidth()
                        .testTag("settings_test"),
                ) {
                    Text(stringResource(R.string.settings_send_test_now))
                }

                Column(Modifier.semantics { liveRegion = LiveRegionMode.Polite }) {
                    state.feedback?.let { Text(stringResource(it), style = MaterialTheme.typography.bodySmall) }
                    state.testStatus?.let { Text(stringResource(it), style = MaterialTheme.typography.bodySmall) }
                }
            }
        }

        SectionLabel(stringResource(R.string.settings_app))
        ElevatedCard {
            Column(modifier = Modifier.fillMaxWidth()) {
                DeveloperOptions()
            }
        }

        UpdatesSection(
            state = state,
            onCheckForUpdates = onCheckForUpdates,
            onInstallUpdate = onInstallUpdate,
            onOpenInstallSettings = onOpenInstallSettings,
        )
    }

    if (confirmDisableTls) {
        AlertDialog(
            onDismissRequest = { confirmDisableTls = false },
            title = { Text(stringResource(R.string.settings_turn_off_encryption)) },
            text = {
                Text(
                    stringResource(R.string.tls_confirmation),
                )
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        confirmDisableTls = false
                        onUseTlsChange(false)
                    },
                ) { Text(stringResource(R.string.settings_turn_off_tls)) }
            },
            dismissButton = {
                TextButton(onClick = {
                    confirmDisableTls = false
                }) { Text(stringResource(R.string.settings_keep_tls_on)) }
            },
        )
    }
}

@Composable
private fun UpdatesSection(
    state: SettingsUiState,
    onCheckForUpdates: () -> Unit,
    onInstallUpdate: () -> Unit,
    onOpenInstallSettings: () -> Unit
) {
    SectionLabel(stringResource(R.string.settings_updates))
    ElevatedCard {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp)
                .semantics { liveRegion = LiveRegionMode.Polite },
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Column {
                Text(
                    text = stringResource(R.string.settings_installed_version),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(
                    text = state.appVersion,
                    style = MaterialTheme.typography.bodyLarge,
                )
            }

            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)

            when (val update = state.update) {
                UpdateState.Idle -> {
                    Button(onClick = onCheckForUpdates, modifier = Modifier.fillMaxWidth()) {
                        Text(stringResource(R.string.settings_check_for_updates))
                    }
                }

                UpdateState.Checking -> {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(18.dp),
                            strokeWidth = 2.dp,
                        )
                        Text(stringResource(R.string.settings_checking_github_for_a_newer_release))
                    }
                }

                is UpdateState.UpToDate -> {
                    Text(stringResource(R.string.settings_you_re_on_the_latest_version))
                    TextButton(onClick = onCheckForUpdates) { Text(stringResource(R.string.settings_check_again)) }
                }

                is UpdateState.Downloading -> {
                    Text(stringResource(R.string.update_downloading, update.versionName, update.progress))
                    LinearProgressIndicator(
                        progress = { update.progress / 100f },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }

                is UpdateState.Ready -> {
                    Text(
                        text = stringResource(R.string.update_ready, update.versionName),
                        style = MaterialTheme.typography.bodyLarge,
                    )
                    update.notes?.let {
                        Text(
                            text = it.lineSequence().take(4).joinToString("\n"),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    if (!state.canInstallUpdates) {
                        Text(
                            text = stringResource(R.string.settings_android_needs_to_allow_satellite_to_install_apps),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        Button(onClick = onOpenInstallSettings, modifier = Modifier.fillMaxWidth()) {
                            Text(stringResource(R.string.settings_allow_installs))
                        }
                    }
                    Button(
                        onClick = onInstallUpdate,
                        enabled = state.canInstallUpdates,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(stringResource(R.string.update_install, update.versionName))
                    }
                }

                is UpdateState.Failed -> {
                    Text(
                        text = update.message,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                    Button(onClick = onCheckForUpdates, modifier = Modifier.fillMaxWidth()) {
                        Text(stringResource(R.string.settings_try_again))
                    }
                }
            }
        }
    }
}

@Composable
private fun DeveloperOptions() {
    var expanded by remember { mutableStateOf(false) }
    val expansionState = stringResource(if (expanded) R.string.expanded else R.string.collapsed)

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { expanded = !expanded }
            .semantics { stateDescription = expansionState }
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(stringResource(R.string.settings_developer_options), style = MaterialTheme.typography.bodyLarge)
            Text(
                text = stringResource(R.string.settings_test_broker_hints),
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
            text = stringResource(R.string.developer_hints),
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
        modifier = Modifier.padding(start = 4.dp).semantics { heading() },
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun <T> ChoiceRow(
    options: List<T>,
    selected: T,
    label: @Composable (T) -> String,
    onSelect: (T) -> Unit,
    enabled: Boolean = true
) {
    FlowRow(
        modifier = Modifier.fillMaxWidth().selectableGroup(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        options.forEach { option ->
            FilterChip(
                selected = option == selected,
                onClick = { onSelect(option) },
                enabled = enabled,
                shape = MaterialTheme.shapes.small,
                label = { Text(label(option)) },
            )
        }
    }
}

@Composable
private fun ThemePreference.label(): String = when (this) {
    ThemePreference.SYSTEM -> stringResource(R.string.settings_system)
    ThemePreference.LIGHT -> stringResource(R.string.settings_light)
    ThemePreference.DARK -> stringResource(R.string.settings_dark)
}

@Composable
private fun UpdateMode.label(): String = when (this) {
    UpdateMode.PERIODIC -> stringResource(R.string.settings_periodic)
    UpdateMode.EVENT_DRIVEN -> stringResource(R.string.settings_event_driven)
}

@Composable
private fun UpdateInterval.label(): String = when (this) {
    UpdateInterval.THIRTY_SECONDS -> stringResource(R.string.settings_30s)
    UpdateInterval.ONE_MINUTE -> stringResource(R.string.settings_1m)
    UpdateInterval.FIVE_MINUTES -> stringResource(R.string.settings_5m)
    UpdateInterval.FIFTEEN_MINUTES -> stringResource(R.string.settings_15m)
}

@Composable
private fun Transport.label(): String = when (this) {
    Transport.MQTT -> stringResource(R.string.settings_mqtt)
    Transport.WEBHOOK -> stringResource(R.string.settings_webhook)
}

@Suppress("UnusedPrivateMember") // Used by Android Studio previews.
@Preview(name = "Light", showBackground = true)
@Preview(name = "Dark", showBackground = true, uiMode = Configuration.UI_MODE_NIGHT_YES)
@Preview(name = "Large text", showBackground = true, fontScale = 2f)
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
            onCheckForUpdates = {},
            onInstallUpdate = {},
            onOpenInstallSettings = {},
        )
    }
}
