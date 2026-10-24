package uk.co.aaronburt.satellite.mqtt

import com.hivemq.embedded.EmbeddedHiveMQ
import java.net.ServerSocket
import java.nio.file.Files
import java.nio.file.Path

/**
 * A real MQTT 5 broker (HiveMQ CE) running in-process on a free local port.
 * Persistence is in memory, so every [start] begins with an empty broker.
 */
class EmbeddedBroker(val port: Int = freePort()) : AutoCloseable {

    private val root: Path = Files.createTempDirectory("hivemq-test")
    private var hivemq: EmbeddedHiveMQ? = null

    init {
        val config = Files.createDirectories(root.resolve("conf"))
        Files.writeString(
            config.resolve("config.xml"),
            """
            <?xml version="1.0"?>
            <hivemq>
              <listeners>
                <tcp-listener>
                  <port>$port</port>
                  <bind-address>127.0.0.1</bind-address>
                </tcp-listener>
              </listeners>
              <persistence>
                <mode>in-memory</mode>
              </persistence>
              <anonymous-usage-statistics>
                <enabled>false</enabled>
              </anonymous-usage-statistics>
            </hivemq>
            """.trimIndent(),
        )
        Files.createDirectories(root.resolve("data"))
        Files.createDirectories(root.resolve("extensions"))
    }

    fun start(): EmbeddedBroker {
        val broker = EmbeddedHiveMQ.builder()
            .withConfigurationFolder(root.resolve("conf"))
            .withDataFolder(root.resolve("data"))
            .withExtensionsFolder(root.resolve("extensions"))
            .build()
        broker.start().join()
        hivemq = broker
        return this
    }

    fun stop() {
        hivemq?.let { broker ->
            broker.stop().join()
            broker.close()
        }
        hivemq = null
    }

    override fun close() {
        stop()
        root.toFile().deleteRecursively()
    }

    companion object {
        fun freePort(): Int = ServerSocket(0).use { it.localPort }
    }
}
