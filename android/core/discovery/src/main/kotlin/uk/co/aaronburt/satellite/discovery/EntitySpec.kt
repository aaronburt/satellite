package uk.co.aaronburt.satellite.discovery

/**
 * Declares that an entity accepts commands from Home Assistant.
 *
 * @property action topic segment under `<base>/cmd/` and the handler key
 * @property kind payload contract the handler expects
 */
data class CommandSpec(
    val action: String,
    val kind: Kind,
) {
    enum class Kind {
        /** Integer 0–100. */
        NUMBER,

        /** `ON` / `OFF`. */
        ON_OFF,
    }
}

/**
 * Declaration of a single Home Assistant entity exposed by the app.
 *
 * Entities with a [command] are **controllable**. They are never exposed by
 * default: the user opts in per entity, so the app cannot be used to change the
 * phone's state unless they asked for it.
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
 * @property command present for controllable entities
 * @property min inclusive minimum for `number` platforms
 * @property max inclusive maximum for `number` platforms
 * @property step increment for `number` platforms
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
    val command: CommandSpec? = null,
    val min: Int? = null,
    val max: Int? = null,
    val step: Int? = null,
) {
    /** Controllable entities are opt-in and off by default. */
    val isControl: Boolean get() = command != null
}
