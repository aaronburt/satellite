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
| Home Assistant smoke test | `HomeAssistantSmokeTest` runs the real reporter, client and connection manager against an isolated Mosquitto and Home Assistant: device and read-only entities created with state, disabled controls never exposed, a volume command round trip from Home Assistant, a control removed when turned off, and entities unavailable after stopping | Docker, `jq`, JDK, SDK; run `docker/smoke/run.sh` (CI: [HA smoke workflow](../../.github/workflows/ha-smoke.yml)) |
| Broker configuration E2E | `BrokerConfigurationE2ETest` drives settings and checks connection to Mosquitto | API 33+ emulator, development broker, fresh app setup |

Unit tests do not prove real-device lifecycle behavior (foreground service,
Doze, OEM background limits, reboot). The Home Assistant smoke test covers
entity creation and a command round trip, but runs under Robolectric rather
than on a phone, so the manual checks below still apply before a release.

`HiveMqttClientTest` also uses a broker that requires a username and password.
It checks that missing or incorrect credentials are rejected, then restarts the
broker twice and verifies automatic reconnection, state delivery, and one command
delivery per publish. This catches credentials configured only for the initial
connection instead of being retained in the client's reconnect defaults.

## Home Assistant smoke test

Needs Docker (with Compose), `jq`, `curl`, and the JDK/SDK used for unit tests.
From the repository root:

```bash
android/docker/smoke/run.sh
```

The script starts its own Mosquitto and Home Assistant under a unique Compose
project, on random loopback ports and throwaway volumes, so it never touches
your normal `docker/` environment or Home Assistant data. It onboards Home
Assistant and adds the MQTT integration through Home Assistant's API, runs
`HomeAssistantSmokeTest`, then removes everything. Each test uses a fresh
device id and deletes its device afterwards.

Evidence is written to `android/build/ha-smoke/` on every run: broker and Home
Assistant logs, retained broker topics, Home Assistant states and error log,
and the MQTT setup responses. Options: `HA_VERSION=2026.10.3` pins a Home
Assistant release, `HA_IMAGE=homeassistant/home-assistant` pulls from Docker
Hub instead of GHCR, and `KEEP_STACK=1` leaves the stack running and prints
its credentials for debugging. Without the script's environment the test is
skipped, so `gradlew test` is unaffected.

In CI the [HA smoke workflow](../../.github/workflows/ha-smoke.yml) runs on
pull requests touching discovery, reporting, MQTT, telemetry or `docker/`, and
on manual dispatch (with a Home Assistant version input). Failed runs upload
the evidence as the `ha-smoke-evidence` artifact.

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
Device tests are not run by that workflow. The Home Assistant smoke test and the
on-demand emulator device checks (see below) each have their own workflow.

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

## Automated emulator run

The [Android device checks](../../.github/workflows/android-device-checks.yml)
workflow runs most of the checklists below on an emulator. Start it from the
Actions tab ("Run workflow"); you can pick the checks, the API level and the
length of the background check. It also runs on pull requests that change the
checks. Results go to the job summary, and logs, logcat and broker output are
uploaded as an artifact.

The driver is [`scripts/device-checks.sh`](../scripts/device-checks.sh). It
starts a throwaway Mosquitto container, changes settings through the debug-only
`DeviceCheckReceiver`, and judges each step by the MQTT traffic, the service and
notification state, and the audio state. Locally, with an emulator running and
Docker available:

```bash
./gradlew :app:installDebug
scripts/device-checks.sh                    # everything
scripts/device-checks.sh doze broker_restart  # selected checks
```

| Checklist step | Automated check | How |
| --- | --- | --- |
| #9-1 reboot with reporting on | `reboot_reporting_on` | `adb reboot`; service, notification and state within one interval |
| #9-2 reboot while paused | `reboot_reporting_paused` | no service, notification or traffic; switch still paused |
| #9-3 background, screen off | `background_screen_off` | 5 min by default (`BACKGROUND_MINUTES`), not 30 |
| #9-4 / #8-4 Doze | `doze` | battery unplug, `deviceidle force-idle`, then unforce; state must resume |
| #9-5 rapid pause/resume + rotation | `rapid_toggle_and_rotate` | 10 toggles, rotation, reopen; one notification, no duplicate publishes, clean final pause |
| #9-6 broker down at start | `broker_down_at_start` | broker stopped, reporting enabled, broker started; `online` and "Connected" notification |
| #8-4 broker restart | `broker_restart` | `docker restart`; `online` and state flowing |
| #8-4 / #8-6 network loss, one command | `network_loss_then_one_command` | Wi-Fi and mobile data off/on 3 times, then one volume command: exactly one confirming message |
| #8-5 Home Assistant restart | `ha_birth_republishes` | publishes HA's `homeassistant/status` birth; discovery must be republished |
| #9-7 MQTT ↔ Webhook | `transport_switch` | local webhook receiver; MQTT `offline` and silent while on Webhook, clean switch back |
| #9-8 microphone restore offline | `microphone_restore_offline` | mute via MQTT, airplane mode, disable control; mic must unmute. Skipped if the image refuses app mic mute |

Not covered by the emulator, so still manual:

- OEM battery managers and background limits; an emulator is stock Android.
- A real Home Assistant instance: entity creation in the UI, and commands sent
  from HA rather than published directly. The run uses HA's birth message instead.
- The minified release build (the hook only exists in debug builds).
- The 30-minute background soak, unless you set `background_minutes` to 30.
- A lock screen with a PIN (`BOOT_COMPLETED` before unlock) and real Wi-Fi radios.
- Webhook failure responses and checking the recorded audio in a recorder app.

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
