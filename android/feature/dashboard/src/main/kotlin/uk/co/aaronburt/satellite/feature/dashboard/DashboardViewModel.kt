package uk.co.aaronburt.satellite.feature.dashboard

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.mqtt.MqttConnectionManager
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import javax.inject.Inject

@HiltViewModel
class DashboardViewModel @Inject constructor(
    connectionManager: MqttConnectionManager,
    settingsRepository: SettingsRepository,
) : ViewModel() {

    val state: StateFlow<ConnectionState> = connectionManager.connectionState

    val transport: StateFlow<Transport> = settingsRepository.transport
        .stateIn(
            scope = viewModelScope,
            started = SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS),
            initialValue = Transport.MQTT,
        )

    private companion object {
        const val STOP_TIMEOUT_MILLIS = 5_000L
    }
}
