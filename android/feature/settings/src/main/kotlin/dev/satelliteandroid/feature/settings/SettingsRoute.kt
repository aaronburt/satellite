package dev.satelliteandroid.feature.settings

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle

@Composable
fun SettingsRoute(
    onBack: () -> Unit,
    viewModel: SettingsViewModel = hiltViewModel(),
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    SettingsScreen(
        state = state,
        onDeviceNameChange = viewModel::onDeviceNameChange,
        onThemeChange = viewModel::onThemeChange,
        onUpdateModeChange = viewModel::onUpdateModeChange,
        onUpdateIntervalChange = viewModel::onUpdateIntervalChange,
        onTransportChange = viewModel::onTransportChange,
        onHostChange = viewModel::onHostChange,
        onPortChange = viewModel::onPortChange,
        onUsernameChange = viewModel::onUsernameChange,
        onPasswordChange = viewModel::onPasswordChange,
        onUseTlsChange = viewModel::onUseTlsChange,
        onWebhookUrlChange = viewModel::onWebhookUrlChange,
        onBearerTokenChange = viewModel::onBearerTokenChange,
        onSave = viewModel::save,
        onTest = viewModel::onTest,
        onBack = onBack,
    )
}
