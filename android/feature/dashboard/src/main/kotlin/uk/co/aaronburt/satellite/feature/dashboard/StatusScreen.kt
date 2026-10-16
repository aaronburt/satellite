package uk.co.aaronburt.satellite.feature.dashboard

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay
import uk.co.aaronburt.satellite.designsystem.theme.LocalExtendedColors
import uk.co.aaronburt.satellite.designsystem.theme.SatelliteTheme
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.model.Transport

@Composable
fun StatusScreen(
    state: StatusUiState,
    testStatus: String?,
    onReportingChange: (Boolean) -> Unit,
    onTest: () -> Unit,
    onSetUpBroker: () -> Unit,
    onUseWebhook: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        if (state.configured) {
            ConnectionCard(state)
            ReportingCard(state.reportingEnabled, onReportingChange)

            if (state.highlights.isNotEmpty()) {
                SectionLabel("Highlights")
                HighlightsCard(state.highlights)
            }

            Spacer(Modifier.height(4.dp))
            FilledTonalButton(
                onClick = onTest,
                enabled = state.reportingEnabled,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text("Send test now")
            }
            testStatus?.let {
                Text(
                    text = it,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        } else {
            Onboarding(onSetUpBroker = onSetUpBroker, onUseWebhook = onUseWebhook)
        }
    }
}

@Composable
private fun ConnectionCard(state: StatusUiState) {
    ElevatedCard {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            StatusPill(state)

            Text(
                text = state.deviceName,
                style = MaterialTheme.typography.headlineSmall,
            )
            Text(
                text = state.statusLine(),
                style = MaterialTheme.typography.bodyMedium,
                fontFamily = FontFamily.Monospace,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            HorizontalDivider(Modifier.padding(vertical = 8.dp))

            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Stat("Last published", rememberRelativeTime(state.lastPublishedAtMillis), Modifier.weight(1f))
                Stat("Entities", state.entityCount.toString(), Modifier.weight(1f))
            }
        }
    }
}

@Composable
private fun StatusPill(state: StatusUiState) {
    val extended = LocalExtendedColors.current
    val scheme = MaterialTheme.colorScheme

    val (container, content, label) = when {
        !state.reportingEnabled ->
            Triple(scheme.surfaceVariant, scheme.onSurfaceVariant, "Paused")
        state.connectionState is ConnectionState.Connected ->
            Triple(extended.successContainer, extended.onSuccessContainer, "Connected")
        state.connectionState is ConnectionState.Error ->
            Triple(scheme.errorContainer, scheme.onErrorContainer, "Error")
        state.connectionState is ConnectionState.Connecting ->
            Triple(scheme.primaryContainer, scheme.onPrimaryContainer, "Connecting")
        else ->
            Triple(scheme.surfaceVariant, scheme.onSurfaceVariant, "Not configured")
    }

    Surface(color = container, shape = CircleShape) {
        Row(
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .clip(CircleShape)
                    .background(content),
            )
            Text(
                text = label,
                style = MaterialTheme.typography.labelLarge,
                color = content,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

@Composable
private fun ReportingCard(enabled: Boolean, onChange: (Boolean) -> Unit) {
    ElevatedCard {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text("Reporting", style = MaterialTheme.typography.titleSmall)
                Text(
                    text = if (enabled) {
                        "Publishing device state to Home Assistant"
                    } else {
                        "Paused \u2014 Home Assistant shows this device offline"
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Switch(checked = enabled, onCheckedChange = onChange)
        }
    }
}

@Composable
private fun HighlightsCard(highlights: List<StatusHighlight>) {
    ElevatedCard {
        Column(Modifier.fillMaxWidth()) {
            highlights.forEachIndexed { index, highlight ->
                if (index > 0) {
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 16.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        text = highlight.label,
                        style = MaterialTheme.typography.bodyMedium,
                        modifier = Modifier.weight(1f),
                    )
                    Text(
                        text = highlight.value,
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }
        }
    }
}

@Composable
private fun Onboarding(onSetUpBroker: () -> Unit, onUseWebhook: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(
            text = "Let's connect your satellite",
            style = MaterialTheme.typography.headlineSmall,
        )
        Text(
            text = "Point it at your Home Assistant broker and it will register itself " +
                "and start reporting device state.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )

        Spacer(Modifier.height(4.dp))

        listOf(
            "Enter your broker host and credentials",
            "Grant permissions to unlock more entities",
            "Watch it appear in Home Assistant",
        ).forEachIndexed { index, step ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Surface(
                    color = MaterialTheme.colorScheme.secondaryContainer,
                    shape = CircleShape,
                ) {
                    Box(Modifier.size(22.dp), contentAlignment = Alignment.Center) {
                        Text(
                            text = "${index + 1}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSecondaryContainer,
                        )
                    }
                }
                Text(
                    text = step,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }

        Spacer(Modifier.height(4.dp))

        Button(
            onClick = onSetUpBroker,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text("Set up broker")
        }
        TextButton(onClick = onUseWebhook) {
            Text("Use a webhook instead")
        }
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

@Composable
private fun Stat(label: String, value: String, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier,
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
        shape = MaterialTheme.shapes.medium,
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text(
                text = value,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

private fun StatusUiState.statusLine(): String = when {
    !reportingEnabled -> "Not reporting to Home Assistant"
    transport == Transport.WEBHOOK -> "Webhook mode"
    else -> when (val connection = connectionState) {
        ConnectionState.Disconnected -> "Not configured"
        ConnectionState.Connecting -> "Connecting\u2026"
        is ConnectionState.Connected -> connection.broker
        is ConnectionState.Error -> connection.message
    }
}

/** Formats [millis] as "12s ago", ticking every second while on screen. */
@Composable
private fun rememberRelativeTime(millis: Long?): String {
    if (millis == null) return "never"

    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(millis) {
        while (true) {
            now = System.currentTimeMillis()
            delay(1_000L)
        }
    }

    val seconds = ((now - millis) / 1000).coerceAtLeast(0)
    return when {
        seconds < 5 -> "just now"
        seconds < 60 -> "${seconds}s ago"
        seconds < 3_600 -> "${seconds / 60}m ago"
        seconds < 86_400 -> "${seconds / 3_600}h ago"
        else -> "${seconds / 86_400}d ago"
    }
}

@Preview(showBackground = true)
@Composable
private fun StatusScreenPreview() {
    SatelliteTheme {
        StatusScreen(
            state = StatusUiState(
                deviceName = "Satellite-02A1",
                connectionState = ConnectionState.Connected("mqtt://192.168.1.10:1883", 0L),
                lastPublishedAtMillis = System.currentTimeMillis() - 12_000,
                entityCount = 15,
                highlights = listOf(
                    StatusHighlight("battery", "Battery", "78%"),
                    StatusHighlight("charging", "Charging", "On"),
                ),
            ),
            testStatus = null,
            onReportingChange = {},
            onTest = {},
            onSetUpBroker = {},
            onUseWebhook = {},
        )
    }
}
