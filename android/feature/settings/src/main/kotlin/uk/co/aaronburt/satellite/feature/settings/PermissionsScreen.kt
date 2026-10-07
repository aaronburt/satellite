package uk.co.aaronburt.satellite.feature.settings

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
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
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
    val minSdk: Int = 0,
)

private fun runtimePermissions(): List<RuntimePermission> = buildList {
    add(RuntimePermission(Manifest.permission.ACCESS_COARSE_LOCATION, "Approximate location"))
    add(RuntimePermission(Manifest.permission.ACCESS_FINE_LOCATION, "Precise location"))
    add(RuntimePermission(Manifest.permission.ACCESS_BACKGROUND_LOCATION, "Background location", Build.VERSION_CODES.Q))
    add(RuntimePermission(Manifest.permission.ACTIVITY_RECOGNITION, "Physical activity", Build.VERSION_CODES.Q))
    add(RuntimePermission(Manifest.permission.READ_PHONE_STATE, "Phone state"))
    add(RuntimePermission(Manifest.permission.BLUETOOTH_CONNECT, "Bluetooth", Build.VERSION_CODES.S))
    add(RuntimePermission(Manifest.permission.BLUETOOTH_SCAN, "Bluetooth scanning", Build.VERSION_CODES.S))
    add(RuntimePermission(Manifest.permission.NEARBY_WIFI_DEVICES, "Nearby Wi-Fi", Build.VERSION_CODES.TIRAMISU))
    add(RuntimePermission(Manifest.permission.POST_NOTIFICATIONS, "Notifications", Build.VERSION_CODES.TIRAMISU))
}.filter { Build.VERSION.SDK_INT >= it.minSdk }

@Composable
fun PermissionsScreen(
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
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

    Scaffold(modifier = modifier.fillMaxSize()) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Permissions", style = MaterialTheme.typography.headlineMedium)
                TextButton(onClick = onBack) { Text("Back") }
            }

            Text(
                "Grant the device-state permissions you want the satellite to report. " +
                    "Each one unlocks more entities in Home Assistant.",
                style = MaterialTheme.typography.bodyMedium,
            )

            permissions.forEach { item ->
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(item.label, style = MaterialTheme.typography.bodyLarge)
                        Text(
                            text = if (granted[item.permission] == true) "Granted" else "Not granted",
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    if (granted[item.permission] != true) {
                        val requiresSettings =
                            item.permission == Manifest.permission.ACCESS_BACKGROUND_LOCATION
                        Button(
                            onClick = {
                                if (requiresSettings) {
                                    context.openAppSettings()
                                } else {
                                    launcher.launch(arrayOf(item.permission))
                                }
                            },
                        ) {
                            Text(if (requiresSettings) "Settings" else "Grant")
                        }
                    }
                }
            }

            HorizontalDivider()

            Text("Special access (opens system Settings)", style = MaterialTheme.typography.titleMedium)

            OutlinedButton(
                onClick = { context.startActivity(Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS)) },
                modifier = Modifier.fillMaxWidth(),
            ) { Text("Usage access") }

            OutlinedButton(
                onClick = { context.startActivity(Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS)) },
                modifier = Modifier.fillMaxWidth(),
            ) { Text("Notification access") }

            OutlinedButton(
                onClick = { context.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS)) },
                modifier = Modifier.fillMaxWidth(),
            ) { Text("Battery optimization") }

            OutlinedButton(
                onClick = { context.openAppSettings() },
                modifier = Modifier.fillMaxWidth(),
            ) { Text("App settings") }
        }
    }
}

private fun Context.isGranted(permission: String): Boolean =
    ContextCompat.checkSelfPermission(this, permission) == PackageManager.PERMISSION_GRANTED

private fun Context.openAppSettings() {
    startActivity(
        Intent(
            Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
            Uri.fromParts("package", packageName, null),
        ),
    )
}
