# Android testing

Run build commands from `android/`. Use JDK 21 to match CI and configure the Android
SDK as described in the [app guide](../README.md). Windows examples use the Gradle
batch wrapper; on Linux/macOS use `./gradlew` instead.

## Automated checks currently present

| Layer | Existing coverage | Prerequisites |
| --- | --- | --- |
| JVM unit tests | Dispatcher provider, discovery/webhook payload builders, command parser, GitHub release parser | JDK, SDK, Gradle dependencies |
| Static analysis | Android Lint | JDK, SDK, Gradle dependencies |
| Dashboard instrumented tests | Connected, paused, and first-run UI states in `StatusScreenTest` | Device/emulator |
| Broker configuration E2E | `BrokerConfigurationE2ETest` drives settings and checks connection to Mosquitto | API 33+ emulator, development broker, fresh app setup |

Connection-manager and reporter lifecycle behavior does not currently have
comprehensive automated unit coverage. Broker configuration E2E does not prove
Last Will, reconnect recovery, command round trips, or Home Assistant entity
creation. Those require the manual checks below.

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
   proof of complete recovery.
5. Restart Home Assistant and verify its `online` birth triggers discovery/state.
6. Switch to Webhook. Verify MQTT disconnects and the receiver gets JSON with
   the selected settings. Test a failed request as well as a 2xx response.
7. Background the app and reboot the device with reporting enabled, then repeat
   with reporting paused. Confirm the service respects the saved reporting switch.

See [current limitations](../../docs/troubleshooting.md) when assessing Android
reconnect availability and last-published status. Use the
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
