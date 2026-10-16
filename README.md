# Satellite

Turn your devices into Home Assistant sensors and controls over MQTT.

This is a monorepo containing the Satellite apps:

| App | Path | Description |
| --- | --- | --- |
| Windows & Linux | [`windows/`](windows/) | Lightweight telemetry agent and remote controller. Written in Go. |
| Android | [`android/`](android/) | Companion app with MQTT Discovery and a foreground service. Written in Kotlin. |

## Screenshots

The Windows & Linux agent's local web UI:

| Processes | Performance | Settings |
| --- | --- | --- |
| ![Processes view](windows/docs/screenshots/processes.jpg) | ![Performance view](windows/docs/screenshots/performance.jpg) | ![Settings view](windows/docs/screenshots/settings.jpg) |

Each app has its own README with build and setup instructions:

- [`windows/README.md`](windows/README.md)
- [`android/README.md`](android/README.md)

## Releases

Each app is released independently by a workflow in [`.github/workflows/`](.github/workflows/):

| App | Trigger | Tag |
| --- | --- | --- |
| Windows & Linux | Bump `Version` in `windows/internal/config/config.go` and push to `main` | `vX.Y.Z` |
| Android | Push a tag | `android-vX.Y.Z` |

Android releases are not marked as the repo's "latest" release, so the Windows app's update checker keeps seeing Windows releases.

## Disclaimer

Written with AI, reviewed by a human.
