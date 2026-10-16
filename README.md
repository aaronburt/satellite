# Satellite

Turn your devices into Home Assistant sensors and controls over MQTT.

This is a monorepo containing the Satellite apps:

| App | Path | Description |
| --- | --- | --- |
| Windows & Linux | [`windows/`](windows/) | Lightweight telemetry agent and remote controller. Written in Go. |
| Android | [`android/`](android/) | Companion app with MQTT Discovery and a foreground service. Written in Kotlin. |

Each app has its own README with build and setup instructions:

- [`windows/README.md`](windows/README.md)
- [`android/README.md`](android/README.md)

These were previously developed as separate repositories,
[satellite-windows](https://github.com/aaronburt/satellite-windows) and
[satellite-android](https://github.com/aaronburt/satellite-android). Their full commit history is preserved here.
