package dev.satelliteandroid.app

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dagger.hilt.android.AndroidEntryPoint
import dev.satelliteandroid.app.service.SatelliteService
import dev.satelliteandroid.datastore.SettingsRepository
import dev.satelliteandroid.designsystem.theme.SatelliteTheme
import dev.satelliteandroid.feature.dashboard.DashboardRoute
import dev.satelliteandroid.feature.settings.PermissionsScreen
import dev.satelliteandroid.feature.settings.SettingsRoute
import dev.satelliteandroid.model.ThemePreference
import javax.inject.Inject

private const val SCREEN_DASHBOARD = "dashboard"
private const val SCREEN_SETTINGS = "settings"
private const val SCREEN_PERMISSIONS = "permissions"

@AndroidEntryPoint
class MainActivity : ComponentActivity() {

    @Inject
    lateinit var settingsRepository: SettingsRepository

    private val notificationPermissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { /* no-op */ }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            val themePreference by settingsRepository.themePreference
                .collectAsStateWithLifecycle(initialValue = ThemePreference.SYSTEM)
            val darkTheme = when (themePreference) {
                ThemePreference.SYSTEM -> isSystemInDarkTheme()
                ThemePreference.LIGHT -> false
                ThemePreference.DARK -> true
            }

            SatelliteTheme(darkTheme = darkTheme) {
                var screen by rememberSaveable { mutableStateOf(SCREEN_DASHBOARD) }
                when (screen) {
                    SCREEN_SETTINGS -> SettingsRoute(onBack = { screen = SCREEN_DASHBOARD })
                    SCREEN_PERMISSIONS -> PermissionsScreen(onBack = { screen = SCREEN_DASHBOARD })
                    else -> DashboardRoute(
                        onConfigure = { screen = SCREEN_SETTINGS },
                        onPermissions = { screen = SCREEN_PERMISSIONS },
                    )
                }
            }
        }

        requestNotificationPermissionIfNeeded()
        startSatelliteService()
    }

    private fun requestNotificationPermissionIfNeeded() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }

    private fun startSatelliteService() {
        ContextCompat.startForegroundService(this, Intent(this, SatelliteService::class.java))
    }
}
