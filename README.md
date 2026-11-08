# Satellite

Turn your devices into Home Assistant sensors and controls over MQTT.

This is a monorepo containing the Satellite apps:

| App | Path | Description |
| --- | --- | --- |
| Windows & Linux | [`windows/`](windows/) | Lightweight telemetry agent and remote controller. Written in Go. |
| Android | [`android/`](android/) | Companion app with MQTT Discovery and a foreground service. Written in Kotlin. |

## Desktop web UI

The Windows & Linux agent provides a local web UI with five tabs:

| Tab | Purpose |
| --- | --- |
| Sensors | Live readings, processor/memory/network/GPU history graphs, threshold highlights and optional screen previews. |
| Privacy | Saved permissions, privacy-pause status, reporting destinations, allowed media commands and the current JSON dataset. |
| Diagnostics | MQTT and telemetry webhook health, connection tests and recent failures. |
| Logs | Live log events with search and severity filtering. |
| Settings | Reporting, sensors, warning/critical thresholds, remote controls, local access and maintenance. |

Performance graphs are part of Sensors. Thresholds highlight the web UI only and are disabled by default. See [desktop monitoring and privacy](windows/README.md#web-ui-monitoring-and-privacy) for details.

Each app has its own README with build and setup instructions:

- [`windows/README.md`](windows/README.md)
- [`android/README.md`](android/README.md)

## Documentation

Start with the [documentation index](docs/README.md) for setup and integration guides.

- [GitHub issues](https://github.com/aaronburt/satellite/issues): prioritized fixes, verification work, and feature candidates.
- [Architecture](docs/architecture.md): components, data flow, and lifecycle.
- [HTTP API](docs/http-api.md): desktop endpoint access, authentication, and PowerShell/Go examples.
- [MQTT and webhooks](docs/mqtt-and-webhooks.md): topics, commands, payloads, and platform differences.
- [Troubleshooting](docs/troubleshooting.md): connectivity, discovery, reporting, and current limitations.
- [Contributing](CONTRIBUTING.md): development, testing, versioning, and releases.
- [Privacy statement](PRIVACY.md): what is shared, where it goes, and your controls.
- [Security policy](SECURITY.md): private vulnerability reporting.

## Releases

Builds and releases follow the folders changed on `main`:

| Changed files | Workflow | Release assets |
| --- | --- | --- |
| `windows/**` | [Desktop release](.github/workflows/desktop-release.yml) | `satellite.exe`, `Satellite-Setup-vX.Y.exe`, `satellite-linux-amd64` |
| `android/**` | [Android release](.github/workflows/android-release.yml) | Signed `satellite-android-vX.Y.apk` |

Changes to both folders release both platforms independently. Markdown and platform
`docs/` changes do not publish releases. Each release workflow also watches its own
file and the shared version scripts.

Versions advance independently using `desktop-vMAJOR.MINOR` and
`android-vMAJOR.MINOR` tags. Each app checks its own releases for updates.
A one-time [bridge release](.github/workflows/release.yml) publishes the updated
updaters together before the independent streams begin. Install that release to
receive subsequent platform-specific updates.

See [CONTRIBUTING.md](CONTRIBUTING.md#releases) for versioning, migration, and signing.

## Disclaimer

Written with AI, reviewed by a human.
