# Privacy implementation review

Reviewed on 24 October 2026 against source commit `ef0e8d26b9da`, app version
`0.27.0`, to support [PRIVACY.md](../PRIVACY.md).

This review traced runtime network requests, configuration defaults, reporting
payloads, permissions, storage, and update entry points in the source. The
developer's description and historical planning comments were not treated as
proof of behavior.

## Findings and evidence

| Claim | Evidence and qualifications |
| --- | --- |
| Device data is not sent to a developer collection service | The desktop network paths are the configured [MQTT broker](../windows/internal/mqtt/client.go), [telemetry webhook](../windows/internal/output/webhook.go), [log webhook](../windows/internal/logger/logger.go), and optional [GitHub release checker](../windows/internal/updater/updater.go). Android uses the configured [broker](../android/core/mqtt/src/main/kotlin/uk/co/aaronburt/satellite/mqtt/HiveMqttClient.kt), [webhook](../android/core/reporter/src/main/kotlin/uk/co/aaronburt/satellite/reporter/WebhookReporter.kt), and explicit [release check](../android/core/update/src/main/kotlin/uk/co/aaronburt/satellite/update/UpdateChecker.kt)/[APK download](../android/core/update/src/main/kotlin/uk/co/aaronburt/satellite/update/UpdateDownloader.kt). No additional developer collection destination was found. |
| No built-in advertising, usage analytics, or crash upload | No such SDK or collection integration was found in runtime source, [Go dependencies](../windows/go.mod), or [Android dependency declarations](../android/gradle/libs.versions.toml) and module build files. Android's app initialization has no collection hook. |
| GitHub polling is off by default | Desktop [defaults](../windows/internal/config/config.go) set `CheckUpdates: false`; [startup](../windows/main.go) applies that setting before starting the background checker, whose requests are gated by `IsEnabled()`. The explicit manual check is separate. Android's only runtime caller of `checkAndDownload()` is the [Settings action](../android/feature/settings/src/main/kotlin/uk/co/aaronburt/satellite/feature/settings/SettingsViewModel.kt). A comment in `UpdateManager` mentions a Home Assistant trigger, but no such call is implemented. |
| Update requests have limited contents | The desktop checker sends an HTTP GET with `Satellite-Agent/<version>` and an Accept header. Android sends an HTTP GET with `SatelliteAndroid` and an Accept header. Neither adds sensor data, credentials, or installation IDs. Android's [update manager](../android/core/update/src/main/kotlin/uk/co/aaronburt/satellite/update/UpdateManager.kt) automatically downloads a newer APK after an explicit check, then waits for installation approval. |
| Users choose destinations, with different sensor controls on each platform | Desktop [defaults](../windows/internal/config/config.go) have an empty broker, disabled webhooks, CPU/memory/network enabled, and other exposures disabled. [Collection](../windows/internal/telemetry/collector.go) gates readings by those switches, with battery presence and timestamps always included. Android's [settings store](../android/core/datastore/src/main/kotlin/uk/co/aaronburt/satellite/datastore/DataStoreSettingsRepository.kt) starts without a destination but defaults reporting to true. Its [catalog](../android/core/discovery/src/main/kotlin/uk/co/aaronburt/satellite/discovery/EntityCatalog.kt) always includes read-only sensors; only controls have individual opt-ins. |
| Reports can identify devices | Desktop IDs default to hostnames. The [desktop webhook envelope](../windows/internal/output/webhook.go) independently reads the actual hostname. Android [discovery](../android/core/discovery/src/main/kotlin/uk/co/aaronburt/satellite/discovery/DiscoveryPayloadBuilder.kt) and [webhooks](../android/core/discovery/src/main/kotlin/uk/co/aaronburt/satellite/discovery/WebhookPayloadBuilder.kt) include the generated ID, name, manufacturer, model, and software versions. |
| Camera/microphone features report status rather than recordings | The [Windows readers](../windows/internal/telemetry/platform_windows.go) inspect OS status/registry entries; the [Linux readers](../windows/internal/telemetry/platform_linux.go) inspect device usage. Android [audio controls](../android/core/telemetry/src/main/kotlin/uk/co/aaronburt/satellite/telemetry/AudioControls.kt) read/change volume or mute state. The [Android state reader](../android/core/telemetry/src/main/kotlin/uk/co/aaronburt/satellite/telemetry/DeviceStateReader.kt) uses location permission only for Wi-Fi SSID, with no GPS-reading path. |
| Local storage protections differ | Desktop [config writes](../windows/internal/config/config.go) save JSON containing secrets; [exports](../windows/internal/config/export.go) encrypt only with a passphrase. Android [settings](../android/core/datastore/src/main/kotlin/uk/co/aaronburt/satellite/datastore/DataStoreSettingsRepository.kt) use [Keystore encryption](../android/core/datastore/src/main/kotlin/uk/co/aaronburt/satellite/datastore/KeystoreSecretCipher.kt), with a [legacy migration](../android/core/datastore/src/main/kotlin/uk/co/aaronburt/satellite/datastore/LegacySecretMigration.kt). Its [manifest](../android/app/src/main/AndroidManifest.xml) disables app backup. |
| Stopping reporting does not erase recipients' copies | Desktop MQTT retains discovery/availability but not state snapshots. [Android reporting](../android/core/reporter/src/main/kotlin/uk/co/aaronburt/satellite/reporter/SatelliteReporter.kt) retains discovery/state with expiry values; broker and Home Assistant storage remain separate. The [coordinator](../android/core/reporter/src/main/kotlin/uk/co/aaronburt/satellite/reporter/ReporterCoordinator.kt) stops normal reporting when paused. There is no general deletion of receiver-side history on pause or uninstall. |

