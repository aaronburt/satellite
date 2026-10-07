package uk.co.aaronburt.satellite.feature.entities

/** One read-only entity row: the Home Assistant key, its label and the value. */
data class EntityRow(
    val key: String,
    val label: String,
    val value: String,
)

/**
 * One controllable entity row.
 *
 * @property enabled whether Home Assistant is allowed to change it (off by default)
 */
data class ControlRow(
    val key: String,
    val label: String,
    val value: String,
    val enabled: Boolean,
)

data class EntitiesUiState(
    val sensors: List<EntityRow> = emptyList(),
    val controls: List<ControlRow> = emptyList(),
)
