package uk.co.aaronburt.satellite.app.service

import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import uk.co.aaronburt.satellite.model.WebhookSettings
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository.Companion.BROKER

class BootPolicyTest {

    @Test
    fun `restarts after boot once a broker is configured`() = runTest {
        assertTrue(BootPolicy.shouldStartService(FakeSettingsRepository(broker = BROKER)))
    }

    @Test
    fun `restarts after boot once a webhook is configured`() = runTest {
        val settings = FakeSettingsRepository(webhook = WebhookSettings("https://ha.local/hook"))
        assertTrue(BootPolicy.shouldStartService(settings))
    }

    @Test
    fun `stays off when nothing is configured`() = runTest {
        assertFalse(BootPolicy.shouldStartService(FakeSettingsRepository()))
        assertFalse(BootPolicy.shouldStartService(FakeSettingsRepository(webhook = WebhookSettings(" "))))
    }

    @Test
    fun `stays off when reporting was paused`() = runTest {
        val settings = FakeSettingsRepository(broker = BROKER, reportingEnabled = false)
        assertFalse(BootPolicy.shouldStartService(settings))
    }
}
