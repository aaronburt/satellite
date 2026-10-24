package uk.co.aaronburt.satellite.datastore

/**
 * Encrypts secret settings (broker password, webhook token) before they are
 * written to DataStore. Implementations must never log the values they handle.
 */
interface SecretCipher {
    /** Returns an opaque, storable ciphertext. Throws if the key is unavailable. */
    fun encrypt(plaintext: String): String

    /**
     * Returns the plaintext, or `null` when the ciphertext cannot be decrypted
     * (key deleted, invalidated, or not restored with a backup). A `null` here
     * means "the user has to enter this secret again"; it never affects other
     * settings.
     */
    fun decrypt(ciphertext: String): String?
}
