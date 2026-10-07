package uk.co.aaronburt.satellite.feature.settings

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.platform.LocalContext
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle

@Composable
fun SettingsRoute(viewModel: SettingsViewModel = hiltViewModel()) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    val context = LocalContext.current

    // The user may grant "install unknown apps" in system settings and come back.
    LifecycleResumeEffect(Unit) {
        viewModel.refreshUpdateCapabilities()
        onPauseOrDispose { }
    }

    SettingsScreen(
        state = state,
        onDeviceNameChange = viewModel::onDeviceNameChange,
        onThemeChange = viewModel::onThemeChange,
        onReportingChange = viewModel::onReportingChange,
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
        onCheckForUpdates = viewModel::onCheckForUpdates,
        onInstallUpdate = viewModel::onInstallUpdate,
        onOpenInstallSettings = { context.startActivity(viewModel.unknownSourcesIntent()) },
    )
}
