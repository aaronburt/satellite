package uk.co.aaronburt.satellite.feature.dashboard

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle

@Composable
fun StatusRoute(
    onOpenSettings: () -> Unit,
    viewModel: StatusViewModel = hiltViewModel(),
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    val testStatus by viewModel.testStatus.collectAsStateWithLifecycle()

    StatusScreen(
        state = state,
        testStatus = testStatus,
        onReportingChange = viewModel::onReportingChange,
        onTest = viewModel::onTest,
        onSetUpBroker = onOpenSettings,
        onUseWebhook = {
            viewModel.onUseWebhook()
            onOpenSettings()
        },
    )
}
