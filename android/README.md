# Satellite for Android

An Android companion app that reports device state to Home Assistant over MQTT,
with device discovery, periodic or event-driven updates, and optional audio
controls. It can alternatively send telemetry to an HTTP webhook.

[Documentation index](../docs/README.md) | [Architecture](../docs/architecture.md) |
[Protocol reference](../docs/mqtt-and-webhooks.md) | [Troubleshooting](../docs/troubleshooting.md) |
[Privacy](../PRIVACY.md)

## Install and configure

1. Download the `.apk` from a [Satellite release](https://github.com/aaronburt/satellite/releases)
   and install it. Android may ask you to allow installation from that source.
2. Open the app and choose MQTT or Webhook in Settings. These are alternative
   transports; selecting Webhook stops the MQTT connection.
3. For MQTT, enter the broker host, port, credentials, and TLS choice. Configure
   Home Assistant's MQTT integration to use the same broker. For a webhook,
   enter its URL and optional bearer token.
4. Set the device name, reporting mode, and interval. Enable reporting on the
   Status screen. Event-driven mode also keeps periodic reporting as a safety net.
5. Grant the permissions needed for the readings you want. In Entities, explicitly
   enable supported media-volume or microphone-mute controls if needed; controls
   are disabled by default.

On a physical phone, use the broker's LAN address rather than `localhost` or
`10.0.2.2`. The standard Android emulator uses `10.0.2.2` to reach the development
host. The foreground notification indicates reporting is running; pausing
reporting stops the service. A configured, enabled installation can restart
reporting after reboot.

The app includes an APK update flow using GitHub Releases. Updates must match the
installed application's ID and signing key. The application ID is
`uk.co.aaronburt.satellite`.

## Build from source

Use JDK 21 to match CI, the committed Gradle wrapper, and the Android SDK.
Set `ANDROID_HOME` or create an untracked `local.properties` with `sdk.dir`.
The app currently declares compile SDK 37, target SDK 36, and minimum SDK 26.
CI installs `platforms;android-37.2` and `build-tools;37.0.0`; consult
[`app/build.gradle.kts`](app/build.gradle.kts) and
[CI](../.github/workflows/android-ci.yml) when matching SDK packages.

Run from `android/` on Windows:

```powershell
.\gradlew.bat :app:assembleDebug
.\gradlew.bat test lint
```

On Linux/macOS, substitute `./gradlew` for `.\gradlew.bat`.
The debug APK is written to `app/build/outputs/apk/debug/app-debug.apk`.

For a minified release build:

```powershell
.\gradlew.bat :app:assembleRelease
```

The APK is written under `app/build/outputs/apk/release/`. Without
`keystore.properties`, this build uses the local Android debug keystore, which
must already exist (a debug build creates it). Official releases use a separate
release key, so a development-signed APK cannot update an official installation.

## Testing and releases

See [Android testing](docs/testing.md) for JVM checks, device tests, and broker
integration prerequisites. The [Docker environment](docker/README.md) supplies a
local Mosquitto broker and Home Assistant for integration checks.

The shared [release workflow](../.github/workflows/release.yml) reads the version
from `windows/internal/config/config.go` and builds the signed APK alongside the
desktop artifacts. Local builds use the Android Gradle defaults unless version
properties are supplied. See [Contributing](../CONTRIBUTING.md) for versioning,
signing secrets, and release steps.

Dependency versions are maintained in [`libs.versions.toml`](gradle/libs.versions.toml).
The [development plan](docs/development-plan.md), [research](docs/research.md),
[sensor catalog](docs/sensors.md), and [handoff notes](TODO.md) retain historical
and proposed work; they are not the current feature reference.
