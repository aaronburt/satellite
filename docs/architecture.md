# Architecture

Satellite has two independently built applications in one repository. They share
a release and a purpose, but do not share application code or an identical MQTT
schema. Neither application requires a Satellite cloud service.

## Desktop agent: Windows and Linux

The Go module lives in `windows/`, including Linux implementations. Platform
files use Go build constraints to select operating-system APIs.

| Component | Responsibility |
| --- | --- |
| `main.go` | Startup, collection timer, output wiring, and shutdown |
| `internal/config` | Defaults, persisted JSON, configuration import/export, version |
| `internal/capabilities` | Detect which sensors and controls the host supports |
| `internal/telemetry` | Read OS state, build snapshots, detect significant changes |
| `internal/mqtt` | Connection/retry loop, discovery, state publishing, command subscriptions |
| `internal/actions` | Parse and dispatch a fixed set of device actions |
| `internal/output` | Deliver snapshots to MQTT and the optional telemetry webhook |
| `internal/webui` | Embedded web UI, authenticated management API, optional `/json` endpoint |
| `internal/logger` | Application events, UI log streaming, optional log webhook |
| `internal/toast`, `internal/tray` | Native notifications and the Windows system tray |
| `internal/updater` | Check GitHub releases and surface update information |

### Data flow and lifecycle

Startup loads configuration, detects capabilities, collects an initial snapshot,
and starts the MQTT client. A timer collects subsequent snapshots and passes each
to the configured outputs. MQTT publishes significant changes and heartbeats;
the telemetry webhook has its own change and interval checks. A slow webhook
runs in the background, with one request in flight, so intermediate snapshots
can be skipped. There is no persistent telemetry queue.

MQTT retries failed connections with exponential backoff. On connection it sends
availability, subscribes to enabled command/notification topics, and publishes
discovery. Graceful shutdown attempts to publish `offline`; the broker's retained
Last Will covers unexpected connection loss.

The web UI saves configuration and applies it to the MQTT client, logger, JSON
endpoint, and update checker. The collection timer is created at startup;
changing its interval currently requires restarting the process.

### Run modes

| Mode | Behavior |
| --- | --- |
| Default on Windows | System tray; enable the web UI from the tray menu |
| Default on Linux, or `--headless` | Background agent without a tray; exits on a termination signal |
| `--debug` | Agent plus web UI; prints the token-bearing URL and reads UI HTML from disk when available |
| `--cli` | Local terminal telemetry dashboard; returns before MQTT/webhook startup |
| `--export` / `--import` | Configuration operation followed by exit |

`--debug` is the setup/development mode for Linux. `--headless` alone does not
enable the management UI. The HTML development override is relative to the
`windows/` working directory.

### Configuration and access

Defaults enable CPU, memory, and network telemetry. Remote controls and
notifications default to disabled. Capability detection limits what discovery
and the UI expose, while MQTT command handling also checks the enabled controls.

The HTTP listener defaults to `127.0.0.1` and an automatically selected port. The
web UI and its `/api/*` routes use an ephemeral token. The separately enabled
`/json` snapshot endpoint uses the persistent API key. The two credentials serve
different purposes. See the [HTTP API reference](http-api.md) for endpoints,
authentication, and client examples, or [troubleshooting](troubleshooting.md)
for access problems.

The log webhook and telemetry webhook are separate outputs. Logs contain events;
telemetry contains device-state snapshots.

## Android app

Android uses Kotlin, Compose, Hilt, coroutines, and Preferences DataStore. The
Gradle projects are listed in [`settings.gradle.kts`](../android/settings.gradle.kts).

| Module | Responsibility |
| --- | --- |
| `:app` | Application wiring, navigation, foreground service, boot receiver |
| `:core:common`, `:core:model` | Coroutine dispatchers/scope and shared value types |
| `:core:datastore` | Persist settings and device identity |
| `:core:mqtt` | HiveMQ adapter and MQTT connection lifecycle |
| `:core:telemetry` | Read Android device state and apply audio controls |
| `:core:discovery` | Entity catalog, discovery payloads, topics, webhook JSON |
| `:core:reporter` | Select transport, schedule reporting, observe state changes, route commands |
| `:core:update` | Check, download, verify, and hand APK updates to the installer |
| `:core:designsystem` | Shared Compose styling and components |
| `:feature:dashboard`, `:feature:entities`, `:feature:settings` | User-facing screens and their state |

`SatelliteService` owns the foreground notification and starts
`ReporterCoordinator`. The coordinator observes the reporting switch and transport
setting, then starts either MQTT reporting or webhook reporting. Unlike desktop,
Android selects one transport at a time.

MQTT connection management observes broker settings. The reporter observes
connection state to publish discovery and state, subscribes to Home Assistant's
birth topic, and routes enabled volume/microphone commands. Reporting is periodic
or event-driven with a periodic safety net. Webhook reporting uses the same mode
and interval preferences, with no persistent offline queue.

Controls default to disabled and are checked again by `CommandRouter`. Device
permissions and OS restrictions determine which readings and controls work.
`BootReceiver` starts reporting after reboot only when configured and enabled.

## Compatibility boundaries

Stable device identifiers, discovery unique IDs, topics, entity keys, and JSON
fields are externally consumed interfaces. Changing them can create replacement
Home Assistant entities or break automations. Review both payload builders and
their tests before changing a contract.

The [protocol reference](mqtt-and-webhooks.md) describes the platform differences.
The [contributor guide](../CONTRIBUTING.md) describes builds, checks, and the shared
release process. Current operational caveats are in [troubleshooting](troubleshooting.md).
