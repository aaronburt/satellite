# Android testing

Run build commands from `android/`. Use JDK 21 to match CI and configure the Android
SDK as described in the [app guide](../README.md). Windows examples use the Gradle
batch wrapper; on Linux/macOS use `./gradlew` instead.

## Automated checks currently present

| Layer | Existing coverage | Prerequisites |
| --- | --- | --- |
| JVM unit tests | Every module: settings storage on a real DataStore file, discovery/webhook payload builders, command parsing and routing, telemetry readers, dispatchers, view models, boot policy, and notification text | JDK, SDK, Gradle dependencies |
| Shared fixtures | `:core:testing` (test-only): `FakeMqttClient`, `FakeSettingsRepository`, `MainDispatcherRule`, test dispatchers/scopes, and Android fixtures | Used by the module tests |
| Embedded-broker tests | `HiveMqttClientTest` runs the real HiveMQ client against an in-process broker (`EmbeddedBroker`); `MqttConnectionManagerTest` covers Last Will, `online` after automatic reconnects, stop/start races, and settings changes | JDK only; no Docker |
| Reporter tests | `SatelliteReporterTest`, `WebhookReporterTest`, `ReporterCoordinatorTest`, `ReporterStatusTest`, `StateChangeWatcherTest`, `TelemetryContractTest`: MQTT and webhook delivery end to end, publish-result reporting, transport switching, and that every telemetry key is a declared entity | JDK only |
| Robolectric tests | Android-API code: telemetry readers, audio controls (including offline microphone restore), reporter/watcher tests, view models, and service notification | JDK, SDK |
| View-model tests | `StatusViewModelTest`, `EntitiesViewModelTest`, `SettingsViewModelTest` | JDK, SDK |
| Update-flow tests | `:core:update`: release parsing, version codes, check, HTTPS download (truncation, partial-file cleanup), checksum/signing-key verification, install hand-off, and duplicate-download handling | JDK only |
| Static analysis | Android Lint | JDK, SDK, Gradle dependencies |
| Dashboard instrumented tests | Connected, paused, and first-run UI states in `StatusScreenTest` | Device/emulator |
| MQTT reconnect recovery | `ReconnectRecoveryTest` runs the real client and connection manager against an in-process HiveMQ broker: network loss (Last Will then retained `online`), broker restart with empty state, a command handled once after three reconnects, clean stop leaves `offline` | JDK, Gradle dependencies (runs in CI) |
| Broker configuration E2E | `BrokerConfigurationE2ETest` drives settings and checks connection to Mosquitto | API 33+ emulator, development broker, fresh app setup |

Unit tests do not prove real-device lifecycle behavior (foreground service,
Doze, OEM background limits, reboot) or Home Assistant integration (entity
creation, birth-message refresh, command round trips through a real broker
and Home Assistant). Those still require the manual checks below.

## Build and unit checks

```powershell
.\gradlew.bat test
.\gradlew.bat lint
.\gradlew.bat :app:assembleDebug
.\gradlew.bat :app:assembleRelease
```

The release build checks R8 and resource shrinking. Local signing differs from
GitHub release signing; see [Contributing](../../CONTRIBUTING.md).

[Android CI](../../.github/workflows/android-ci.yml) runs unit tests, lint, and
assembleDebug on relevant pushes and pull requests. It uploads the debug APK.
Device tests and Home Assistant end-to-end checks are not run by that workflow.

## Device and emulator tests

Create an Android emulator in Android Studio, or connect a phone with USB
debugging enabled. Check `adb devices` before running:

```powershell
.\gradlew.bat :app:installDebug
.\gradlew.bat connectedDebugAndroidTest
```

The full device test run includes a non-hermetic broker setup test. It assumes
an API 33+ emulator (including the notification permission), broker host
`10.0.2.2`, port `1883`, and development credentials `satellite` / `satellite`.
Start the [Docker environment](../docker/README.md) first. Run against a fresh
emulator/install so the dashboard shows the initial configuration action; saved
settings can change that flow. This test writes broker settings to the test app.
A physical phone needs an adapted host address.

To run only dashboard rendering tests, without the broker configuration test:

```powershell
.\gradlew.bat :feature:dashboard:connectedDebugAndroidTest
```

## Verify MQTT and Home Assistant manually

From `android/docker/`, watch discovery and state traffic:

```powershell
docker compose exec mosquitto mosquitto_sub -h localhost -u satellite -P satellite -v -t "homeassistant/#" -t "satellite/#"
```

