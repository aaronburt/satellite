# SatelliteAndroid — Research Notes

**Date:** 2026-10-07
**Status:** Research complete, plan pending owner decisions
**Scope:** An Android app that announces itself and its state to Home Assistant (HA) over MQTT, with a HACS-like presence (device, entities, update notifications).

> **Owner decisions (2026-10-07):** v1 is MQTT-only — the voice satellite (Wyoming) is out of scope. Distribution via GitHub Releases + F-Droid. §5 is retained as background research for a possible future track.

---

## 0. TL;DR

- **Yes, this is possible and it is a well-trodden path.** The mechanism is **Home Assistant MQTT Discovery**: the app publishes a JSON "config" payload describing itself (device metadata + entities) to a discovery topic, and HA auto-creates the device and all its entities. This is how ESPHome/Tasmota-style devices self-register.
- **A clarification about HACS:** HACS (Home Assistant Community Store) is a *store for HA custom integrations and frontend resources*. It does not use MQTT and it cannot install Android apps. What you likely want is the two things HACS is associated with:
  1. **Self-registration in HA without manual YAML** → MQTT Discovery (covered here).
  2. **An "update available" experience** → an MQTT `update` entity pointing at GitHub Releases (covered in §2.9).
- **HA now requires the *broker* to support MQTT v5.** Clients may use v5 or 3.1.1. Using an MQTT v5 client unlocks session expiry, message expiry (anti-ghost-entity), and better error reporting.
- **Client recommendation:** HiveMQ MQTT Client (MQTT 5, officially supports Android, actively maintained) hidden behind our own `MqttClient` interface. Paho Android (hannesa2 fork) is the simpler fallback but is in maintenance mode and is MQTT 3.1.1.
- **Android reality:** a persistent connection needs a **foreground service** (FGS). Use the `specialUse` type for an MQTT companion (not `dataSync`, which Android 15 limits to 6 h/24 h). Doze, OEM battery killers, and boot-start restrictions are the main robustness risks.
- **If the app is a voice satellite** (the repo name suggests it): audio should travel over the **Wyoming protocol**, not MQTT. MQTT carries device info, status, diagnostics, and control. See §5.

---

## 1. What "like HACS" means in HA terms

| Thing | What it actually is | Relevant to us? |
|---|---|---|
| HACS | A custom integration that downloads/manages *other HA integrations* and frontend cards from GitHub. Runs inside HA. | No — cannot distribute an Android app. |
| MQTT Discovery | A protocol by which a device publishes its own configuration to HA via MQTT; HA auto-creates a device + entities. | **Yes — this is the "self-reporting" mechanism.** |
| MQTT `update` entity | An HA entity that shows installed vs latest version, release notes, and an install action. | **Yes — this reproduces the HACS "update available" experience for our app.** |
| ESPHome / Tasmota / Zigbee2MQTT | Precedents that self-register via MQTT Discovery. | Good reference implementations. |

**Conclusion:** "Send information about itself via MQTT like HACS" = publish an MQTT Discovery payload (device + entity configs) plus state topics. HA then shows the app as a device with entities, and optionally an update entity.

---

## 2. HA MQTT Discovery — how the app self-reports

Source of truth: <https://www.home-assistant.io/integrations/mqtt/> (retrieved 2026-10-07).

### 2.1 Topic format

```text
<discovery_prefix>/<component>/[<node_id>/]<object_id>/config
```

- `discovery_prefix` defaults to `homeassistant` (configurable in HA; must be a user setting in our app).
- `component` is an MQTT platform (e.g. `sensor`, `binary_sensor`, `switch`, `number`, `text`, `button`, `notify`, `update`, `device_tracker`, `event`) **or `device`** for device discovery.
- `node_id` is optional and only for topic structuring; `object_id` is the per-item ID.
- Topic segments allow `[a-zA-Z0-9_-]` only.
- HA recommends: for entities with a `unique_id`, set `object_id` to the `unique_id` and omit `node_id`.

### 2.2 Device discovery (recommended — one message per device)

