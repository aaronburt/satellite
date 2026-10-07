# TODO / Handoff — Satellite (Android)

Pick-up notes for tomorrow. The app is an Android companion for Home Assistant
that reports device state over **MQTT** or a **webhook**.

---

## Where we are

- **M0** foundations done: Gradle + modules, Compose, Hilt, CI.
- **M1** done: broker settings, HiveMQ MQTT 5 client, foreground service
  (`specialUse`), boot restart, availability (LWT) — verified across
  backgrounding and reboot.
- **M2** done: HA MQTT **device discovery** + periodic/event-driven state,
  telemetry (battery, charging, temperature, network, uptime, storage, …).
- **Transport choice** done: MQTT **or** Webhook (JSON POST) with bearer token
  and a "Send test now" button — both verified end to end.
- Settings: System/Light/Dark theme, random-but-editable device name, update
  mode (periodic vs event-driven) + interval, permission onboarding.
- **Release** `v0.1.0` published (signed with the project release keystore).
- Repo is **public**: <https://github.com/aaronburt/SatelliteAndroid>
- **CI is green.**

Verified on a real phone (Galaxy S23 Ultra): device `Satellite-AB68` online,
15 entities discovered, live state publishing.

---

## ⚠️ Do not lose

- `keystore/satellite-release.jks` and `keystore.properties` are **gitignored**.
  Back both up. Without them you cannot ship updates to an existing install.
- The repo is public — **never commit** credentials, LAN IPs, machine paths, or
  personal email. Dev broker credentials are documented in `docker/README.md`.

---

## Next up — in priority order

### 1. M3: controllable entities (biggest feature gap)
Right now everything is read-only. Add command topics so HA can act on the phone.

**Scope decision: minimum viable set first — volume + mic mute.** TTS (`text`)
and restart/re-publish (`button`) deferred to a follow-up.

- [ ] `MqttClient` already supports `subscribe`; add a command router in
      `:core:reporter` that parses `<base>/cmd/<action>`.
- [ ] Entities to add (see `docs/development-plan.md` §4.2):
  - [ ] Media volume — `number` + `command_topic`, publish confirming state
  - [ ] Mic mute — `switch` + `command_topic`
- [ ] Follow-up: Speak text (`text`, Android `TextToSpeech`), restart service /
      re-publish discovery (`button`).
- [ ] Publish a confirming state after each command.
- [ ] Tests: command parsing (unit) + round-trip against Mosquitto.

### 2. Permission-gated sensors
- [ ] Location → `device_tracker` (needs `ACCESS_FINE_LOCATION`)
- [ ] Activity / steps (`ACTIVITY_RECOGNITION`)
- [ ] Bluetooth connected devices (`BLUETOOTH_CONNECT`)
- [ ] Verify Wi-Fi SSID + carrier populate after granting on the phone
      (currently missing on the test phone).

### 3. End-to-end in Home Assistant
- [ ] Onboard the HA test instance (Docker) and add the MQTT integration
      (broker = host LAN IP, port 1883; credentials in `docker/README.md`).
- [ ] Confirm the device + all entities appear, and that availability flips
      offline when the app is killed.
- [ ] Optional nightly CI job asserting discovery against an HA container.

### 4. Robustness / correctness backlog
- [ ] **Encrypt broker credentials at rest** (DataStore is currently plain).
      Jetpack Security is deprecated — use Tink or a Keystore-backed key.
- [ ] Webhook: retry with backoff + optional offline queue (currently a failed
      POST is dropped).
- [ ] Webhook: optional basic auth as an alternative to the bearer token.
- [ ] `DiscoveryPayloadBuilder.SUPPORT_URL` is a placeholder
      (`https://github.com/`) — set it to the real repo URL.
- [ ] Replace deprecated `WifiManager.connectionInfo` with a `NetworkCallback`.
- [ ] Retained-message expiry verification + automatic ghost cleanup.
- [ ] Consider a watchdog (`setAndAllowWhileIdle`) for aggressive OEMs.

### 5. Release engineering
- [x] **Rename the application ID** to `uk.co.aaronburt.satellite` (app code
      `uk.co.aaronburt.satellite.app`). ✅ Done. *(Migration note for existing
      installs still owed before release.)*
- [x] GitHub Actions **release workflow** (`.github/workflows/release.yml`): on
      `v*` tag push, build a signed APK and attach it to a GitHub Release.
      ✅ Done. Required secrets (set): `RELEASE_KEYSTORE_BASE64`,
      `RELEASE_STORE_PASSWORD`, `RELEASE_KEY_ALIAS`, `RELEASE_KEY_PASSWORD`.
- [x] Bump CI actions to current majors — `checkout@v7`, `setup-java@v6`,
      `gradle/actions/setup-gradle@v6`, `upload-artifact@v7`. ✅ Done. *(The old
      `@v5` target was already stale.)*
- [x] Version bumping strategy: tag-driven — `versionName` from the tag,
      `versionCode` = `major*10000 + minor*100 + patch`. ✅ Done.
