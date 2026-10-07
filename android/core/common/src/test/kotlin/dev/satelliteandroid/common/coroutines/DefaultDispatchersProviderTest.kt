package dev.satelliteandroid.common.coroutines

import kotlinx.coroutines.Dispatchers
import org.junit.Assert.assertSame
import org.junit.Test

class DefaultDispatchersProviderTest {

    @Test
    fun `exposes the standard coroutine dispatchers`() {
        val provider = DefaultDispatchersProvider()

        assertSame(Dispatchers.Default, provider.default)
        assertSame(Dispatchers.IO, provider.io)
    }
}
