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
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LifecycleResumeEffect

private data class RuntimePermission(
    val permission: String,
    val label: Int,
    val rationale: Int,
    val group: Int,
    val minSdk: Int = 0,
    val maxSdk: Int = Int.MAX_VALUE
)

private fun runtimePermissions(): List<RuntimePermission> = buildList {
    add(
        RuntimePermission(
            Manifest.permission.ACCESS_COARSE_LOCATION,
            R.string.settings_approximate_location,
            R.string.permission_coarse_rationale,
            R.string.settings_sensors,
            maxSdk = Build.VERSION_CODES.S_V2,
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.ACCESS_FINE_LOCATION,
            R.string.settings_precise_location,
            R.string.permission_fine_rationale,
            R.string.settings_sensors,
            maxSdk = Build.VERSION_CODES.S_V2,
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.NEARBY_WIFI_DEVICES,
            R.string.settings_nearby_wi_fi,
            R.string.permission_wifi_rationale,
            R.string.settings_sensors,
            minSdk = Build.VERSION_CODES.TIRAMISU,
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.READ_PHONE_STATE,
            R.string.settings_phone_state,
            R.string.permission_phone_rationale,
            R.string.settings_sensors,
        ),
    )
    add(
        RuntimePermission(
            Manifest.permission.POST_NOTIFICATIONS,
            R.string.settings_notifications,
            R.string.permission_notification_rationale,
            R.string.settings_reliability,
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
                Column {
                    Text(
                        text = stringResource(R.string.permissions_count, grantedCount, permissions.size),
                        modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
                        style = MaterialTheme.typography.titleSmall,
                    )
                    Text(
                        text = stringResource(R.string.settings_more_unlocks_more_entities),
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
            SectionLabel(stringResource(group))
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

        SectionLabel(stringResource(R.string.settings_battery))
        ElevatedCard {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Column {
                    Text(
                        stringResource(R.string.settings_battery_optimisation),
                        style = MaterialTheme.typography.bodyLarge
                    )
                    Text(
                        text = stringResource(R.string.battery_rationale),
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
                    Text(stringResource(R.string.open_battery_settings))
                }
            }
        }
    }
}

@Composable
private fun PermissionRow(item: RuntimePermission, granted: Boolean, onGrant: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Column {
            Text(stringResource(item.label), style = MaterialTheme.typography.bodyLarge)
            Text(
                text = stringResource(item.rationale),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (granted) {
            Text(
                text = stringResource(R.string.settings_granted),
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier,
            )
        } else {
            Button(onClick = onGrant) {
                Text(stringResource(R.string.grant_permission, stringResource(item.label)))
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
        modifier = Modifier.padding(start = 4.dp).semantics { heading() },
    )
}

private fun Context.isGranted(permission: String): Boolean =
    ContextCompat.checkSelfPermission(this, permission) == PackageManager.PERMISSION_GRANTED