Since HA 2024.11, a device can publish **one discovery message** exposing all of its entities. HA recommends this for multi-entity devices (fewer messages, device info sent once).

- Topic: `homeassistant/device/<object_id>/config`
- The payload **requires** `device` (`dev`) and `origin` (`o`) mappings at the root.
- Entities go in the `components` (`cmps`) map. Each component requires:
  - `p` (platform, e.g. `sensor`),
  - a `unique_id` for entity-based components,
  - at least one platform-specific option.
- Shared options allowed at root: `availability`, `origin`, `command_topic`, `state_topic`, `qos`, `encoding`.

**Example — how SatelliteAndroid could announce itself:**

```json
{
  "~": "satellite/ab12cd34",
  "dev": {
    "ids": ["satellite_android_ab12cd34"],
    "name": "Kitchen Tablet",
    "mf": "SatelliteAndroid",
    "mdl": "Pixel Tablet",
    "mdl_id": "tangorpro",
    "sw": "0.1.0",
    "hw": "Pixel Tablet",
    "sn": "ab12cd34",
    "sa": "Kitchen",
    "cu": "https://github.com/<owner>/SatelliteAndroid"
  },
  "o": {
    "name": "SatelliteAndroid",
    "sw": "0.1.0",
    "url": "https://github.com/<owner>/SatelliteAndroid"
  },
  "avty_t": "~/availability",
  "qos": 1,
  "cmps": {
    "battery": {
      "p": "sensor",
      "uniq_id": "ab12cd34_battery",
      "dev_cla": "battery",
      "unit_of_meas": "%",
      "stat_cla": "measurement",
      "stat_t": "~/state/battery",
      "ent_cat": "diagnostic"
    },
    "charging": {
      "p": "binary_sensor",
      "uniq_id": "ab12cd34_charging",
      "dev_cla": "battery_charging",
      "stat_t": "~/state/charging",
      "ent_cat": "diagnostic"
    },
    "app_update": {
      "p": "update",
      "uniq_id": "ab12cd34_update",
      "dev_cla": "firmware",
      "stat_t": "~/state/update",
      "cmd_t": "~/cmd/install_update"
    }
  }
}
```

Notes:
- `~` is the base topic; `_topic` values starting or ending with `~` are expanded.
- Abbreviations are legal and recommended for constrained devices; full names are fine on Android. The complete abbreviation tables are in the HA docs (§ "Supported abbreviations").
- `entity_category: diagnostic` (`ent_cat`) keeps app/diagnostic entities out of the main UI — important for battery/system sensors.
- `device.connections` (`cns`) supports `[["mac", "..."]]`, but Android 10+ randomizes Wi-Fi MACs and hardware MAC is inaccessible; use `identifiers` (`ids`) as the primary ID and skip `mac`. Similarly, `serial_number` from `Build.getSerial()` is restricted — use a hashed `Settings.Secure.ANDROID_ID` instead.

### 2.3 Single-component discovery (alternative)

One discovery message per entity:

```text
homeassistant/sensor/<node_id>/<object_id>/config
```

Also valid, but more messages and device info must be repeated (or referenced by `identifiers`). We should use **device discovery** and fall back to single-component only if we need compatibility with older HA versions.

### 2.4 Availability, birth and last will

- **LWT (last will):** on CONNECT the app registers a will: topic `<base>/availability`, payload `offline`, retained. If the app disconnects (cleanly or not), the broker publishes `offline`.
- **Birth message:** after connecting, the app publishes `online` (retained) to its availability topic.
- HA subscribes via `availability_topic` / `availability` (with `payload_available` default `online`, `payload_not_available` default `offline`, and `availability_mode` `latest|all|any`).
- **HA birth:** HA publishes `online`/`offline` to `homeassistant/status` by default. Devices **should subscribe** and re-publish discovery + states when they see `online` (HA docs recommend a small random delay to avoid broker IO storms).
- Alternative to birth-triggered discovery: **retained discovery payloads** (simpler, but can leave ghost entities). Best practice is birth-triggered republish; we can do both, with Message Expiry Intervals to bound ghosts.