## Other behavior relevant to privacy

- The [desktop web server](../windows/internal/webui/server.go) defaults to a
  loopback listener, uses a session token for its web API, and requires an API key
  for the optional JSON endpoint. Its [HTML](../windows/internal/webui/static/index.html)
  has embedded styles/scripts and no external font, script, or analytics fetch.
- The [log webhook](../windows/internal/logger/logger.go) can forward incoming
  notification payloads and remote commands as well as connection messages.
  It is distinct from the sensor webhook and is disabled by default.
- [Windows notification delivery](../windows/internal/toast/toast_windows.go)
  opens an accepted web URL automatically after showing a notification. A browser
  can therefore contact another service as a result of an enabled MQTT
  notification or an update notification. This is disclosed in the statement.
- Both desktop local-IP readers call `net.Dial("udp", "8.8.8.8:80")`, inspect the
  socket's local address, then close it. They do not write application data to
  that socket. This runs only with the local-IP sensor enabled. The statement
  does not claim that the app never opens any third-party-addressed socket.
- Transport encryption is conditional. Desktop supports plaintext MQTT/HTTP and
  an option to skip MQTT certificate verification. Android defaults MQTT to TLS
  but allows an explicit plaintext choice; its [HTTP policy](../android/app/src/main/res/xml/network_security_config.xml)
  permits cleartext only for specified development hosts.

## Verification and limits

The existing desktop tests passed on Windows with:

```powershell
go test -count=1 ./internal/config ./internal/updater ./internal/output ./internal/logger ./internal/mqtt ./internal/webui
```

These include default settings, disabled background checks, disabled webhook
delivery, actual payloads against local test receivers, remote-command gating,
authentication, and secret redaction. They run with isolated test settings.

Android behavior was checked in source; its tests were not run because this
workspace has no configured Android SDK. Linux-specific readers were reviewed
in source, not executed. This was not a packet-capture test of released binaries
or a full audit of every transitive dependency. It cannot establish what a
third-party receiver retains, what the operating system does independently, or
how someone handles information voluntarily sent outside the app.

Recheck the statement when reporting, permissions, dependencies, network paths,
defaults, or storage behavior changes.