Use a development device and broker, then check:

1. Configure MQTT and enable reporting. Verify retained discovery, `online`
   availability, state topics, and the device/entities in Home Assistant.
2. Enable volume or microphone control. Send a command from Home Assistant and
   compare the phone's observed state with the confirming MQTT state. Disable
   the control and verify subsequent commands are ignored.
3. Pause reporting: verify `offline` availability and removal of the foreground
   notification. Resume and verify discovery/state plus `online` availability.
4. Interrupt the network or restart the broker, then restore it. Verify state,
   subscriptions, and availability separately; connection status alone is not
   proof of complete recovery. `ReconnectRecoveryTest` covers the MQTT side of
   this automatically; the device run is still needed for Android's own network
   handling (Wi-Fi off/on, Doze) and is tracked in
   [issue #8](https://github.com/aaronburt/satellite/issues/8).
5. Restart Home Assistant (`docker compose restart homeassistant`). Verify the
   device and entities return with current values without touching the phone
   (the reporter republishes discovery and state on HA's `online` birth
   message).
6. Toggle Wi-Fi off and on three times, then send one volume command from Home
   Assistant. It must change the volume once (one confirming state message).

Record the app version, Android version, broker and Home Assistant versions,
and the observed messages from the `mosquitto_sub` window in issue #8.
5. Restart Home Assistant and verify its `online` birth triggers discovery/state.
6. Switch to Webhook. Verify MQTT disconnects and the receiver gets JSON with
   the selected settings. Test a failed request as well as a 2xx response.
7. Background the app and reboot the device with reporting enabled, then repeat
   with reporting paused. Confirm the service respects the saved reporting switch.

In step 4, builds from current `main` should republish retained `online` after
an automatic reconnect, and the last-published time and test result reflect
broker acceptance. The published v0.26.0 APK predates these fixes; see
[troubleshooting](../../docs/troubleshooting.md). Use the
[protocol reference](../../docs/mqtt-and-webhooks.md) for exact topics and payloads.

## Power and network behavior

On a development emulator/device, use the following commands to exercise Doze,
App Standby, and network transitions. These change the device's test state; run
all corresponding restore commands when finished.

```powershell
$adb = Join-Path $env:ANDROID_HOME 'platform-tools/adb.exe'
$pkg = 'uk.co.aaronburt.satellite'

& $adb shell dumpsys deviceidle force-idle
& $adb shell dumpsys deviceidle unforce

& $adb shell dumpsys battery unplug
& $adb shell am set-inactive $pkg true
& $adb shell am set-inactive $pkg false
& $adb shell dumpsys battery reset

& $adb shell svc wifi disable
& $adb shell svc wifi enable
```

Record device model, Android version, build type, permissions, reporting mode,
and broker observations with results. OEM background restrictions differ, so an
emulator pass does not establish behavior on every phone. Test the minified
release build on a device as well as the debug build.

## Lifecycle and microphone device checklist

Robolectric covers the logic (boot policy, coordinator transport switching,
offline microphone restore in `SatelliteReporterTest`). Only a phone can show
how Android and the OEM treat the foreground service. Record the device model,
Android version, and OEM battery setting for each run; never record credentials.

| # | Steps | Expected |
| --- | --- | --- |
| 1 | Reporting on, reboot the phone, unlock | Notification returns; broker/webhook receives state within one interval |
| 2 | Pause reporting, reboot | No notification, no traffic, switch still paused |
| 3 | Background the app for 30 min with the screen off | Notification stays; periodic updates keep arriving |
| 4 | `adb shell dumpsys deviceidle force-idle`, wait, then `adb shell dumpsys deviceidle unforce` | Reporting resumes after Doze ends; note any gap |
| 5 | Toggle pause/resume ten times quickly, then rotate the screen and reopen the app | One notification; one message per interval (no duplicates); after the last pause the notification is gone |
| 6 | Stop the broker, enable reporting, start the broker | Status recovers to connected without reopening the app |
| 7 | Switch MQTT to Webhook and back while running | Only the selected transport sends; MQTT shows `offline` availability while on Webhook |
| 8 | Enable the microphone control, mute from Home Assistant, turn on airplane mode, disable the control in the app | Microphone is unmuted (check in a recorder app) even though offline |
| 9 | After each step | Saved switch, notification text, actual traffic, and device state agree |
