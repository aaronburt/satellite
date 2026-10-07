# SatelliteAndroid — Development Plan (v0.1)

Companion to [`docs/research.md`](research.md). This plan assumes the app is a **Home Assistant companion for Android** that self-registers and reports via **MQTT Discovery**.

**Confirmed decisions (2026-10-07):**
- v1 is **MQTT only** — no voice satellite features.
- Distribution via **GitHub Releases + F-Droid** (Google Play optional later).
- The **Docker test environment** (Mosquitto + Home Assistant) is set up first, before M0. See [`docker/README.md`](../docker/README.md).

**Toolchain (implemented in M0, 2026-10-07):** AGP 9.4.1 (built-in Kotlin) · Gradle 9.6.0 · Kotlin 2.4.20 · KSP 2.3.12 · Hilt 2.60.1 · Compose BOM 2026.09.00 · JDK 21 · compileSdk 37.2 / targetSdk 36 / minSdk 26.

---

## 1. Product definition

**Goal:** an Android app that can run unattended on phones/tablets and present itself to Home Assistant as a first-class device over MQTT, with no YAML on the HA side.

**Must have (v1):**
- Configure broker (host/port/TLS/credentials) and device identity/name/area in-app.
- Connect over MQTT 5 (fallback 3.1.1), with LWT availability and auto-reconnect.
- Publish HA device discovery + diagnostic entities (battery, charging, app/Android version, network, uptime, power state).
- Publish retained state; republish on HA birth (`homeassistant/status`).
- Accept commands: notify, TTS speak, volume, mute, restart, refresh.
- Foreground service that survives backgrounding, reboots, and network changes.
- In-app diagnostics: connection state, last error, message log, battery-optimization checklist.
- Update entity backed by GitHub Releases + a signed release pipeline.

**Must have (release):**
- F-Droid listing and reproducible build notes (distribution decision 2026-10-07).

**Nice to have (v1.x):**
- Additional sensors (memory, storage, SSID, IP, doze/interactive/power-save, mic state).
- device_tracker (optional location).
- Multi-broker support.
- MQTT event entities for satellite events.

**Out of scope for v1 (decided 2026-10-07):**
- Wyoming-protocol voice satellite (wake word, mic streaming, HA Assist pipeline). Kept in the research notes for a possible future track.
- Google Play distribution (optional later).
- Cloud dependency, FCM push, HACS installation (HACS cannot install Android apps — see research §1).

---

## 2. Proposed decisions (needs owner confirmation)

| # | Decision | Proposal | Why |
|---|---|---|---|
| D1 | Language/UI | Kotlin + Jetpack Compose (Material 3) | Google's recommended modern stack. |
| D2 | Architecture | Layered (UI/domain/data), MVVM + UDF, single Activity | `developer.android.com/topic/architecture/recommendations`. |
| D3 | DI | Hilt | Recommended for multi-screen + WorkManager apps. |
| D4 | MQTT client | HiveMQ MQTT Client behind our own `MqttClient` interface | MQTT 5, official Android support, active maintenance. |
| D5 | Persistence | DataStore (settings) + Room (event log / publish queue) | Standard modern stack. |
| D6 | Secrets | Google Tink (`tink-android`) for broker credentials | EncryptedSharedPreferences is deprecated. |
| D7 | Background model | Foreground service, `specialUse` type + boot receiver + watchdog alarm | Persistent socket; not `dataSync` (6 h cap). |
| D8 | Min/target SDK | **Implemented:** minSdk 26, targetSdk 36, compileSdk 37.2 | Play requires targetSdk 36 from 2026-08-31; current stable AndroidX requires compileSdk 37, so compileSdk is 37.2 while targetSdk stays 36. |
| D9 | Discovery format | HA **device discovery** (`homeassistant/device/...`), retained + birth-triggered | One message, device info once, HA-recommended. |
| D10 | Distribution | **Confirmed:** GitHub Releases + F-Droid first; Play later (optional) | Avoids Play policy risk for `specialUse`/battery exemptions; HA users expect sideload/F-Droid. |
| D11 | Voice | **Confirmed out of scope for v1** | Owner decision 2026-10-07; MQTT and Wyoming are orthogonal, avoid scope creep. |
| D12 | License | TBD (Apache-2.0 suggested) | Needed before publishing. |

