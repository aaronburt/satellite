package dev.satelliteandroid.feature.dashboard

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import dev.satelliteandroid.datastore.SettingsRepository
import dev.satelliteandroid.model.ConnectionState
import dev.satelliteandroid.model.Transport
import dev.satelliteandroid.mqtt.MqttConnectionManager
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
