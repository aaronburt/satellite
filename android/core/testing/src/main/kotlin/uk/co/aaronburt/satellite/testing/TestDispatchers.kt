package uk.co.aaronburt.satellite.testing

import uk.co.aaronburt.satellite.common.coroutines.DispatchersProvider
import kotlinx.coroutines.CoroutineDispatcher

/** Routes every dispatcher to one test dispatcher so virtual time controls it all. */
class TestDispatchers(dispatcher: CoroutineDispatcher) : DispatchersProvider {
    override val default: CoroutineDispatcher = dispatcher
    override val io: CoroutineDispatcher = dispatcher
    override val main: CoroutineDispatcher = dispatcher
}
