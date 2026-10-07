package uk.co.aaronburt.satellite.designsystem.theme

import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/**
 * Colour roles Material 3 does not define. Provided by [SatelliteTheme] so
 * screens can use semantic colours (e.g. a green "connected" state) without
 * hardcoding hex values.
 */
@Immutable
data class ExtendedColors(
    val success: Color,
    val successContainer: Color,
    val onSuccessContainer: Color,
)

internal val LightExtendedColors = ExtendedColors(
    success = Color(0xFF1B873F),
    successContainer = Color(0xFFDFF6E3),
    onSuccessContainer = Color(0xFF06421C),
)

internal val DarkExtendedColors = ExtendedColors(
    success = Color(0xFF7BD88F),
    successContainer = Color(0xFF0E3B1E),
    onSuccessContainer = Color(0xFFA6F1B8),
)

val LocalExtendedColors = staticCompositionLocalOf { LightExtendedColors }
