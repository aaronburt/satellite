package uk.co.aaronburt.satellite.common.logging

/**
 * Minimal logging abstraction. The Android implementation will be added in a
 * later milestone; keeping the interface here avoids leaking Android types into
 * the core modules.
 */
interface AppLogger {
    fun debug(tag: String, message: String)
    fun info(tag: String, message: String)
    fun warn(tag: String, message: String, throwable: Throwable? = null)
    fun error(tag: String, message: String, throwable: Throwable? = null)
}
