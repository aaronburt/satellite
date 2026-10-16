package uk.co.aaronburt.satellite.model

/**
 * Base publish interval, used by [UpdateMode.PERIODIC] and as the safety net in
 * [UpdateMode.EVENT_DRIVEN].
 */
enum class UpdateInterval(val seconds: Long) {
    THIRTY_SECONDS(30),
    ONE_MINUTE(60),
    FIVE_MINUTES(300),
    FIFTEEN_MINUTES(900),
}
