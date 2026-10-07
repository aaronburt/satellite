package uk.co.aaronburt.satellite.discovery

/**
 * The entities the satellite reports by default. Every entry here needs no
 * runtime permission. Permission-gated entities are added in a later pass.
 */
object EntityCatalog {

    val entities: List<EntitySpec> = listOf(
        EntitySpec(
            key = "battery",
            platform = "sensor",
            name = "Battery",
            deviceClass = "battery",
            unit = "%",
            stateClass = "measurement",
            icon = "mdi:battery",
        ),
        EntitySpec(
            key = "charging",
            platform = "binary_sensor",
            name = "Charging",
            deviceClass = "battery_charging",
            icon = "mdi:battery-charging",
        ),
        EntitySpec(
            key = "charger_type",
            platform = "sensor",
            name = "Charger type",
            deviceClass = "enum",
            icon = "mdi:power-plug",
            options = listOf("none", "ac", "usb", "wireless", "dock"),
        ),
        EntitySpec(
            key = "battery_temperature",
            platform = "sensor",
            name = "Battery temperature",
            deviceClass = "temperature",
            unit = "°C",
            stateClass = "measurement",
            icon = "mdi:thermometer",
        ),
        EntitySpec(
            key = "power_save",
            platform = "binary_sensor",
            name = "Power save",
            deviceClass = "power",
            icon = "mdi:leaf",
        ),
        EntitySpec(
            key = "doze",
            platform = "binary_sensor",
            name = "Doze",
            icon = "mdi:sleep",
        ),
        EntitySpec(
            key = "interactive",
            platform = "binary_sensor",
            name = "Interactive",
            icon = "mdi:cellphone",
        ),
        EntitySpec(
            key = "app_version",
            platform = "sensor",
            name = "App version",
            icon = "mdi:cellphone-arrow-down",
        ),
        EntitySpec(
            key = "android_version",
            platform = "sensor",
            name = "Android version",
            icon = "mdi:android",
        ),
        EntitySpec(
            key = "uptime",
            platform = "sensor",
            name = "Uptime",
            deviceClass = "duration",
            unit = "s",
            stateClass = "total_increasing",
            icon = "mdi:timer-outline",
        ),
        EntitySpec(
            key = "network_transport",
            platform = "sensor",
            name = "Network transport",
            deviceClass = "enum",
            icon = "mdi:network",
            options = listOf("wifi", "cellular", "ethernet", "vpn", "none"),
        ),
        EntitySpec(
            key = "storage_free",
            platform = "sensor",
            name = "Storage free",
            deviceClass = "data_size",
            unit = "B",
            stateClass = "measurement",
            icon = "mdi:harddisk",
        ),
        EntitySpec(
            key = "ringer_mode",
            platform = "sensor",
            name = "Ringer mode",
            deviceClass = "enum",
            icon = "mdi:volume-high",
            options = listOf("normal", "vibrate", "silent"),
        ),
        EntitySpec(
            key = "wifi_ssid",
            platform = "sensor",
            name = "Wi-Fi network",
            icon = "mdi:wifi",
        ),
        EntitySpec(
            key = "carrier",
            platform = "sensor",
            name = "Carrier",
            icon = "mdi:sim",
        ),
    )
}