---

## 3. Architecture

### 3.1 Layers

```
UI (Compose) ── ViewModel (StateFlow) ── Domain (use cases, models)
                                          │
                     ┌────────────────────┼─────────────────────┐
                     ▼                    ▼                     ▼
              core:telemetry        core:discovery         core:mqtt
              (Android APIs)        (topic+payload)        (client + supervisor)
                     │                    │                     │
                     └────────── core:data (DataStore, Room, Tink) ──┘
                                          │
                                   core:service (FGS, boot, watchdog)
```

- **Single source of truth:** the `ConnectionSupervisor` exposes `StateFlow<ConnectionState>`; telemetry exposes `Flow<DeviceState>`; UI and publishers both consume these.
- **Unidirectional data flow:** UI sends intents to ViewModels; everything else reacts to flows.
- **No Android framework types in ViewModels.**

### 3.2 Key components

| Component | Responsibility |
|---|---|
| `MqttClient` (interface) | `connect`, `disconnect`, `publish(topic, payload, qos, retain, expiry)`, `subscribe(filter): Flow<MqttMessage>`, `connectionState: StateFlow<ConnectionState>`. |
| `HiveMqMqttClient` | Implementation; maps HiveMQ callbacks to our flows. |
| `ConnectionSupervisor` | Owns connect/disconnect lifecycle; exponential backoff with jitter (1s → 5min cap); immediate retry on network-change callback; exposes state. |
| `NetworkMonitor` | `ConnectivityManager.registerDefaultNetworkCallback`; emits connectivity changes. |
| `DiscoveryPublisher` | Builds and publishes the device discovery payload; republishes on connect and HA birth; handles component add/remove. |
| `EntityCatalog` | Declarative list of supported entities (id, platform, device class, category, state key, command key, enabled-by-default). Single source for discovery + state + commands. |
| `StatePublisher` | Maps `DeviceState` to state topics; retained + message expiry; coalesces rapid updates. |
| `CommandRouter` | Subscribes to command topics; validates and dispatches to handlers (notify, TTS, volume, mute, restart, install_update). |
| `SatelliteService` (FGS) | Hosts the supervisor; notification shows connection state + actions (reconnect, stop); START_STICKY. |
| `BootReceiver` | Starts the service after reboot when enabled. |
| `Watchdog` | Periodic `setAndAllowWhileIdle` alarm; verifies service liveness; restarts if needed. |
| `DeviceStateProviders` | Battery, charging, power save, doze, interactive, network transport, SSID/IP, memory, storage, uptime, app version, TTS/volume state. |

---

## 4. MQTT interface design

### 4.1 Topics

Let `<base>` = `satellite/<device_id>` (configurable prefix, default `satellite`).

| Purpose | Topic | Retain | QoS | Expiry |
|---|---|---|---|---|
| Availability (LWT + birth) | `<base>/availability` | yes | 1 | none |
| State | `<base>/state/<key>` | yes | 0/1 | 1 h (MQTT 5) |
| Commands | `<base>/cmd/<action>` | no | 1 | n/a |
| Discovery | `homeassistant/device/satellite_<device_id>/config` | yes | 1 | 7 d |
| HA birth subscription | `homeassistant/status` | — | 0 | — |

### 4.2 Entity catalog v1

> Full catalog of everything the phone can expose (and what needs permissions):
> [`docs/sensors.md`](sensors.md).

