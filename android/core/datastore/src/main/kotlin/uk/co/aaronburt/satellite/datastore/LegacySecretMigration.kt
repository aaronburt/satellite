package uk.co.aaronburt.satellite.datastore

import androidx.datastore.core.DataMigration
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.stringPreferencesKey

/**
 * One-time move of secrets that older versions stored in plaintext.
 *
 * DataStore runs this before the first read and commits the result in a single
 * atomic write, so an interruption leaves either the old file or the fully
 * migrated one, never a half state. If encryption fails (Keystore unavailable)
 * the migration throws, nothing is written, the plaintext value stays where it
 * was, and DataStore retries on the next open. Reads fall back to the legacy
 * value in the meantime, so nothing is lost and no other setting is touched.
 */
internal class LegacySecretMigration(private val cipher: SecretCipher) : DataMigration<Preferences> {

    override suspend fun shouldMigrate(currentData: Preferences): Boolean =
        PAIRS.any { (legacy, _) -> currentData[legacy] != null }

    override suspend fun migrate(currentData: Preferences): Preferences {
        val prefs = currentData.toMutablePreferences()
        for ((legacy, encrypted) in PAIRS) {
            val plaintext = prefs[legacy] ?: continue
            // An encrypted value already present is newer than the legacy one.
            if (prefs[encrypted] == null && plaintext.isNotBlank()) {
                prefs[encrypted] = cipher.encrypt(plaintext)
            }
            prefs.remove(legacy)
        }
        return prefs.toPreferences()
    }

    override suspend fun cleanUp() = Unit

    internal companion object {
        val LegacyPassword = stringPreferencesKey("broker_password")
        val EncryptedPassword = stringPreferencesKey("broker_password_enc")
        val LegacyWebhookToken = stringPreferencesKey("webhook_token")
        val EncryptedWebhookToken = stringPreferencesKey("webhook_token_enc")

        val PAIRS = listOf(
            LegacyPassword to EncryptedPassword,
            LegacyWebhookToken to EncryptedWebhookToken,
        )
    }
}
