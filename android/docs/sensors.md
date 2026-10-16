# Probeable device data (sensor catalog)

What an Android app can read from the phone, and how each item maps to a Home
Assistant entity. Use this to decide what the satellite reports.

Two rules of thumb:
- **No-permission data** is cheap and safe to report by default.
- **Permission-gated / special-access data** should be opt-in per entity, with a
  clear explanation in the app.

---

## 1. Recommended default set (v1, no permissions)

Everything below needs no runtime permission and works on API 26+.

| Data point | Source | HA entity | Notes |
|---|---|---|---|
| Battery level | `BatteryManager.EXTRA_LEVEL` | `sensor`, `device_class: battery`, `%` | diagnostic |
| Charging state | `EXTRA_STATUS` / `isCharging` | `binary_sensor`, `battery_charging` | diagnostic |
| Charger type | `EXTRA_PLUGGED` | `sensor`, `enum` (ac/usb/wireless/dock) | diagnostic |
| Battery temperature | `EXTRA_TEMPERATURE` (÷10 °C) | `sensor`, `temperature` | not all OEMs report it |
| Battery health | `EXTRA_HEALTH` | `sensor`, `enum` | diagnostic |
| Battery voltage | `EXTRA_VOLTAGE` (mV) | `sensor`, `voltage` | device-dependent |
| Battery current/power | `EXTRA_CURRENT_NOW` (µA) × voltage | `sensor`, `power`/`current` | divisor varies by OEM |
| Battery cycle count | `EXTRA_CYCLE_COUNT` (API 34+) | `sensor` | often unavailable |
| Remaining charge time | `EXTRA_CHARGE_TIME_REMAINING` (API 28+) | `sensor`, `duration` | charging only |
| Power-save mode | `PowerManager.isPowerSaveMode` | `binary_sensor`, `power` | diagnostic |
| Doze / device idle | `PowerManager.isDeviceIdleMode` | `binary_sensor` | diagnostic |
| Screen interactive | `PowerManager.isInteractive` | `binary_sensor` | diagnostic |
| Battery-optimization exempt | `isIgnoringBatteryOptimizations` | `binary_sensor` | diagnostic |
| App version | `PackageInfo.versionName` | `sensor` | diagnostic |
| Android version / SDK | `Build.VERSION.RELEASE` / `SDK_INT` | `sensor` | diagnostic |
| Security patch | `Build.VERSION.SECURITY_PATCH` | `sensor` | diagnostic |
| Device model / manufacturer | `Build.MANUFACTURER` / `MODEL` | **device block**, not an entity | goes in discovery |
| Uptime | `SystemClock.elapsedRealtime()` | `sensor`, `duration` | diagnostic |
| Network transport | `ConnectivityManager` + `NetworkCapabilities` | `sensor`, `enum` (wifi/cellular/ethernet/vpn/none) | diagnostic |
| Metered connection | `NET_CAPABILITY_NOT_METERED` | `binary_sensor` | diagnostic |
| IP addresses | `LinkProperties` (v4/v6) | `sensor` | diagnostic |
| Wi-Fi link speed | `WifiInfo.linkSpeed` | `sensor` | diagnostic |
| Wi-Fi signal strength | `WifiInfo.rssi` | `sensor`, `signal_strength` | diagnostic |
| Wi-Fi frequency band | `WifiInfo.frequency` | `sensor` | diagnostic |
| Storage (internal) | `StatFs` total/free | `sensor`, `data_size` | diagnostic |
| App memory usage | `Runtime` total/free | `sensor`, `data_size` | diagnostic |
| App data usage | `TrafficStats` tx/rx | `sensor`, `data_size` | since boot |
| Ringer mode | `AudioManager.ringerMode` | `sensor`, `enum` (normal/vibrate/silent) | diagnostic |
| Do Not Disturb | `NotificationManager.getCurrentInterruptionFilter()` | `sensor`, `enum` | diagnostic |
| Media volume | `AudioManager.getStreamVolume` | `number` (controllable) | also per stream |
| Headphones connected | `AudioManager.isWiredHeadsetOn` / device callback | `binary_sensor` | |
| Microphone muted | `AudioManager.isMicrophoneMute` | `binary_sensor` | |
| Screen brightness | `Settings.System.SCREEN_BRIGHTNESS` | `sensor` | |
| Screen off timeout | `Settings.System.SCREEN_OFF_TIMEOUT` | `sensor`, `duration` | |
| Screen orientation / rotation | `Configuration.orientation` / `Display.rotation` | `sensor`, `enum` | |
| Next alarm | `AlarmManager.getNextAlarmClock()` | `sensor`, `timestamp` | read-only |
| Device locked / secure | `KeyguardManager` | `binary_sensor` | |
| NFC enabled | `NfcAdapter.isEnabled` | `binary_sensor` | |
| Work profile active | `UserManager.isManagedProfile` | `binary_sensor` | |
| Time zone | `TimeZone.getDefault()` | `sensor` | |

