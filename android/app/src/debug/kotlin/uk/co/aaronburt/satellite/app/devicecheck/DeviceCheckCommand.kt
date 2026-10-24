package uk.co.aaronburt.satellite.app.devicecheck

import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.Transport
import uk.co.aaronburt.satellite.model.UpdateInterval
import uk.co.aaronburt.satellite.model.WebhookSettings

/**
 * One settings change requested by the device-check script over adb.
 *
 * Extras arrive as strings (`am broadcast --es key value`). [parse] turns them
 * into commands, applied in the order listed here, so one broadcast can, for
 * example, configure the broker and switch reporting on.
 */
sealed interface DeviceCheckCommand {
    data class ConfigureBroker(val settings: BrokerSettings) : DeviceCheckCommand
    data class SetInterval(val interval: UpdateInterval) : DeviceCheckCommand
    data class SetWebhook(val settings: WebhookSettings) : DeviceCheckCommand
    data class SetTransport(val transport: Transport) : DeviceCheckCommand
    data class SetControl(val key: String, val enabled: Boolean) : DeviceCheckCommand
    data class SetReporting(val enabled: Boolean) : DeviceCheckCommand

    companion object {
        private const val DEFAULT_PORT = 1883
        private const val MAX_PORT = 65535

        /**
         * @throws IllegalArgumentException for a malformed value, so a typo in
         * the script fails loudly instead of silently testing the wrong state.
         */
        fun parse(extras: Map<String, String>): List<DeviceCheckCommand> {
            val commands = mutableListOf<DeviceCheckCommand>()

            extras["host"]?.let { host ->
                require(host.isNotBlank()) { "host is blank" }
                val port =
                    extras["port"]?.let { requireNotNull(it.toIntOrNull()) { "port '$it' is not a number" } }
                        ?: DEFAULT_PORT
                require(port in 1..MAX_PORT) { "port $port out of range" }
                commands += ConfigureBroker(
                    BrokerSettings(
                        host = host,
                        port = port,
                        username = extras["username"]?.ifBlank { null },
                        password = extras["password"]?.ifBlank { null },
                        useTls = extras["tls"]?.let(::parseBoolean) ?: false,
                    ),
                )
            }

            extras["interval"]?.let { raw ->
                val seconds = raw.toLongOrNull()
                val interval = requireNotNull(UpdateInterval.entries.firstOrNull { it.seconds == seconds }) {
                    "interval '$raw' must be one of ${UpdateInterval.entries.map { it.seconds }}"
                }
                commands += SetInterval(interval)
            }

            extras["webhook_url"]?.let { url ->
                require(url.startsWith("http://") || url.startsWith("https://")) { "webhook_url '$url' is not http(s)" }
                commands += SetWebhook(WebhookSettings(url, extras["bearer"]?.ifBlank { null }))
            }

            extras["transport"]?.let { raw ->
                val transport =
                    requireNotNull(Transport.entries.firstOrNull { it.name.equals(raw, ignoreCase = true) }) {
                        "transport '$raw' must be mqtt or webhook"
                    }
                commands += SetTransport(transport)
            }

            extras["control"]?.let { key ->
                require(key.isNotBlank()) { "control is blank" }
                val enabled =
                    requireNotNull(extras["control_enabled"]?.let(::parseBoolean)) { "control needs control_enabled" }
                commands += SetControl(key, enabled)
            }

            // Last, so the service starts with every other change already saved.
            extras["reporting"]?.let { commands += SetReporting(parseBoolean(it)) }

            return commands
        }

        private fun parseBoolean(raw: String): Boolean = when (raw.lowercase()) {
            "true", "1", "on" -> true
            "false", "0", "off" -> false
            else -> throw IllegalArgumentException("'$raw' is not a boolean")
        }
    }
}
