package uk.co.aaronburt.satellite.feature.entities

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle

@Composable
fun EntitiesRoute(viewModel: EntitiesViewModel = hiltViewModel()) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    EntitiesScreen(
        state = state,
        onControlToggled = viewModel::onControlToggled,
    )
}
