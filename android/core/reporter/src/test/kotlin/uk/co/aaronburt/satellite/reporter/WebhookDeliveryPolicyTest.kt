package uk.co.aaronburt.satellite.reporter

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import uk.co.aaronburt.satellite.reporter.WebhookDeliveryPolicy.MAX_DELAY_MILLIS
import uk.co.aaronburt.satellite.reporter.WebhookDeliveryPolicy.MAX_RETRY_AFTER_MILLIS

class WebhookDeliveryPolicyTest {

    @Test
    fun `network failures, timeouts, rate limits and server errors are retryable`() {
        listOf(null, 408, 425, 429, 500, 502, 503, 599).forEach {
            assertTrue("$it", WebhookDeliveryPolicy.isRetryable(it))
        }
    }

    @Test
    fun `other client errors are permanent`() {
        listOf(400, 401, 403, 404, 410, 422).forEach {
            assertFalse("$it", WebhookDeliveryPolicy.isRetryable(it))
        }
    }

    @Test
    fun `backoff doubles from two seconds with jitter in the upper half, and is capped`() {
        mapOf(2 to 2_000L, 3 to 4_000L, 4 to 8_000L, 12 to MAX_DELAY_MILLIS).forEach { (attempt, full) ->
            assertEquals("lower $attempt", full / 2, WebhookDeliveryPolicy.delayMillis(attempt, 0, 0.0))
            val upper = WebhookDeliveryPolicy.delayMillis(attempt, 0, 0.999999)
            assertTrue("upper $attempt = $upper", upper in (full - 1)..full)
        }
    }

    @Test
    fun `a longer Retry-After wins but is capped`() {
        assertEquals(30_000L, WebhookDeliveryPolicy.delayMillis(2, 30_000, 0.0))
        assertEquals(4_000L, WebhookDeliveryPolicy.delayMillis(4, 1_000, 0.0))
        assertEquals(MAX_RETRY_AFTER_MILLIS, WebhookDeliveryPolicy.delayMillis(2, 3_600_000, 0.0))
    }

    @Test
    fun `Retry-After parses delta seconds only`() {
        assertEquals(12_000L, WebhookDeliveryPolicy.parseRetryAfterMillis(" 12 "))
        listOf(null, "", "-1", "0", "Wed, 21 Oct 2026 07:28:00 GMT").forEach {
            assertEquals("$it", 0L, WebhookDeliveryPolicy.parseRetryAfterMillis(it))
        }
    }
}
