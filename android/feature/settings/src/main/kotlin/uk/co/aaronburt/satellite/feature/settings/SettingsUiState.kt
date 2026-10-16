package uk.co.aaronburt.satellite.feature.settings

import uk.co.aaronburt.satellite.model.ThemePreference
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.UpdateMode
import uk.co.aaronburt.satellite.update.UpdateState

/**
 * @property deviceName generated randomly on first launch, then editable.
 * @property themePreference colour scheme (System / Light / Dark).
 * @property reportingEnabled master switch; off stops publishing and the service.
 * @property updateMode periodic "base ping" vs event-driven publishing.
 * @property updateInterval base publish interval.
 * @property transport MQTT broker or HTTP webhook.
 * @property host defaults to the emulator alias for the host machine; change it
 * to the PC's LAN IP when running on a physical device.
 * @property webhookUrl endpoint for [Transport.WEBHOOK].
 */
data class SettingsUiState(
    val deviceName: String = "",
    val themePreference: ThemePreference = ThemePreference.SYSTEM,
    val reportingEnabled: Boolean = true,
    val updateMode: UpdateMode = UpdateMode.PERIODIC,
    val updateInterval: UpdateInterval = UpdateInterval.FIVE_MINUTES,
    val transport: Transport = Transport.MQTT,
    val host: String = DEFAULT_HOST,
    val port: String = DEFAULT_PORT,
    val username: String = "",
    val password: String = "",
    val useTls: Boolean = true,
    val webhookUrl: String = DEFAULT_WEBHOOK_URL,
    val bearerToken: String = "",
    val testStatus: String? = null,
    val saved: Boolean = false,
    val appVersion: String = "",
    val update: UpdateState = UpdateState.Idle,
    val canInstallUpdates: Boolean = false,
) {
    companion object {
        const val DEFAULT_HOST = "10.0.2.2"
        const val DEFAULT_PORT = "8883"
        const val DEFAULT_WEBHOOK_URL = "https://10.0.2.2:8123/api/webhook/"
    }
}
