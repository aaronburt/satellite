package uk.co.aaronburt.satellite.reporter

import android.app.Application
import android.media.AudioManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.json.JSONArray
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import uk.co.aaronburt.satellite.discovery.EntityCatalog
import uk.co.aaronburt.satellite.discovery.EntityCatalog.CONTROL_VOLUME
import uk.co.aaronburt.satellite.discovery.Topics
import uk.co.aaronburt.satellite.model.BrokerSettings
import uk.co.aaronburt.satellite.model.ConnectionState
import uk.co.aaronburt.satellite.mqtt.HiveMqttClient
import uk.co.aaronburt.satellite.mqtt.MqttConnectionManager
import uk.co.aaronburt.satellite.telemetry.AudioControls
import uk.co.aaronburt.satellite.telemetry.DeviceStateReader
import uk.co.aaronburt.satellite.testing.FakeSettingsRepository
import uk.co.aaronburt.satellite.testing.TestDispatchers
import uk.co.aaronburt.satellite.testing.setBattery
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration
import java.util.UUID

/**
 * Issue #21: the real reporter, MQTT client and connection manager against a
 * real Mosquitto and a real Home Assistant. Checks what Home Assistant itself
 * makes of the app's discovery, availability, state and commands.
 *
 * Skipped unless SATELLITE_HA_URL is set. Run it with
 * `android/docker/smoke/run.sh`, which starts an isolated broker and Home
 * Assistant, onboards it and supplies the environment below.
 *
 * Every test uses a fresh device id and name and deletes its device again, so
 * runs never see each other's entities.
 */
@RunWith(RobolectricTestRunner::class)
class HomeAssistantSmokeTest {

    private val env = System.getenv()
    private val haUrl: String? = env["SATELLITE_HA_URL"]

    private val context: Application = RuntimeEnvironment.getApplication()
    private val audio = AudioControls(context)
    private val suffix = UUID.randomUUID().toString().replace("-", "").take(8)
    private val deviceId = "smoke$suffix"
    private val deviceName = "Satellite Smoke $suffix"
    private val topics = Topics(deviceId)

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private lateinit var broker: BrokerSettings
    private lateinit var settings: FakeSettingsRepository
    private lateinit var client: HiveMqttClient
    private lateinit var manager: MqttConnectionManager
    private lateinit var ha: HomeAssistantApi

    @Before
    fun setUp() {
        assumeTrue("SATELLITE_HA_URL not set; run android/docker/smoke/run.sh", haUrl != null)
        ha = HomeAssistantApi(haUrl!!, env.getValue("SATELLITE_HA_TOKEN"))
        broker = BrokerSettings(
            host = env.getValue("SATELLITE_MQTT_HOST"),
            port = env.getValue("SATELLITE_MQTT_PORT").toInt(),
            username = env["SATELLITE_MQTT_USERNAME"],
            password = env["SATELLITE_MQTT_PASSWORD"],
            useTls = false,
        )
        context.setBattery(level = 64)
        context.getSystemService(AudioManager::class.java).setStreamVolume(AudioManager.STREAM_MUSIC, 0, 0)
    }

    @After
    fun tearDown() = runBlocking {
        if (haUrl == null) return@runBlocking
        if (::manager.isInitialized) {
            manager.stop()
            client.disconnect()
        }
        scope.cancel()
        removeDevice()
    }

    private fun startApp(enabledControls: Set<String>) = runBlocking {
        settings = FakeSettingsRepository(
            broker = broker,
            enabledControls = enabledControls,
            deviceIdValue = deviceId,
            deviceName = deviceName,
        )
        client = HiveMqttClient(TestDispatchers(Dispatchers.IO))
        manager = MqttConnectionManager(client, settings, scope)
        SatelliteReporter(
            client = client,
            settingsRepository = settings,
            telemetry = DeviceStateReader(context, audio),
            commandRouter = CommandRouter(settings, audio),
            audioControls = audio,
            status = ReporterStatus(),
            context = context,
            appScope = scope,
        ).start()
        manager.start()
        withTimeout(30_000) { client.connectionState.first { it is ConnectionState.Connected } }
    }

    /** Entity ids Home Assistant has registered for this run's device. */
    private fun deviceEntities(): List<String> {
        val ids = ha.template(
            "{{ integration_entities('mqtt') | select('is_device_attr', 'name', '$deviceName') | list | tojson }}",
        )
        return JSONArray(ids).let { array -> List(array.length()) { array.getString(it) } }
    }

    /** friendly_name -> state object for this run's device. */
    private fun deviceStates(): Map<String, JSONObject> {
        val ids = deviceEntities().toSet()
        return ha.states()
            .filter { it.getString("entity_id") in ids }
            .associateBy { it.getJSONObject("attributes").optString("friendly_name") }
    }

    private fun entityFor(name: String): JSONObject? = deviceStates()["$deviceName $name"]

    @Test
    fun `Home Assistant creates the device with every read-only entity and its state`() {
        startApp(enabledControls = emptySet())

        eventually("every read-only entity is created and online") {
            val states = deviceStates()
            EntityCatalog.entities.forEach { spec ->
                val state = states["$deviceName ${spec.name}"]
                assertTrue("missing ${spec.key}; HA has ${states.keys}", state != null)
                assertTrue("${spec.key} is unavailable", state!!.getString("state") != "unavailable")
            }
        }
        eventually("battery state reaches Home Assistant") {
            val battery = EntityCatalog.entities.single { it.key == "battery" }
            assertEquals("64", entityFor(battery.name)!!.getString("state"))
        }
    }

