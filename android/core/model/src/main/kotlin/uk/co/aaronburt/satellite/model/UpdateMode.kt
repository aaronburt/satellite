package uk.co.aaronburt.satellite.model

/**
 * How device state reaches Home Assistant.
 *
 * - [PERIODIC]: publish on a fixed interval only ("base ping").
 * - [EVENT_DRIVEN]: publish immediately when relevant state changes, with the
 *   periodic interval still running as a safety net.
 */
enum class UpdateMode {
    PERIODIC,
    EVENT_DRIVEN,
}
