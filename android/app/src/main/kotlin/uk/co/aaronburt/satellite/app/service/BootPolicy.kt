package uk.co.aaronburt.satellite.app.service

import uk.co.aaronburt.satellite.datastore.SettingsRepository
import kotlinx.coroutines.flow.first

/**
 * Whether the service should come back after a reboot: only once something
 * has been configured and reporting has not been paused, so the user never
 * gets a permanent notification they did not ask for.
 */
internal object BootPolicy {
    suspend fun shouldStartService(settings: SettingsRepository): Boolean =
        settings.isConfigured() && settings.reportingEnabled.first()
}
