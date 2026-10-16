package uk.co.aaronburt.satellite.model

/**
 * How the app delivers state to the outside world.
 *
 * - [MQTT]: publish discovery + state to an MQTT broker (Home Assistant native).
 * - [WEBHOOK]: HTTP POST a JSON snapshot to a URL (e.g. a Home Assistant
 *   webhook trigger, or any custom endpoint).
 */
enum class Transport {
    MQTT,
    WEBHOOK,
}
