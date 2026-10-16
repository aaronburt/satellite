package uk.co.aaronburt.satellite.app

import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.List
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.feature.dashboard.StatusRoute
import uk.co.aaronburt.satellite.feature.entities.EntitiesRoute
import uk.co.aaronburt.satellite.feature.settings.PermissionsScreen
import uk.co.aaronburt.satellite.feature.settings.SettingsRoute

/** Top-level destinations shown in the bottom navigation bar. */
private enum class SatelliteTab(val route: String, val label: String, val icon: ImageVector) {
    STATUS("status", "Status", Icons.Filled.Home),
    ENTITIES("entities", "Entities", Icons.Filled.List),
    PERMISSIONS("permissions", "Permissions", Icons.Filled.Lock),
    SETTINGS("settings", "Settings", Icons.Filled.Settings),
}

/**
 * App shell: a single top app bar, the four-tab bottom navigation bar, and the
 * navigation graph. System back pops the back stack.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SatelliteRoot() {
    val navController = rememberNavController()
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route

    Scaffold(
        topBar = { TopAppBar(title = { Text(titleFor(currentRoute)) }) },
        bottomBar = {
            NavigationBar {
                SatelliteTab.entries.forEach { tab ->
                    NavigationBarItem(
                        selected = currentRoute == tab.route,
                        onClick = { navController.switchTab(tab) },
                        icon = { Icon(tab.icon, contentDescription = null) },
                        label = { Text(tab.label) },
                    )
                }
            }
        },
    ) { innerPadding ->
        NavHost(
            navController = navController,
            startDestination = SatelliteTab.STATUS.route,
            modifier = Modifier.padding(innerPadding),
        ) {
            composable(SatelliteTab.STATUS.route) {
                StatusRoute(onOpenSettings = { navController.switchTab(SatelliteTab.SETTINGS) })
            }
            composable(SatelliteTab.ENTITIES.route) { EntitiesRoute() }
            composable(SatelliteTab.PERMISSIONS.route) { PermissionsScreen() }
            composable(SatelliteTab.SETTINGS.route) { SettingsRoute() }
        }
    }
}

private fun titleFor(route: String?): String = when (route) {
    SatelliteTab.ENTITIES.route -> "Entities \u00b7 ${EntityCatalog.entities.size}"
    SatelliteTab.PERMISSIONS.route -> "Permissions"
    SatelliteTab.SETTINGS.route -> "Settings"
    else -> "Status"
}

/** Switches tabs without stacking duplicates, preserving each tab's scroll state. */
private fun NavHostController.switchTab(tab: SatelliteTab) {
    navigate(tab.route) {
        popUpTo(graph.findStartDestination().id) { saveState = true }
        launchSingleTop = true
        restoreState = true
    }
}