### Hardware sensors (no permission)

| Sensor | `Sensor.TYPE_*` | HA entity | Notes |
|---|---|---|---|
| Ambient light | `LIGHT` | `sensor`, `illuminance` (lx) | common |
| Proximity | `PROXIMITY` | `sensor` | often binary near/far |
| Barometric pressure | `PRESSURE` | `sensor`, `pressure` (hPa) | device-dependent |
| Ambient temperature | `AMBIENT_TEMPERATURE` | `sensor`, `temperature` | rare on phones |
| Relative humidity | `RELATIVE_HUMIDITY` | `sensor`, `humidity` | rare |
| Magnetic field | `MAGNETIC_FIELD` | `sensor` | raw µT |
| Accelerometer / gyroscope | `ACCELEROMETER` / `GYROSCOPE` | raw, high-frequency | usually not useful for HA |

> Continuous hardware sensors should be **rate-limited and off by default** —
> they cost battery and generate a lot of MQTT traffic.

---

## 2. Permission-gated data (opt-in)

| Data point | Permission / access | Notes |
|---|---|---|
| Steps since boot | `ACTIVITY_RECOGNITION` (API 29+) | `sensor`, also `TYPE_STEP_COUNTER` |
| Activity (walking/running/in_vehicle/still) | `ACTIVITY_RECOGNITION` | Activity Recognition API |
| Sleep confidence / segments | `ACTIVITY_RECOGNITION` + Play services | Android Sleep API |
| Location (lat/long, accuracy, speed, altitude, bearing) | `ACCESS_FINE_LOCATION` | `device_tracker` |
| Background location | `ACCESS_BACKGROUND_LOCATION` | extra grant + Play policy |
| Geocoded address | location + `Geocoder` | `sensor`, `enum` |
| Wi-Fi SSID / BSSID | `ACCESS_FINE_LOCATION` (≤32) or `NEARBY_WIFI_DEVICES` (33+) | privacy-gated |
| Cellular carrier / SIM / roaming | `READ_PHONE_STATE` | `sensor` |
| Call state (idle/ringing/off-hook) | `READ_PHONE_STATE` | `sensor` |
| Bluetooth enabled / paired / connected devices | `BLUETOOTH_CONNECT` (12+); scan needs `BLUETOOTH_SCAN` + location | `sensor` / `binary_sensor` |
| Last used app | `PACKAGE_USAGE_STATS` (special, user enables in Settings) | Play-restricted |
| App standby bucket / inactive | `UsageStatsManager` | reflects how the OS treats us |
| Active notifications / last notification | `NotificationListenerService` (special) | Play-restricted |
| Health Connect (steps, heart rate, sleep, weight, …) | Health Connect permissions | Android 14+ built-in |
| Phone/Contacts/SMS/Calendar | respective runtime permissions | out of scope for a satellite |

---

## 3. Not available (restricted by Android)

These are commonly requested but **cannot** be read by a normal app:

- Hardware **serial number**, **IMEI/MEID**, **SIM serial** (privileged only).
- Hardware **MAC address** (randomized; not accessible since Android 10).
- Exact **installed-apps list** without `QUERY_ALL_PACKAGES` (Play-restricted).
- **Clipboard** contents while in the background (blocked since Android 10).
- Other apps' data, keystrokes, or screen contents.

This is why our device identity uses a **hashed/random id**, not a MAC or serial.

---

## 4. Mapping notes for HA

- Put app/system diagnostics under `entity_category: diagnostic` so they stay out
  of the main dashboard.
- Use `device_class` + `unit_of_measurement` + `state_class: measurement` so HA
  draws graphs and picks icons automatically.
- Enumerated values (charger type, transport, ringer mode) use `device_class: enum`
  with an `options` list.
- Controllable values (media volume, mic mute) are `number` / `switch` with a
  `command_topic`, not read-only sensors.
- One `device` block groups everything: name (random/editable), manufacturer,
  model, Android version, app version, configuration URL.

## 5. Proposed product behaviour

- **Default on:** the no-permission set in §1.
- **Opt-in:** everything in §2, with an in-app explanation and a permission
  request on enable.
- **Update cadence:** periodic (e.g. every 15 min) for slow-changing values, plus
  event-driven updates for battery/charging, connectivity, and power state.
- **Rate-limit:** hardware sensors (light/proximity) only when enabled, with a
  minimum interval and delta threshold.

