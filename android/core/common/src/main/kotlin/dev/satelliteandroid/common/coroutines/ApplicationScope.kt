package dev.satelliteandroid.common.coroutines

import javax.inject.Qualifier

/**
 * Marks the application-wide [kotlinx.coroutines.CoroutineScope] that outlives
 * any screen and is used for long-running work such as the MQTT connection.
 */
@Qualifier
@Retention(AnnotationRetention.RUNTIME)
annotation class ApplicationScope