    @Test
    fun `controls that are not enabled never appear in Home Assistant`() {
        startApp(enabledControls = emptySet())

        eventually("device is discovered") { assertTrue(deviceEntities().isNotEmpty()) }
        val controlNames = EntityCatalog.controlEntities.map { "$deviceName ${it.name}" }
        val entities = deviceEntities()
        assertFalse("a control was exposed: $entities", entities.any { it.startsWith("number.") || it.startsWith("switch.") })
        assertTrue(deviceStates().keys.none { it in controlNames })
    }

    @Test
    fun `volume set from Home Assistant reaches the phone and the new state comes back`() {
        startApp(enabledControls = setOf(CONTROL_VOLUME))
        val volumeName = EntityCatalog.controlEntities.single { it.key == CONTROL_VOLUME }.name

        val entityId = eventually("volume entity is created") {
            entityFor(volumeName)!!.getString("entity_id")
        }

        ha.callService("number", "set_value", JSONObject().put("entity_id", entityId).put("value", 40))

        eventually("the phone applies the volume") { assertEquals(40, audio.mediaVolumePercent()) }
        eventually("Home Assistant shows the volume the phone reports") {
            assertEquals(40.0, entityFor(volumeName)!!.getString("state").toDouble(), 0.01)
        }
    }

    @Test
    fun `turning a control off removes it from Home Assistant`() {
        startApp(enabledControls = setOf(CONTROL_VOLUME))
        val volumeName = EntityCatalog.controlEntities.single { it.key == CONTROL_VOLUME }.name
        eventually("volume entity is created") { assertTrue(entityFor(volumeName) != null) }

        runBlocking { settings.setControlEnabled(CONTROL_VOLUME, false) }

        eventually("volume entity is removed") {
            assertFalse(deviceEntities().any { it.startsWith("number.") })
        }
        // Removing a control must not take the rest of the device with it.
        assertTrue("battery entity was removed too", entityFor("Battery") != null)
    }

    @Test
    fun `entities go unavailable when the app stops reporting`() {
        startApp(enabledControls = emptySet())
        eventually("device is online") {
            assertTrue(deviceStates().isNotEmpty())
            assertTrue(deviceStates().values.none { it.getString("state") == "unavailable" })
        }

        manager.stop()

        eventually("every entity is unavailable") {
            val states = deviceStates()
            assertTrue(states.isNotEmpty())
            states.forEach { (name, state) -> assertEquals(name, "unavailable", state.getString("state")) }
        }
    }

    /** Clears every retained topic this run wrote; an empty discovery deletes the device in HA. */
    private suspend fun removeDevice() {
        val cleaner = HiveMqttClient(TestDispatchers(Dispatchers.IO))
        cleaner.connect(broker, "satellite-smoke-cleanup-$suffix", "satellite-smoke/$suffix/cleanup").getOrThrow()
        cleaner.publish(topics.discovery, "", qos = 1, retain = true)
        cleaner.publish(topics.availability, "", qos = 1, retain = true)
        EntityCatalog.all.forEach { cleaner.publish(topics.state(it.key), "", qos = 1, retain = true) }
        cleaner.disconnect()
        runCatching {
            eventually("device is deleted from Home Assistant") { assertTrue(deviceEntities().isEmpty()) }
        }
    }

    /** Retries [block] until it stops throwing [AssertionError], or fails with the last one. */
    private fun <T> eventually(what: String, timeoutMillis: Long = 60_000, block: () -> T): T {
        val deadline = System.currentTimeMillis() + timeoutMillis
        var last: Throwable? = null
        while (System.currentTimeMillis() < deadline) {
            try {
                return block()
            } catch (e: AssertionError) {
                last = e
            } catch (e: NullPointerException) {
                last = e
            }
            Thread.sleep(500)
        }
        throw AssertionError("Timed out waiting until $what", last)
    }

    /** The small slice of Home Assistant's REST API the smoke test needs. */
    private class HomeAssistantApi(private val baseUrl: String, private val token: String) {
        private val http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build()

        fun states(): List<JSONObject> =
            JSONArray(send(request("/api/states").GET())).let { array -> List(array.length()) { array.getJSONObject(it) } }

        fun template(template: String): String =
            send(request("/api/template").POST(body(JSONObject().put("template", template))))

        fun callService(domain: String, service: String, data: JSONObject) {
            send(request("/api/services/$domain/$service").POST(body(data)))
        }

        private fun request(path: String) = HttpRequest.newBuilder(URI.create(baseUrl + path))
            .timeout(Duration.ofSeconds(15))
            .header("Authorization", "Bearer $token")
            .header("Content-Type", "application/json")

        private fun body(json: JSONObject) = HttpRequest.BodyPublishers.ofString(json.toString())

        private fun send(builder: HttpRequest.Builder): String {
            val request = builder.build()
            val response = http.send(request, HttpResponse.BodyHandlers.ofString())
            check(response.statusCode() in 200..299) {
                "${request.method()} ${request.uri().path} -> ${response.statusCode()}: ${response.body()}"
            }
            return response.body()
        }
    }
}
