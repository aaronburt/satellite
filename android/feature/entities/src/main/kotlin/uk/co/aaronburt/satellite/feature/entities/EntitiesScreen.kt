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
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.co.aaronburt.satellite.designsystem.theme.SatelliteTheme

@Composable
fun EntitiesScreen(
    state: EntitiesUiState,
    onControlToggled: (String, Boolean) -> Unit,
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
                    SensorRow(row)
                }
            }
        }

        SectionLabel("Controls")
        Text(
            text = "Off by default. Turn one on to let Home Assistant change it.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = 4.dp),
        )
        ElevatedCard {
            Column(Modifier.fillMaxWidth()) {
                state.controls.forEachIndexed { index, control ->
                    if (index > 0) {
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                    }
                    ControlItem(control, onControlToggled)
                }
            }
        }
    }
}

@Composable
private fun SensorRow(row: EntityRow) {
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
private fun ControlItem(control: ControlRow, onControlToggled: (String, Boolean) -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = control.label,
                style = MaterialTheme.typography.bodyLarge,
                color = if (control.available) {
                    MaterialTheme.colorScheme.onSurface
                } else {
                    MaterialTheme.colorScheme.onSurfaceVariant
                },
            )
            Text(
                text = if (!control.available) {
                    "Not supported on this device"
                } else {
                    "${control.value} \u00b7 " + if (control.enabled) {
                        "Visible to Home Assistant"
                    } else {
                        "Hidden from Home Assistant"
                    }
                },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Switch(
            checked = control.enabled,
            onCheckedChange = { onControlToggled(control.key, it) },
            // Greyed out when the device cannot honour the control at all.
            enabled = control.available,
            modifier = Modifier.testTag("control_${control.key}"),
        )
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
                controls = listOf(
                    ControlRow("volume_media", "Media volume", "70%", enabled = false),
                    ControlRow("mic_muted", "Microphone muted", "Live", enabled = false, available = false),
                ),
            ),
            onControlToggled = { _, _ -> },
        )
    }
}
