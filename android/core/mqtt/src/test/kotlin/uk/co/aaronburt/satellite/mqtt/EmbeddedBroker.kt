package uk.co.aaronburt.satellite.mqtt

import com.hivemq.embedded.EmbeddedExtension
import com.hivemq.embedded.EmbeddedHiveMQ
import com.hivemq.extension.sdk.api.ExtensionMain
import com.hivemq.extension.sdk.api.auth.SimpleAuthenticator
import com.hivemq.extension.sdk.api.packets.auth.DefaultAuthorizationBehaviour
import com.hivemq.extension.sdk.api.parameter.ExtensionStartInput
import com.hivemq.extension.sdk.api.parameter.ExtensionStartOutput
import com.hivemq.extension.sdk.api.parameter.ExtensionStopInput
import com.hivemq.extension.sdk.api.parameter.ExtensionStopOutput
import com.hivemq.extension.sdk.api.services.Services
import java.net.ServerSocket
import java.nio.file.Files
import java.nio.file.Path

/**
 * A real MQTT 5 broker (HiveMQ CE) running in-process on a free local port.
 * Persistence is in memory, so every [start] begins with an empty broker.
 */
class EmbeddedBroker(
    val port: Int = freePort(),
    private val username: String? = null,
    private val password: String = ""
) : AutoCloseable {

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
        val builder = EmbeddedHiveMQ.builder()
            .withConfigurationFolder(root.resolve("conf"))
            .withDataFolder(root.resolve("data"))
            .withExtensionsFolder(root.resolve("extensions"))
        username?.let { builder.withEmbeddedExtension(authentication(it)) }
        val broker = builder.build()
        broker.start().join()
        hivemq = broker
        return this
    }

    private fun authentication(expectedUsername: String): EmbeddedExtension = EmbeddedExtension.builder()
        .withId("test-authentication")
        .withName("Test authentication")
        .withVersion("1.0")
        .withExtensionMain(object : ExtensionMain {
            override fun extensionStart(input: ExtensionStartInput, output: ExtensionStartOutput) {
                Services.securityRegistry().setAuthenticatorProvider {
                    SimpleAuthenticator { authInput, authOutput ->
                        val packet = authInput.connectPacket
                        val suppliedPassword = packet.password
                            .map { Charsets.UTF_8.decode(it.duplicate()).toString() }
                            .orElse(null)
                        if (packet.userName.orElse(null) == expectedUsername && suppliedPassword == password) {
                            authOutput.defaultPermissions.defaultBehaviour = DefaultAuthorizationBehaviour.ALLOW
                            authOutput.authenticateSuccessfully()
                        } else {
                            authOutput.failAuthentication()
                        }
                    }
                }
            }

            override fun extensionStop(input: ExtensionStopInput, output: ExtensionStopOutput) = Unit
        })
        .build()

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
