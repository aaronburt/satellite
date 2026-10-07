package uk.co.aaronburt.satellite.feature.entities

/** One entity row: the Home Assistant key, its label and the current value. */
data class EntityRow(
    val key: String,
    val label: String,
    val value: String,
)

data class EntitiesUiState(
    val sensors: List<EntityRow> = emptyList(),
    val mediaVolumePercent: Int? = null,
    val microphoneMuted: Boolean? = null,
)
