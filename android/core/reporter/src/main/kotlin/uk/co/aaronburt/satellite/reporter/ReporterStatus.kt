package uk.co.aaronburt.satellite.reporter

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Shared, in-memory view of reporting health for the UI: when the last
 * successful publish happened. Reset when reporting is switched off.
 */
@Singleton
class ReporterStatus @Inject constructor() {

    private val _lastPublishedAtMillis = MutableStateFlow<Long?>(null)
    val lastPublishedAtMillis: StateFlow<Long?> = _lastPublishedAtMillis.asStateFlow()

    fun markPublished() {
        _lastPublishedAtMillis.value = System.currentTimeMillis()
    }
}
