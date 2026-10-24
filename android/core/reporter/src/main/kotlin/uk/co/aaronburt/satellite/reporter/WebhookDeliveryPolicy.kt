package uk.co.aaronburt.satellite.reporter

/**
 * Webhook delivery recovery policy (Android). See docs/mqtt-and-webhooks.md.
 *
 * Only the newest snapshot is kept, in memory. A failed POST is retried with
 * the original payload (and timestamp) using capped exponential backoff with
 * jitter, until it succeeds, a permanent error comes back, the attempt or age
 * limit is hit, or a newer publish replaces it.
 */
object WebhookDeliveryPolicy {
    const val MAX_ATTEMPTS = 5
    const val BASE_DELAY_MILLIS = 2_000L
    const val MAX_DELAY_MILLIS = 60_000L
    const val MAX_RETRY_AFTER_MILLIS = 5 * 60_000L
    const val MAX_AGE_MILLIS = 5 * 60_000L

    /**
     * [code] is the HTTP status, or null when no response arrived (offline,
     * timeout, connection reset). Network failures, 408, 425, 429 and 5xx are
     * retryable; any other status is permanent.
     */
    fun isRetryable(code: Int?): Boolean = when {
        code == null -> true
        code == 408 || code == 425 || code == 429 -> true
        code >= 500 -> true
        else -> false
    }

    /**
     * Delay before attempt number [nextAttempt] (2 for the first retry).
     * [jitter] is in [0,1); the delay lands in [d/2, d]. A longer server
     * Retry-After wins, capped at [MAX_RETRY_AFTER_MILLIS].
     */
    fun delayMillis(nextAttempt: Int, retryAfterMillis: Long, jitter: Double): Long {
        var d = BASE_DELAY_MILLIS
        var i = 2
        while (i < nextAttempt && d < MAX_DELAY_MILLIS) {
            d *= 2
            i++
        }
        d = d.coerceAtMost(MAX_DELAY_MILLIS)
        d = d / 2 + (jitter * (d / 2)).toLong()
        if (retryAfterMillis > d) d = retryAfterMillis.coerceAtMost(MAX_RETRY_AFTER_MILLIS)
        return d
    }

    /** Parses a delta-seconds Retry-After header; anything else is ignored. */
    fun parseRetryAfterMillis(value: String?): Long =
        value?.trim()?.toLongOrNull()?.takeIf { it > 0 }?.times(1000) ?: 0L
}