### 2.5 Retained messages, expiry and ghost entities

- Discovery configs may be retained so HA picks them up after restart. HA explicitly warns that retained configs can create **ghost entities** that keep coming back after the device is gone.
- MQTT v5 `message_expiry_interval` lets a retained message expire at the broker — the clean mitigation.
- State topics can be retained so HA restores the last value; same ghost/expiry considerations.
- Removing an entity: publish an empty payload to its config topic (removes the component; device entry disappears when no references remain).

**Proposed policy for the app:**
1. Publish discovery on every connect and on `homeassistant/status = online`.
2. Publish discovery **retained with a long expiry** (e.g. 7 days) as a safety net.
3. Publish state **retained with a short expiry** (e.g. 1 hour).
4. Availability is retained and controlled by LWT/birth.

### 2.6 Platforms useful for this app

| Platform | Use for SatelliteAndroid |
|---|---|
| `sensor` | battery %, battery temperature, app version, uptime, memory, storage, network type, IP, SSID |
| `binary_sensor` | charging, power-save mode, doze mode, interactive/screen-on, mic muted |
| `number` | media volume (with `cmd_t`, min/max/step, slider mode) |
| `switch` | wake word enabled, mic mute toggle |
| `text` | send arbitrary text to speak via Android TTS |
| `button` | restart service, re-publish discovery, run self-test |
| `notify` | HA sends notifications that the app displays (works with `command_topic`) |
| `update` | app update available (HACS-like) |
| `device_tracker` | optional location reporting |
| `event` | wake-word detected, command received (later, for satellite events) |

### 2.7 The HACS-like update entity