- [ ] Add detekt / ktlint to CI (skipped in M0).
- [ ] Migration note for existing installs (package/ID change is breaking).
- [ ] F-Droid listing + reproducible build notes (decided distribution).

### 6. Housekeeping
- [ ] Add a **LICENSE** — the repo is public with none (defaults to
      all-rights-reserved). **Decided: GPL-3.0.**
- [ ] Clear the stale emulator ghost topics (`satellite/<old-id>/availability`).
- [ ] Keep `README.md` / `docs/development-plan.md` in sync as things land.

### 7. UI/UX refinement (v2 design — largely built)
Built:
- [x] **4-tab bottom navigation** (Status · Entities · Permissions · Settings) via
      Navigation Compose; single top app bar; system back pops the back stack.
- [x] Design system: full light/dark colour roles, shapes, opt-in dynamic colour,
      plus a semantic `ExtendedColors.success` role for the "connected" pill.
- [x] **Status**: connection hero (state pill / device name / broker / last
      published / entity count), **Reporting master switch**, highlights,
      "Send test now", first-run onboarding.
- [x] **Entities** (new `:feature:entities`): live sensor values mirroring what is
      published, plus read-only Controls.
- [x] **Permissions**: rationale per permission, "X of Y granted" progress,
      usage/notification-access shortcuts removed.
- [x] **Settings**: sections (Identity / Appearance / Reporting / Connection /
      App); Theme control (System/Light/Dark); dev hints behind Developer options;
      version footer.
- [x] **Reporting pause/resume** (see §8) — off stops the service, removes the
      notification and marks the device offline.

Still open:
- [ ] Snackbar instead of inline "Saved." text.
- [ ] Input validation + password reveal toggle.
- [ ] Localize UI strings into `strings.xml` (currently hardcoded in Compose).
- [ ] Per-entity enable/disable.

### 8. Reporting master switch (done)
- [x] `SettingsRepository.reportingEnabled` persisted in DataStore.
- [x] `ReporterCoordinator` stops everything and closes MQTT (publishing retained
      `offline`) when off.
- [x] `SatelliteService` stops itself + cancels the notification; `BootReceiver`
      respects the flag; `MainActivity` starts/stops the service on toggle.
- [x] Fixed: notification was posted via `NotificationManager.notify` and so
      outlived the service — updates now go through `startForeground`, and
      `onDestroy` force-removes it.
- [ ] Wire a Pause action into the ongoing notification.

---

## Decisions (2026-10-07)

1. **License** — **GPL-3.0**. Add `LICENSE` + copyright header policy.
2. **Voice satellite (Wyoming)** — **permanently out of scope**. Remove from the
   roadmap; note the rationale in `docs/development-plan.md`.
3. **Multi-broker / multi-server** — **single broker for v1**; defer.
4. **M3 command scope** — **minimum viable set**: volume + mic mute first; TTS
   and restart/button deferred.
5. **App identity** — keep the name **Satellite**; renamed package from
   `dev.satelliteandroid.*` to **`uk.co.aaronburt.satellite`** (application ID
   `uk.co.aaronburt.satellite`; app code `uk.co.aaronburt.satellite.app`). ⚠️ Was
   breaking for existing installs (new sideload/install identity; HA device_id
   may reset) — plan the migration before shipping over v0.1.0.

---

## Handy commands

```powershell
# Build + test + lint
./gradlew test lint :app:assembleDebug

# Signed release APK (needs keystore.properties present)
./gradlew :app:assembleRelease

# Test environment (Mosquitto + Home Assistant)
cd docker; docker compose up -d       # credentials: see docker/README.md

# Watch everything the app publishes
cd docker
docker compose exec mosquitto mosquitto_sub -h localhost -u <user> -P <pass> -v -t "satellite/#"

# Emulator
& "$env:ANDROID_HOME\emulator\emulator.exe" -avd satellite
```

---

## Map of the code

| Module | Responsibility |
|---|---|
| `:app` | Application, MainActivity, DI, foreground service, boot receiver |
| `:core:model` | `BrokerSettings`, `WebhookSettings`, `ConnectionState`, enums |
| `:core:datastore` | DataStore-backed `SettingsRepository` |
| `:core:mqtt` | `MqttClient` + HiveMQ impl, `MqttConnectionManager` |
| `:core:telemetry` | `DeviceStateReader` (reads the phone) |
| `:core:discovery` | `EntityCatalog`, `Topics`, discovery + webhook payload builders |
| `:core:reporter` | `SatelliteReporter` (MQTT), `WebhookReporter`, `ReporterCoordinator`, `ReporterStatus` |
| `:feature:dashboard` | Status screen (hero, reporting switch, highlights, test) |
| `:feature:entities` | Entities screen (live sensor values, read-only controls) |
| `:feature:settings` | Settings (identity/theme/reporting/connection) + permissions |

**MQTT layout:** `satellite/<deviceId>/availability` · `satellite/<deviceId>/state/<key>`
· discovery at `homeassistant/device/satellite_<deviceId>/config`.

**Webhook payload:** `{ device_id, device_name, manufacturer, model, app_version,
android_version, timestamp, state: { <key>: <value> } }`.
