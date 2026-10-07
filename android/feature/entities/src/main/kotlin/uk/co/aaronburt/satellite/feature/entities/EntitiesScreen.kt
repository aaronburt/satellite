package uk.co.aaronburt.satellite.feature.entities

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.co.aaronburt.satellite.designsystem.theme.SatelliteTheme

@Composable
fun EntitiesScreen(
    state: EntitiesUiState,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        SectionLabel("Sensors")
        ElevatedCard {
            Column(Modifier.fillMaxWidth()) {
                state.sensors.forEachIndexed { index, row ->
                    if (index > 0) {
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                    }
                    EntityRowItem(row)
                }
            }
        }

        SectionLabel("Controls \u00b7 next update")
        ElevatedCard {
            Column(Modifier.fillMaxWidth()) {
                VolumeRow(state.mediaVolumePercent)
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                MicrophoneRow(state.microphoneMuted)
            }
        }
        Text(
            text = "Controls become interactive when the command milestone lands " +
                "\u2014 for now they are read-only.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = 4.dp),
        )
    }
}

@Composable
private fun EntityRowItem(row: EntityRow) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = row.label,
            style = MaterialTheme.typography.bodyMedium,
            modifier = Modifier.weight(1f),
        )
        Text(
            text = row.value,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

@Composable
private fun VolumeRow(percent: Int?) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                text = "Media volume",
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.weight(1f),
            )
            Text(
                text = percent?.let { "$it%" } ?: "\u2014",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.SemiBold,
            )
        }
        Slider(
            value = (percent ?: 0) / 100f,
            onValueChange = {},
            enabled = false,
        )
    }
}

@Composable
private fun MicrophoneRow(muted: Boolean?) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = "Microphone muted",
            style = MaterialTheme.typography.bodyMedium,
            modifier = Modifier.weight(1f),
        )
        Switch(checked = muted ?: false, onCheckedChange = null, enabled = false)
    }
}

@Composable
private fun SectionLabel(text: String) {
    Text(
        text = text.uppercase(),
        style = MaterialTheme.typography.labelMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier.padding(start = 4.dp),
    )
}

@Preview(showBackground = true)
@Composable
private fun EntitiesScreenPreview() {
    SatelliteTheme {
        EntitiesScreen(
            state = EntitiesUiState(
                sensors = listOf(
                    EntityRow("battery", "Battery", "78%"),
                    EntityRow("charging", "Charging", "On"),
                    EntityRow("wifi_ssid", "Wi-Fi network", "HomeNet"),
                ),
                mediaVolumePercent = 70,
                microphoneMuted = false,
            ),
        )
    }
}
