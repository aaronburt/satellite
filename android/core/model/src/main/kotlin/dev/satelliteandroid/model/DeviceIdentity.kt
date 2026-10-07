package dev.satelliteandroid.model

/**
 * Stable identity of this app instance as it will be presented to Home
 * Assistant. The [deviceId] is derived from a hashed Android ID and must remain
 * stable for the lifetime of the install.
 */
data class DeviceIdentity(
    val deviceId: String,
    val name: String,
    val manufacturer: String,
    val model: String,
    val appVersion: String,
    val androidVersion: String,
)
