package uk.co.aaronburt.satellite.mqtt

import java.io.InputStream
import java.io.OutputStream
import java.net.InetSocketAddress
import java.net.ServerSocket
import java.net.Socket
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.concurrent.thread

/**
 * A TCP relay between the client and the broker that tests can cut. Cutting it
 * drops every open connection without an MQTT DISCONNECT, like a phone losing
 * Wi-Fi: the broker publishes the Last Will and the client starts reconnecting.
 */
class FlakyNetwork(private val brokerPort: Int) : AutoCloseable {

    private val server = ServerSocket().apply { bind(InetSocketAddress("127.0.0.1", 0)) }
    private val sockets = CopyOnWriteArrayList<Socket>()

    val port: Int = server.localPort

    init {
        thread(isDaemon = true, name = "flaky-network") {
            while (!server.isClosed) {
                val inbound = runCatching { server.accept() }.getOrNull() ?: break
                val outbound = runCatching { Socket("127.0.0.1", brokerPort) }.getOrElse {
                    inbound.close()
                    continue
                }
                sockets += inbound
                sockets += outbound
                pump(inbound.getInputStream(), outbound.getOutputStream(), inbound, outbound)
                pump(outbound.getInputStream(), inbound.getOutputStream(), inbound, outbound)
            }
        }
    }

    /** Drops every open connection. New connections are accepted again straight away. */
    fun cut() {
        sockets.forEach { runCatching { it.close() } }
        sockets.clear()
    }

    private fun pump(from: InputStream, to: OutputStream, vararg pair: Socket) {
        thread(isDaemon = true) {
            runCatching { from.copyTo(to) }
            pair.forEach { runCatching { it.close() } }
        }
    }

    override fun close() {
        runCatching { server.close() }
        cut()
    }
}
