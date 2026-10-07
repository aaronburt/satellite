package dev.satelliteandroid.model

/**
 * Webhook delivery settings.
 *
 * @property url endpoint that receives the JSON snapshot, e.g.
 * `http://homeassistant.local:8123/api/webhook/<id>`.
 * @property bearerToken optional `Authorization: Bearer <token>` value.
 */
data class WebhookSettings(
    val url: String,
    val bearerToken: String? = null,
)
