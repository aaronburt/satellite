package uk.co.aaronburt.satellite.update

import android.content.pm.PackageInfo
import android.content.pm.Signature
import android.content.pm.SigningInfo
import java.io.File
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.util.ReflectionHelpers

@RunWith(RobolectricTestRunner::class)
class UpdateVerifierTest {

    @get:Rule
    val folder = TemporaryFolder()

    private val context = RuntimeEnvironment.getApplication()
    private val packageManager = shadowOf(context.packageManager)
    private val verifier = UpdateVerifier(context)

    private val ownKey = Signature(ByteArray(32) { 1 })
    private val otherKey = Signature(ByteArray(32) { 2 })

    private fun apk(content: String = "apk bytes"): File = folder.newFile().apply { writeText(content) }

    // SHA-256 of "apk bytes"
    private val apkBytesSha = java.security.MessageDigest.getInstance("SHA-256")
        .digest("apk bytes".toByteArray()).joinToString("") { "%02x".format(it) }

    private fun signedBy(vararg keys: Signature): SigningInfo =
        ReflectionHelpers.callConstructor(SigningInfo::class.java).also {
            shadowOf(it).setSignatures(arrayOf(*keys))
        }

    private fun installedSignedBy(vararg keys: Signature) {
        packageManager.getInternalMutablePackageInfo(context.packageName).signingInfo = signedBy(*keys)
    }

    private fun archiveSignedBy(apk: File, vararg keys: Signature) {
        packageManager.setPackageArchiveInfo(
            apk.absolutePath,
            PackageInfo().apply {
                packageName = context.packageName
                signingInfo = signedBy(*keys)
            },
        )
    }

    @Test
    fun `a matching checksum passes, in either case`() {
        assertTrue(verifier.verifySha256(apk(), apkBytesSha))
        assertTrue(verifier.verifySha256(apk(), apkBytesSha.uppercase()))
    }

    @Test
    fun `a different checksum fails`() {
        assertFalse(verifier.verifySha256(apk("tampered"), apkBytesSha))
    }

    @Test
    fun `a missing file fails the checksum`() {
        assertFalse(verifier.verifySha256(File(folder.root, "gone.apk"), apkBytesSha))
    }

    @Test
    fun `no published checksum is not a failure`() {
        // Older GitHub assets have no digest; the signature check still applies.
        assertTrue(verifier.verifySha256(apk(), null))
        assertTrue(verifier.verifySha256(apk(), " "))
    }

    @Test
    fun `an update signed with the app's own key passes`() {
        val apk = apk()
        installedSignedBy(ownKey)
        archiveSignedBy(apk, ownKey)

        assertTrue(verifier.signatureMatchesInstalledApp(apk))
    }

    @Test
    fun `an update signed with another key fails`() {
        val apk = apk()
        installedSignedBy(ownKey)
        archiveSignedBy(apk, otherKey)

        assertFalse(verifier.signatureMatchesInstalledApp(apk))
    }

    @Test
    fun `an update with an extra or missing signer fails`() {
        val apk = apk()
        installedSignedBy(ownKey)
        archiveSignedBy(apk, ownKey, otherKey)
        assertFalse(verifier.signatureMatchesInstalledApp(apk))

        installedSignedBy(ownKey, otherKey)
        archiveSignedBy(apk, ownKey)
        assertFalse(verifier.signatureMatchesInstalledApp(apk))
    }

    @Test
    fun `a file that is not an APK fails`() {
        installedSignedBy(ownKey)

        assertFalse(verifier.signatureMatchesInstalledApp(apk("garbage")))
    }

    @Test
    fun `nothing passes when the installed app's signers cannot be read`() {
        val apk = apk()
        archiveSignedBy(apk)
        packageManager.getInternalMutablePackageInfo(context.packageName).signingInfo = null

        assertFalse(verifier.signatureMatchesInstalledApp(apk))
    }
}
