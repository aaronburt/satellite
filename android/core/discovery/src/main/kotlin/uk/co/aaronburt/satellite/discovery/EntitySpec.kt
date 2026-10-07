package uk.co.aaronburt.satellite.discovery

/**
 * Declaration of a single Home Assistant entity exposed by the app.
 *
 * @property key stable identifier, also the MQTT state topic segment
 * @property platform HA MQTT platform (`sensor`, `binary_sensor`, `number`, …)
 * @property name entity name shown in HA (device name is prefixed automatically)
 * @property deviceClass HA `device_class`, if any
 * @property unit `unit_of_measurement`, if any
 * @property stateClass HA `state_class` (`measurement`, `total_increasing`, …)
 * @property entityCategory `diagnostic` keeps it out of the main dashboard
 * @property icon MDI icon
 * @property options allowed values, required for `device_class: enum`
 */
data class EntitySpec(
    val key: String,
    val platform: String,
    val name: String,
    val deviceClass: String? = null,
    val unit: String? = null,
    val stateClass: String? = null,
    val entityCategory: String? = "diagnostic",
    val icon: String? = null,
    val options: List<String>? = null,
)
