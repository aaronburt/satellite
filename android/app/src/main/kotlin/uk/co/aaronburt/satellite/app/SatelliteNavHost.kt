package uk.co.aaronburt.satellite.app

import androidx.compose.runtime.Composable
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import uk.co.aaronburt.satellite.feature.dashboard.DashboardRoute
import uk.co.aaronburt.satellite.feature.settings.PermissionsScreen
import uk.co.aaronburt.satellite.feature.settings.SettingsRoute

private object Routes {
    const val DASHBOARD = "dashboard"
    const val SETTINGS = "settings"
    const val PERMISSIONS = "permissions"
}

/**
 * Top-level navigation. System back is handled by the back stack, so the
 * hardware/gesture back gesture returns to the dashboard instead of exiting.
 */
@Composable
fun SatelliteNavHost() {
    val navController = rememberNavController()

    NavHost(navController = navController, startDestination = Routes.DASHBOARD) {
        composable(Routes.DASHBOARD) {
            DashboardRoute(
                onConfigure = { navController.navigate(Routes.SETTINGS) },
                onPermissions = { navController.navigate(Routes.PERMISSIONS) },
            )
        }
        composable(Routes.SETTINGS) {
            SettingsRoute(onBack = { navController.popBackStack() })
        }
        composable(Routes.PERMISSIONS) {
            PermissionsScreen(onBack = { navController.popBackStack() })
        }
    }
}