| Key | Platform | Device class | Category | State source | Command |
|---|---|---|---|---|---|
| `battery` | sensor | `battery` (%) | diagnostic | BatteryManager | — |
| `battery_temperature` | sensor | `temperature` | diagnostic | BatteryManager | — |
| `charging` | binary_sensor | `battery_charging` | diagnostic | BatteryManager | — |
| `power_save` | binary_sensor | `power` | diagnostic | PowerManager | — |
| `doze` | binary_sensor | — | diagnostic | PowerManager.isDeviceIdleMode | — |
| `interactive` | binary_sensor | — | diagnostic | PowerManager.isInteractive | — |
| `app_version` | sensor | — | diagnostic | PackageInfo | — |
| `android_version` | sensor | — | diagnostic | Build.VERSION | — |
| `uptime` | sensor | `duration` (s) | diagnostic | SystemClock.elapsedRealtime | — |
| `network_transport` | sensor | `enum` | diagnostic | ConnectivityManager | — |
| `ip_address` | sensor | — | diagnostic | LinkProperties | — |
| `wifi_ssid` | sensor | — | diagnostic | WifiManager (permission-gated) | — |
| `memory_usage` | sensor | — | diagnostic | Runtime | — |
| `volume_media` | number | — | — | AudioManager | `<base>/cmd/volume` |
| `mic_muted` | switch | — | — | AudioManager | `<base>/cmd/mute` |
| `tts` | text | — | — | TextToSpeech | `<base>/cmd/tts` |
| `notify` | notify | — | — | — | `<base>/cmd/notify` |
| `restart` | button | — | diagnostic | — | `<base>/cmd/restart` |
| `refresh` | button | — | diagnostic | — | `<base>/cmd/refresh` |
| `update` | update | `firmware` | diagnostic | GitHub releases API | `<base>/cmd/install_update` |

### 4.3 Payload conventions

- State: plain scalar strings (`42`, `ON`, `wifi`) except `update`, which uses JSON:
  ```json
  {"installed_version":"0.1.0","latest_version":"0.2.0",
   "title":"SatelliteAndroid 0.2.0","release_url":"https://github.com/.../releases/tag/v0.2.0"}
  ```
- Commands: JSON where structured (`notify`: `{"title":"...","message":"..."}`, `tts`: `{"text":"..."}`), plain payloads for simple ones (`ON`/`OFF`, numbers).
- All command handlers publish a confirming state update after execution.

### 4.4 Identity

- `device_id`: 8-char lowercase hex derived from `sha256(Settings.Secure.ANDROID_ID + packageName)`, persisted; stable across reinstalls of the same user/device.
- `unique_id` per entity: `<device_id>_<key>`; never changes after first release (registry stability).
- `device.name`: random default (`Satellite-XXXX`) generated on first launch, editable in the settings screen, persisted in DataStore.
- `suggested_area`: from settings (not yet implemented).
- Do not use MAC/serial (restricted/randomized on modern Android).

---

## 5. Module layout

Start with this split (can collapse early modules if it slows M0):

```
:app                  — Application, MainActivity, Hilt wiring, navigation
:core:common          — dispatchers, Result, logging, extension utils
:core:model           — domain models (DeviceState, ConnectionState, EntitySpec, ...)
:core:datastore       — settings (DataStore), secrets (Tink)
:core:database        — Room: event log, pending publish queue
:core:mqtt            — MqttClient, HiveMQ impl, ConnectionSupervisor, NetworkMonitor
:core:discovery       — topic builder, payload builder, EntityCatalog, DiscoveryPublisher, StatePublisher, CommandRouter
:core:telemetry       — device state providers
:core:service         — SatelliteService (FGS), BootReceiver, Watchdog, notifications
:core:designsystem    — theme, shared composables
:feature:onboarding   — broker setup, permissions, battery checklist
:feature:dashboard    — connection status, diagnostics, quick actions
:feature:settings     — device identity, entity toggles, advanced (topics, QoS, expiry)
```

Rules: `feature:*` depends on `core:*`; `core:*` never depends on `feature:*`; `:app` wires everything. No feature depends on another feature.

---

## 6. Milestones

### M0 — Project foundation — ✅ done (2026-10-07)
- Gradle/Kotlin DSL + version catalog; Compose; Hilt; minSdk 26 / targetSdk 36 / compileSdk 37.2.
- Modules: `:app`, `:core:common`, `:core:model`, `:core:designsystem`, `:feature:dashboard`.
- Base theme, placeholder dashboard, Hilt wiring, launcher icon, CI (build + lint + unit tests).
- **Verified locally:** `:app:assembleDebug`, `test`, and `lint` all pass; debug APK ≈ 12 MB.
- **Deferred:** detekt and the license decision (still open).

### M1 — MQTT connectivity core — 🟡 M1a + M1b (service) done (2026-10-07)

**Done:**
- `MqttClient` interface + HiveMQ 1.4 implementation (MQTT 5, packaging/ProGuard rules).
- Broker settings screen + DataStore persistence; device id generated on first run.
- Idempotent `MqttConnectionManager` connects on save; `StateFlow<ConnectionState>` drives the dashboard.
- LWT + availability verified end-to-end (`satellite/<id>/availability` online → offline).
- Foreground service (`specialUse`) + low-importance notification; verified to survive backgrounding and to auto-start after reboot (via `BootReceiver`, only when a broker is configured).
- On-device E2E test `BrokerConfigurationE2ETest` drives the real UI and asserts connection.

