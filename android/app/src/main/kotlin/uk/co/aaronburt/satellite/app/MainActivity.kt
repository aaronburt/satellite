package uk.co.aaronburt.satellite.app

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
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dagger.hilt.android.AndroidEntryPoint
import uk.co.aaronburt.satellite.app.service.SatelliteService
import uk.co.aaronburt.satellite.datastore.SettingsRepository
import uk.co.aaronburt.satellite.designsystem.theme.SatelliteTheme
import uk.co.aaronburt.satellite.model.ThemePreference
import javax.inject.Inject

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
                SatelliteNavHost()
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
