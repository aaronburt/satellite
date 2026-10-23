package uk.co.aaronburt.satellite.testing

import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent

/**
 * Runs everything due within the next [millis] of virtual time, including work
 * launched in `backgroundScope` (which `advanceUntilIdle` skips). Code under
 * test that loops forever on a timer is fine: only [millis] are simulated.
 */
@OptIn(ExperimentalCoroutinesApi::class)
fun TestScope.settle(millis: Long = 1_000) {
    advanceTimeBy(millis)
    runCurrent()
}