**Remaining (M1c):**
- Encrypt broker credentials at rest (Tink) — currently plain DataStore.
- TLS with a custom/self-signed CA; "test connection" action; reconnect/backoff unit tests.
- Adopt retained **message expiry** to avoid stale-availability ghosts (also M2).
- Battery-optimization onboarding for OEM battery killers.

### M2 — HA self-reporting (discovery + periodic state) — ✅ done (2026-10-07)

**Done:**
- `:core:discovery`: `EntitySpec`, `EntityCatalog` (15 no-permission entities), `Topics`, `DiscoveryPayloadBuilder` with golden unit tests.
- Device discovery payload published retained (7-day expiry) with device + origin blocks and all components.
- `:core:telemetry`: `DeviceStateReader` reads battery/charging/charger type/battery temperature, power-save/doze/interactive, app + Android version, uptime, network transport, storage, ringer mode; permission-gated Wi-Fi SSID and carrier when granted.
- `:core:reporter`: `SatelliteReporter` publishes discovery + state on connect, re-publishes on the HA birth message (`homeassistant/status`), and re-publishes state every **5 minutes** (states retained with 1-hour expiry).
- MQTT client gained `subscribe` + message-expiry support.
- Verified end-to-end against Docker Mosquitto: discovery topic + all state topics + availability.
- `PermissionsScreen`: per-permission Grant buttons for the device-state runtime set, plus special-access shortcuts (usage access, notification access, battery optimization).

**Also done (2026-10-07):**
- **Transport choice**: **MQTT or Webhook** — user-selectable, switched live by `ReporterCoordinator`. Webhook POSTs a JSON snapshot (device metadata + full state) on the same periodic/event-driven schedule, with an optional **bearer token** (`Authorization: Bearer …`) and a **Send test now** button. Verified end-to-end (payload + auth header captured); switching back to MQTT reconnects and restores availability.
- User-selectable **update mode** (Periodic vs Event-driven) and **publish interval** (30 s / 1 min / 5 min / 15 min), persisted and applied live. Event-driven publishes immediately on battery/power/connectivity/screen broadcasts (2 s debounce), with the interval as a safety net.
- **Dark theme** support (System / Light / Dark).
- App renamed to **Satellite** (launcher label, notification, dashboard title, discovery origin).
- Device name: random default (`Satellite-XXXX`), editable.
- Permissions onboarding screen.
- Robustness fix: telemetry reads are isolated per section; `ACCESS_WIFI_STATE` declared.

**Remaining (M3):**
- Controllable entities (volume `number`, mic mute `switch`, TTS `text`) and command handling.
- Permission-gated sensors beyond Wi-Fi SSID/carrier (location → `device_tracker`, activity/steps, Bluetooth).
- Retained-message expiry verification and ghost cleanup automation.

### M3 — Commands & controls (≈1 week)
- CommandRouter + handlers: notify, TTS, volume, mute, restart, refresh.
- `number`, `switch`, `text`, `notify`, `button` entities with confirmation state.
- Settings for per-entity enable/disable (discovery add/remove semantics: empty payload).
- **Exit:** every command round-trips from HA UI; disabled entities are removed from HA; E2E test against Mosquitto.

### M4 — Update entity + release pipeline (3–5 days)
- GitHub Releases checker (cached, rate-limit-aware), update JSON state, install action opens release URL.
- Release workflow: signed APK/AAB, versionCode from CI, changelog, GitHub Release artifact.
- **Exit:** tagged release produces signed artifact; HA shows "update available" when a newer release exists.

### M5 — Hardening & soak (1–2 weeks)
- Doze/OEM matrix: Pixel + Samsung + Xiaomi at minimum; force-idle and App Standby tests.
- Battery profiling (Battery Historian); tune keepalive/QoS/expiry; optional keepalive setting.
- R8 full mode, baseline profiles, macrobenchmark startup.
- Diagnostics: redacted log export, message counters, last-error UI, battery-optimization checklist with per-OEM guidance.
- **Exit:** 72 h soak on ≥2 devices with no availability flaps; documented battery drain; all critical flows covered by tests.

