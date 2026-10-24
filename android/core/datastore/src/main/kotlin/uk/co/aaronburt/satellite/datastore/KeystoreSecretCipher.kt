package uk.co.aaronburt.satellite.datastore

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * AES-256-GCM with a non-exportable key held in the Android Keystore.
 *
 * Stored format: `v1:` + Base64(12-byte IV || ciphertext+tag). The key never
 * leaves the Keystore and is not part of any backup, so ciphertext restored
 * onto another device (or after the key is wiped) fails to decrypt and the
 * secret reads as absent.
 */
class KeystoreSecretCipher(private val alias: String = DEFAULT_ALIAS) : SecretCipher {

    private val keyStore: KeyStore by lazy { KeyStore.getInstance(PROVIDER).apply { load(null) } }

    override fun encrypt(plaintext: String): String {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, getOrCreateKey())
        val sealed = cipher.iv + cipher.doFinal(plaintext.toByteArray(Charsets.UTF_8))
        return PREFIX + Base64.encodeToString(sealed, Base64.NO_WRAP)
    }

    override fun decrypt(ciphertext: String): String? {
        if (!ciphertext.startsWith(PREFIX)) return null
        val key = existingKey() ?: return null
        return try {
            val sealed = Base64.decode(ciphertext.removePrefix(PREFIX), Base64.NO_WRAP)
            if (sealed.size <= IV_BYTES) return null
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(TAG_BITS, sealed, 0, IV_BYTES))
            String(cipher.doFinal(sealed, IV_BYTES, sealed.size - IV_BYTES), Charsets.UTF_8)
        } catch (e: android.security.keystore.KeyPermanentlyInvalidatedException) {
            // The key can never decrypt again; drop it so the next save makes a fresh one.
            runCatching { keyStore.deleteEntry(alias) }
            null
        } catch (e: Exception) {
            // Deliberately no logging: the exception may carry key or data details.
            null
        }
    }

    private fun existingKey(): SecretKey? =
        runCatching { (keyStore.getEntry(alias, null) as? KeyStore.SecretKeyEntry)?.secretKey }.getOrNull()

    private fun getOrCreateKey(): SecretKey = existingKey() ?: KeyGenerator
        .getInstance(KeyProperties.KEY_ALGORITHM_AES, PROVIDER)
        .apply {
            init(
                KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setKeySize(256)
                    .build(),
            )
        }
        .generateKey()

    private companion object {
        const val PROVIDER = "AndroidKeyStore"
        const val DEFAULT_ALIAS = "satellite_settings_secrets"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val PREFIX = "v1:"
        const val IV_BYTES = 12
        const val TAG_BITS = 128
    }
}
