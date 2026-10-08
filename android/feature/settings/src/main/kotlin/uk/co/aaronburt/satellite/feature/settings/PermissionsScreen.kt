package uk.co.aaronburt.satellite.feature.settings

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LifecycleResumeEffect

private data class RuntimePermission(
    val permission: String,
    val label: String,
    val rationale: String,
    val group: String,
    val minSdk: Int = 0,
    val maxSdk: Int = Int.MAX_VALUE,
)

private fun runtimePermissions(): List<RuntimePermission> = buildList {
    add(
        RuntimePermission(
            Manifest.permission.ACCESS_COARSE_LOCATION,
            "Approximate location",
            "Requested with precise location on Android 12+; only used to read " +
                "the Wi-Fi network name",
            "Sensors",
            maxSdk = Build.VERSION_CODES.S_V2,
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.ACCESS_FINE_LOCATION,
            "Precise location",
            "Reads the connected Wi-Fi network name on Android 12 and below",
            "Sensors",
            maxSdk = Build.VERSION_CODES.S_V2,
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.NEARBY_WIFI_DEVICES,
            "Nearby Wi-Fi",
            "Reads the connected Wi-Fi network name",
            "Sensors",
            minSdk = Build.VERSION_CODES.TIRAMISU,
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.READ_PHONE_STATE,
            "Phone state",
            "Reports the mobile carrier",
            "Sensors",
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.POST_NOTIFICATIONS,
            "Notifications",
            "Shows the connection-status notification",
            "Reliability",
            minSdk = Build.VERSION_CODES.TIRAMISU,
        ),
    )
}.filter { Build.VERSION.SDK_INT in it.minSdk..it.maxSdk }

@Composable
fun PermissionsScreen(modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val permissions = remember { runtimePermissions() }
    var granted by remember {
        mutableStateOf(permissions.associate { it.permission to context.isGranted(it.permission) })
    }

    LifecycleResumeEffect(Unit) {
        granted = permissions.associate { it.permission to context.isGranted(it.permission) }
        onPauseOrDispose { }
    }

    val launcher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { result ->
        granted = granted + result
    }

    val grantedCount = permissions.count { granted[it.permission] == true }

    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = "$grantedCount of ${permissions.size} granted",
                        style = MaterialTheme.typography.titleSmall,
                        modifier = Modifier.weight(1f),
                    )
                    Text(
                        text = "more unlocks more entities",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                LinearProgressIndicator(
                    progress = { grantedCount.toFloat() / permissions.size },
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 12.dp),
                )
            }
        }

        permissions.groupBy { it.group }.forEach { (group, items) ->
            SectionLabel(group)
            ElevatedCard {
                Column(Modifier.fillMaxWidth()) {
                    items.forEachIndexed { index, item ->
                        if (index > 0) {
                            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                        }
                        PermissionRow(
                            item = item,
                            granted = granted[item.permission] == true,
                            onGrant = { launcher.launch(arrayOf(item.permission)) },
                        )
                    }
                }
            }
        }

        SectionLabel("Battery")
        ElevatedCard {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text("Battery optimisation", style = MaterialTheme.typography.bodyLarge)
                    Text(
                        text = "Recommended \u2014 keeps the connection alive",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                Button(
                    onClick = {
                        context.startActivity(
                            Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS),
                        )
                    },
                ) {
                    Text("Open")
                }
            }
        }
    }
}

@Composable
private fun PermissionRow(
    item: RuntimePermission,
    granted: Boolean,
    onGrant: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(item.label, style = MaterialTheme.typography.bodyLarge)
            Text(
                text = item.rationale,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (granted) {
            Text(
                text = "Granted",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(start = 12.dp),
            )
        } else {
            Button(onClick = onGrant, modifier = Modifier.padding(start = 12.dp)) {
                Text("Grant")
            }
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

private fun Context.isGranted(permission: String): Boolean =
    ContextCompat.checkSelfPermission(this, permission) == PackageManager.PERMISSION_GRANTED