---

## 6. Full capability envelope — if the user grants everything

A normal (non-root, non-system) app can go surprisingly far once the user opts
in. Capabilities come in four tiers.

### Tier A — runtime permissions (dialog at first use)

| Permission | Unlocks |
|---|---|
| `ACCESS_FINE_LOCATION` / `ACCESS_COARSE_LOCATION` | GPS/network location, speed, altitude, bearing, geocoding, Wi-Fi SSID/BSSID, cell info |
| `ACCESS_BACKGROUND_LOCATION` | location while backgrounded ("Allow all the time") |
| `ACTIVITY_RECOGNITION` | steps, walking/running/cycling/in-vehicle, sleep confidence |
| `READ_PHONE_STATE` / `READ_PHONE_NUMBERS` | carrier, SIM state, roaming, call state, own number |
| `BLUETOOTH_CONNECT` / `BLUETOOTH_SCAN` / `BLUETOOTH_ADVERTISE` (12+) | paired/connected devices, scanning, BLE beacon transmit |
| `NEARBY_WIFI_DEVICES` (13+) | Wi-Fi info without location |
| `RECORD_AUDIO` | microphone (wake word, level metering) |
| `CAMERA` | camera (motion detection, streaming) |
| `POST_NOTIFICATIONS` | show notifications |
| `READ_MEDIA_*` / `READ_MEDIA_VISUAL_USER_SELECTED` (13+/14+) | photos, video, audio files |
| `ACCESS_NOTIFICATION_POLICY` | change Do Not Disturb |
| Health Connect permissions | steps, heart rate, sleep, weight, etc. |

### Tier B — special app access (user enables in Settings)

| Special access | Unlocks | Notes |
|---|---|---|
| **Usage access** (`PACKAGE_USAGE_STATS`) | app usage, last-used app, app standby bucket | Play-restricted |
| **Notification access** (`NotificationListenerService`) | read/dismiss/reply to all notifications | very powerful; Play-restricted |
| **Battery optimization exemption** | run reliably through Doze | Play-restricted to core-function cases |
| **Display over other apps** (`SYSTEM_ALERT_WINDOW`) | draw overlays | |
| **All files access** (`MANAGE_EXTERNAL_STORAGE`) | broad file access | Play-restricted |
| **Install unknown apps** | trigger APK installs | |
| **Exact alarms** (`SCHEDULE_EXACT_ALARM`) | precise scheduling | |
| **Wi-Fi/Bluetooth control** | limited; toggling radios is blocked for target 29+ | |

### Tier C — explicit user consent to powerful APIs

| API | Unlocks | Caveat |
|---|---|---|
| **MediaProjection** | screen capture / screenshots | system consent dialog every session |
| **Accessibility service** (`BIND_ACCESSIBILITY_SERVICE`) | read screen contents, automate other apps, global actions | Play policy restricts to genuine accessibility/automation use |
| **VpnService** | intercept/route device traffic | consent dialog; heavy |
| **Device admin** (`BIND_DEVICE_ADMIN`) | lock, wipe, password policy | consent |
| **Full-screen intent** | alarms/calls style UI | Play-restricted |

With Tier A + B + C, an app can effectively observe almost everything a user
does — which is exactly why each of these is a separate, explicit opt-in and why
Play scrutinises them.

### Tier D — still impossible for a normal app

Even with every permission and special access:

- **IMEI / MEID / hardware serial / SIM serial** — privileged (system/root) only.
- **Hardware MAC address** — randomized; APIs return `02:00:00:00:00:00`.
- **Other apps' private files/data** — app sandbox is enforced by the kernel.
- **Background clipboard reads** — blocked since Android 10.
- **Silent app install / root-level access** — not possible.
- **Directly toggling Wi-Fi or mobile data** — deprecated/blocked for target 29+.
- **Persistent cross-reinstall device id** — only the resettable Android ID /
  advertising ID; no true hardware identity.

> Screens and other apps' contents *become* readable via **Accessibility** or
> **MediaProjection**, but only with explicit, revocable user consent — never
> silently.

## 7. What this means for the satellite

- **Default (Tier 0):** the no-permission set in §1 — always safe.
- **Recommended opt-ins for a HA satellite:** notification access (rich
  presence/events), usage access (last app), activity recognition (presence),
  background location (device_tracker), battery-optimization exemption.
- **Avoid by default:** Accessibility, MediaProjection, VpnService, device admin —
  high capability, high policy/compliance cost, and not needed to report device
  state.
- **Distribution note:** we ship via GitHub/F-Droid, so Play policy is advisory
  rather than blocking — but following it keeps the Play option open later.

