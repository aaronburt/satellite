# Satellite

An Android companion app that presents itself to Home Assistant over MQTT:
auto-registration via **MQTT Discovery** (device + entities), periodic and
event-driven state reporting, and a foreground service that stays connected.

**Status:** M0 complete (build + CI); M1 complete — the app connects to MQTT, publishes availability, and stays connected via a foreground service across backgrounding and reboot. Home Assistant discovery is next.

## Building

Requires JDK 17+ (JDK 21 used during setup) and the Android SDK
(platform `android-37.2`, build-tools `37.0.0`). Point `local.properties` or
`ANDROID_HOME` at the SDK.

```powershell
./gradlew :app:assembleDebug   # debug APK
./gradlew test lint            # unit tests + Android Lint
```

The APK is written to `app/build/outputs/apk/debug/app-debug.apk`.

### Installing on another phone

```powershell
./gradlew :app:assembleRelease
```

Output: `app/build/outputs/apk/release/app-release.apk` — release build (R8),
signed with the local debug keystore for dev sideloading. Copy it to the phone,
enable *install unknown apps*, and install.

On a physical phone, set the **broker host to the PC's LAN IP** (not `10.0.2.2`)
and make sure the phone is on the same network and the firewall allows inbound
`1883` (MQTT) / `8123` (Home Assistant).

## Documentation

- [Research notes](docs/research.md) — HA MQTT Discovery, MQTT client options for
  Android, platform constraints (foreground services, Doze, Play), references.
- [Development plan](docs/development-plan.md) — architecture, MQTT interface
  design, milestones M0–M5, testing strategy, risks, open questions.
- [Testing](docs/testing.md) — test layers, emulator setup, broker connectivity,
  robustness checks.
- [Probeable device data](docs/sensors.md) — what the app can read from the
  phone and how it maps to HA entities.

## Development test environment

Local Mosquitto broker + Home Assistant in Docker for validating discovery
behavior end to end:

```powershell
cd docker
docker compose up -d
```

Then follow [docker/README.md](docker/README.md) for Home Assistant onboarding
and the discovery smoke test.

## Confirmed decisions

| Decision | Choice |
|---|---|
| v1 scope | MQTT only (no voice satellite) |
| Distribution | GitHub Releases + F-Droid (Google Play optional later) |
| Stack | Kotlin 2.4.20 + Jetpack Compose (BOM 2026.09.00), Hilt 2.60.1, AGP 9.4.1 (built-in Kotlin) + Gradle 9.6.0 |
| Android SDK | minSdk 26 · targetSdk 36 · compileSdk 37.2 |
| MQTT + storage | HiveMQ MQTT client, DataStore + Room (M1) |
| HA integration | MQTT device discovery, LWT availability, birth-message republish |

See [development-plan.md §2](docs/development-plan.md) for the full decision
table and rationale.