MQTT `update` platform (<https://www.home-assistant.io/integrations/update.mqtt/>):

- `state_topic` accepts a JSON payload with `installed_version` (required) plus optional `latest_version`, `title`, `release_summary`, `release_url`, `entity_picture`, and `in_progress` (progress monitoring).
- `command_topic` + `payload_install` trigger the install action (our app can open the GitHub release/APK URL).
- `device_class: firmware` gives it the firmware-update icon/UX.

Combined with GitHub Releases and a CI release workflow, this gives HA users an "update available" badge for the app — the HACS experience, delivered over MQTT.

### 2.8 Testing discovery without Home Assistant

`mosquitto_pub`/`mosquitto_sub` (or MQTT Explorer) can validate payloads before HA exists in the loop:

```bash
mosquitto_sub -h <broker> -v -t "homeassistant/#" -t "satellite/#"
mosquitto_pub -h <broker> -t "homeassistant/status" -m online   # simulate HA birth
```

For CI, run Mosquitto in Docker and assert on published messages.

---

## 3. MQTT client libraries for Android

| Library | MQTT | Android | Maintenance | Notes |
|---|---|---|---|---|
| **HiveMQ MQTT Client** | 5.0 + 3.1.1 | Official support (API 19+) | Active | Async/blocking/Rx flavors, backpressure, Netty-based. Needs Gradle packaging excludes + ProGuard keep rules. |
| **Paho Android (hannesa2 fork)** | 3.1.1 | Native Android service | Maintenance mode ("issues ignored, PRs welcome") | Kotlin, JitPack, v4 uses WorkManager instead of a foreground service; v3 FGS deprecated. |
| **Eclipse Paho Java** | 3.1.1 / separate v5 artifact | JVM | Slow/legacy | Underlies the Android fork. |
| **DitchOoM/mqtt** | 3.1.1 + 5.0 | Kotlin Multiplatform | New/less proven | Coroutine-first API, automatic reconnect, message persistence. |
| KMP-MQTT (ktor-based) | — | KMP | New | Less proven. |

**Recommendation:** define our own `MqttClient` interface and implement it with **HiveMQ** first.
Rationale: MQTT 5 (session/message expiry, reason codes), official Android guidance, active maintenance. Keep the interface small so a swap to Paho/DitchOoM is a one-module change if HiveMQ proves problematic (e.g. R8/Netty issues on some devices).

HiveMQ Android setup essentials (from their docs):
- Android Gradle Plugin 7+, Java 8 compatibility.
- `packagingOptions` excludes: `META-INF/INDEX.LIST`, `META-INF/io.netty.versions.properties`.
- ProGuard: keep `io.netty.**` member names and `org.jctools.**` members.
- minSdk ≥ 24 avoids retrofix backports.

---

## 4. Android platform constraints and robustness

### 4.1 Foreground service type

The MQTT connection must live in a foreground service to survive backgrounding, Doze, and OEM killers.

| FGS type | Fit | Caveats |
|---|---|---|
| `specialUse` | **Chosen for MQTT companion** | Requires a Play Console declaration + justification; no runtime time limit. |
| `dataSync` | Poor | Android 15+: 6 h per 24 h limit; cannot be started from `BOOT_COMPLETED` (API 35+). |
| `connectedDevice` | Not right | Intended for connections to external devices (BT/USB), not broker connections. |
| `microphone` | Needed only for voice features | Cannot start from `BOOT_COMPLETED` (API 34+); background mic access restrictions. |

References:
- FGS types: <https://developer.android.com/develop/background-work/services/fgs/service-types>
- FGS changes by API level: <https://developer.android.com/develop/background-work/services/fgs/changes>
- Android 15 FGS changes: <https://developer.android.com/about/versions/15/changes/foreground-service-types>
- Timeouts: <https://developer.android.com/develop/background-work/services/fgs/timeout>

### 4.2 Doze and App Standby

- Doze defers network access, jobs, syncs, and alarms when the device is stationary with the screen off. A foreground service keeps the app out of App Standby (app considered active), but Google still recommends FCM over persistent connections where possible — not an option for a local-only HA broker, which is exactly why FGS + battery-optimization exemption is the standard approach for HA-companion-style apps.
- Battery optimization exemption: `ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` is **restricted by Play policy** unless the core function is broken without it. A local smart-home satellite/companion arguably qualifies (HA companion app does this), but safer paths:
  1. Deep-link users to battery optimization settings (`ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS`) with clear in-app guidance, and
  2. Use the direct request only if policy allows for our distribution channel.
- A watchdog alarm (`setAndAllowWhileIdle`, min ~9 min intervals) can periodically verify the connection and restart the service.
- Doze docs: <https://developer.android.com/training/monitoring-device-state/doze-standby>

### 4.3 Boot start and background start

- `RECEIVE_BOOT_COMPLETED` + `BootReceiver` can start the service; note Android 15 restrictions on `dataSync`, media, camera, phone-call types (and mic on 14+) from `BOOT_COMPLETED`. `specialUse` is not on the restricted list, but this must be verified on real devices.
- Any FGS started from the background is subject to background-start restrictions; boot, notification actions, and user actions are the safe entry points.

### 4.4 OEM battery killers

Xiaomi/Huawei/Samsung/Oppo/Vivo aggressively kill background processes even with FGS. Mitigations: onboarding checklist with per-OEM instructions (dontkillmyapp.com patterns), "test connection" diagnostics, watchdog, and clear user documentation. The HA companion app has the same problem and solves it with in-app guidance.

### 4.5 Google Play requirements (2026)

- From **2026-08-31**, new apps and updates must target **Android 16 (API 36)**. Existing apps must target API 35+ to stay available.
- FGS type declarations are required in the Play Console (Policy → App content) for apps targeting API 34+.
- Data safety form, privacy policy, and the `specialUse` justification apply.
- References: <https://developer.android.com/google/play/requirements/target-sdk>, <https://support.google.com/googleplay/android-developer/answer/13392821>

### 4.6 Security

- TLS (8883) with a user-supplied CA (self-signed is common for home Mosquitto) via `network_security_config` or a custom trust store; optionally mTLS client certificates.
- Credentials at rest: **`EncryptedSharedPreferences` is deprecated** (Jetpack Security, 2025). Use Google Tink (`tink-android`) or DataStore with a Keystore-backed AES key. Never ship default credentials; onboarding requires explicit configuration.
- `android:usesCleartextTraffic="false"`; allow cleartext only as an explicit opt-in for advanced users.
- Sanitize logs (never log passwords/tokens); offer log export with redaction.

---

## 5. Voice satellite landscape (if in scope)

The repo name suggests a Home Assistant voice satellite. Important architectural fact: **voice satellites do not stream audio over MQTT.**

- **Wyoming protocol** is HA's satellite protocol (TCP + JSONL events + raw PCM audio). HA's Wyoming integration auto-discovers satellites on the network. Reference: <https://www.home-assistant.io/integrations/wyoming/>
- Existing Android projects to study:
  - **HassMic** — Android app + HA custom integration; runs a Wyoming satellite (port 10700) plus a control protocol (port 11700) for volume/media_player/sensors. Notes Android 12+ mic-from-background restrictions. <https://github.com/jeffc/hassmic>
  - **wyoming-satellite-termux** — Python satellite inside Termux. <https://github.com/T-vK/wyoming-satellite-termux>
  - **wyoming-android-tts** — exposes Android TTS to HA as a Wyoming service. <https://github.com/indigane/wyoming-android-tts>
  - A newer native Wyoming satellite app (Android/iOS/TV) was announced on the HA community forum in 2025/2026; worth locating before writing our own.
- Wake word can run on-device (openWakeWord/microWakeWord) or on the HA server. Android's SpeechRecognizer and on-device TTS are usable without cloud.
- **Constraints:** microphone FGS cannot start from `BOOT_COMPLETED`; background mic access generally requires the user to start it from the UI at least once per boot/foreground session; audio focus and echo cancellation need care on tablets.

**Implication for this project:** MQTT (device info, status, control) and Wyoming (audio) are complementary. A clean architecture keeps them in separate modules: `core:mqtt` now, `core:voice` later.

---

## 6. Reference projects to study

| Project | Why it matters |
|---|---|
| HA Android companion app | Sensor catalog, FGS/local-push architecture, battery optimization onboarding, multi-server UX. <https://companion.home-assistant.io/docs/core/sensors/> |
| ESPHome | The canonical MQTT-discovery device implementation; birth/LWT/retained patterns. |
| HASS.Agent (Windows) | A desktop app doing exactly this pattern; its MQTT discovery implementation is documented. |
| HassMic | Android + HA satellite architecture; real-world mic/FGS constraints. |
| Zigbee2MQTT / Tasmota | Discovery payload generation at scale. |

---

## 7. Sources

- HA MQTT integration + discovery spec — <https://www.home-assistant.io/integrations/mqtt/>
- HA MQTT update entity — <https://www.home-assistant.io/integrations/update.mqtt/>
- HiveMQ MQTT Client, Android install — <https://hivemq.github.io/hivemq-mqtt-client/docs/installation/android/>
- Paho Android (hannesa2 fork) — <https://github.com/hannesa2/paho.mqtt.android>
- DitchOoM/mqtt (KMP) — <https://github.com/DitchOoM/mqtt>
- Android architecture recommendations — <https://developer.android.com/topic/architecture/recommendations>
- Doze and App Standby — <https://developer.android.com/training/monitoring-device-state/doze-standby>
- FGS types — <https://developer.android.com/develop/background-work/services/fgs/service-types>
- FGS changes across versions — <https://developer.android.com/develop/background-work/services/fgs/changes>
- FGS timeouts (Android 15+) — <https://developer.android.com/develop/background-work/services/fgs/timeout>
- Play target API requirements — <https://developer.android.com/google/play/requirements/target-sdk>
- EncryptedSharedPreferences deprecation — <https://developer.android.com/reference/androidx/security/crypto/EncryptedSharedPreferences>
- HA companion sensors — <https://companion.home-assistant.io/docs/core/sensors/>
- Wyoming integration — <https://www.home-assistant.io/integrations/wyoming/>
- HassMic — <https://github.com/jeffc/hassmic>
