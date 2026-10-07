# Testing

How the app is tested, from fast JVM checks up to on-device end-to-end runs.

## Test layers

| Layer | Covers | Device? | Command |
|---|---|---|---|
| Unit (JVM) | topic/payload builders, discovery JSON (golden tests), backoff, mappers, ViewModels with fakes | No | `./gradlew test` |
| Static analysis | Android Lint, `lintVitalRelease` | No | `./gradlew lint` |
| MQTT integration (JVM) | real client ↔ Mosquitto: connect, LWT, reconnect, retained/expiry, discovery messages | No | Gradle test against `localhost:1883` *(M1)* |
| Instrumented / UI | Compose UI, Hilt graph, FGS start, permissions | Yes | `./gradlew connectedDebugAndroidTest` |
| End-to-end with Home Assistant | app → Mosquitto → HA auto-creates device + entities | Yes + Docker HA | manual + registry assertions *(M2)* |
| Robustness / behavior | Doze, App Standby, network loss, process death, reboot, battery | Yes | `adb shell …` (below) |
| Release / R8 smoke | minified release APK still works | Yes | install `app-release` |

## Common commands

```powershell
./gradlew test                          # JVM unit tests (all modules)
./gradlew lint                          # Android Lint
./gradlew :app:assembleDebug            # debug APK
./gradlew :app:assembleRelease          # R8 + resource shrinking check
./gradlew :app:installDebug             # install debug on the running device/emulator
./gradlew connectedDebugAndroidTest     # instrumented/UI tests on device
```

Set `JAVA_HOME` and `ANDROID_HOME` (or `local.properties`) first; see the README.

## Emulator

An AVD named `satellite` (API 36, Google APIs, x86_64) is already created.

```powershell
$env:ANDROID_HOME = "$env:LOCALAPPDATA\Android\Sdk"

# start
& "$env:ANDROID_HOME\emulator\emulator.exe" -avd satellite -no-snapshot -no-boot-anim -no-audio

# wait until booted
& "$env:ANDROID_HOME\platform-tools\adb.exe" wait-for-device

# stop
& "$env:ANDROID_HOME\platform-tools\adb.exe" emu kill
```

Requires WHPX/Hyper-V (already available here). Create a new AVD with:

```powershell
avdmanager create avd -n satellite -k "system-images;android-36;google_apis;x86_64"
```

## Talking to the local broker from the emulator

The Docker test environment (Mosquitto + Home Assistant) runs on the host.

- **Emulator** → broker host is `10.0.2.2`, port `1883` (the emulator's alias for the host loopback).
- **Physical device** → use the PC's LAN IP; allow inbound TCP `1883` in Windows Firewall.
- **Credentials** → `satellite` / `satellite`.

Watch everything on the broker:

```powershell
cd docker
docker compose exec mosquitto mosquitto_sub -h localhost -u satellite -P satellite -v -t "homeassistant/#" -t "satellite/#"
```

## Robustness / behavior tests (device)

```powershell
$adb = "$env:ANDROID_HOME\platform-tools\adb.exe"
$pkg = "dev.satelliteandroid.app"

# Doze
& $adb shell dumpsys deviceidle force-idle
& $adb shell dumpsys deviceidle unforce

# App Standby
& $adb shell dumpsys battery unplug
& $adb shell am set-inactive $pkg true
& $adb shell am set-inactive $pkg false

# network transitions
& $adb shell svc wifi disable
& $adb shell svc wifi enable

# process death / reboot
& $adb shell am kill $pkg
& $adb reboot
```

These are the checks that will validate the foreground-service design in M1/M5.

## CI

- Unit tests + lint + debug assemble on every push/PR (`.github/workflows/ci.yml`).
- Instrumented tests via Gradle-managed devices (planned).
- MQTT integration via a Mosquitto service container (planned for M1).
- Discovery golden tests run as plain JVM tests, so HA compatibility regressions fail fast in CI.

## Coverage today

- ✅ Unit tests, Android Lint, debug + release builds.
- ✅ Compose UI instrumented test (`DashboardScreenTest`) on the emulator.
- ✅ App installs and launches on the emulator.
- ⏳ MQTT integration tests, discovery golden tests, HA end-to-end, robustness scripts (M1+).