### M6 — Voice satellite (future track — out of scope for v1)
Kept for reference only; owner decided v1 is MQTT-only.
- Evaluate existing native Wyoming implementations before writing one.
- Wyoming protocol module + mic FGS (`microphone` type) + wake word (on-device or server) + HA Assist pipeline.
- **Exit:** wake word → Assist response on a wall tablet; MQTT status entities reflect voice state.

---

## 7. Testing strategy

| Level | What | Tooling |
|---|---|---|
| Unit | Payload/topic builders (golden JSON), backoff, state mappers, command parsing | JUnit5 + Turbine + fakes (prefer fakes over mocks) |
| Integration | Connect/publish/subscribe/LWT/birth against a real broker | Mosquitto in Docker (testcontainers or CI service) |
| E2E (nightly) | Full discovery → HA consumer validation | Mosquitto + a small Python/HA-container consumer asserting device/entity creation |
| UI | Onboarding, settings, dashboard states | Compose UI tests |
| Device | Doze (`adb shell dumpsys deviceidle force-idle`), App Standby, network transitions, process death, reboot | Manual/scripted matrix + Macrobenchmark |

CI gates: unit tests, lint, detekt, discovery golden tests. Integration tests run on PRs touching `core:mqtt`/`core:discovery`.

---

## 8. CI/CD & release

- **GitHub Actions:** `build` (assembleDebug, lint, detekt, unit tests) on every PR; `integration` (Mosquitto service) on relevant paths; `release` on `v*` tags (sign with repository secrets, create GitHub Release).
- **Versioning:** semver tags; `versionCode` = `major*10000 + minor*100 + patch` from CI.
- **R8 full mode + baseline profiles** in release builds; keep rules for Netty/Tink verified by an instrumented smoke test on release builds.
- **F-Droid:** metadata + reproducible build notes once stable (no Play services dependency — we should avoid Firebase entirely).
- **No telemetry by default.** Diagnostics stay local; optional user-initiated log export only.

---

## 9. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| OEM kills the service despite FGS | App silently offline | Watchdog + boot receiver + onboarding checklist + per-OEM docs; consider optional `REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` where policy allows. |
| Doze drops the socket | Stale entities | Availability via LWT makes staleness visible; short state expiry; watchdog; test matrix. |
| Play policy rejects `specialUse` / battery exemption | Distribution blocked | Ship via GitHub/F-Droid first; Play is optional (D10). |
| HiveMQ/Netty issues on some devices or with R8 | Crashes/connection failures | `MqttClient` abstraction allows swap; release-build smoke tests; ProGuard rules verified. |
| Ghost entities in HA | Confusing UX | Retained discovery with 7-day expiry + empty-payload removal + `refresh`/`remove` support. |
| Background mic restrictions (voice track) | Wake word won't autostart | Design for user-initiated start; document; revisit with platform updates. |
| Secret handling mistakes | Credential leak | Tink-encrypted storage, redacted logs, no defaults, cleartext off by default. |
| HA discovery spec evolution | Breakage | Golden tests + nightly E2E against current HA; pin behavior to documented spec. |

---

## 10. Open questions (owner input needed)

1. ~~Scope~~ — **answered 2026-10-07:** MQTT only, no voice in v1.
2. ~~Distribution~~ — **answered 2026-10-07:** GitHub Releases + F-Droid; Play optional later.
3. **Target devices:** phones, wall-mounted tablets, Android TV? Any minimum Android version requirement from the devices you own?
4. **Broker setup:** is TLS with a self-signed cert the expected default (home Mosquitto)? Should we support username/password only as a fallback?
5. **Branding:** app name, package ID, repo/license.
6. **Update channel:** GitHub Releases only, or also an in-app "check now" with release notes?

---

## 11. Immediate next steps

1. ~~Answer §10 questions (scope, distribution)~~ — done.
2. ~~Stand up the local Mosquitto + HA test environment~~ — done; see [`docker/README.md`](../docker/README.md).
3. Answer the remaining §10 questions (target devices, TLS defaults, branding, update channel).
4. Scaffold M0: Gradle project, modules, CI.
5. Begin M1: `core:mqtt` + settings + FGS skeleton.
