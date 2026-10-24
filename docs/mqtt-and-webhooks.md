# MQTT and webhook reference

Both apps use MQTT 5 clients. Configure them and Home Assistant to use the same
broker, with discovery enabled in Home Assistant. The topic layouts differ by
app; the tables below describe the current implementation.

## Windows and Linux

`<base>` is `<topic_prefix>/<node_id>`. The default prefix is `satellite`; the
default node ID comes from the lowercased hostname with spaces replaced by `-`.

| Purpose | Topic | QoS | Retained |
| --- | --- | --- | --- |
| Availability / Last Will | `<base>/status` | 1 | Yes |
| JSON telemetry snapshot | `<base>/state` | 0 | No |
| Device commands | `<base>/command` | 1 subscription | Publish commands without retain |
| Device notifications | `<base>/notify` | 1 subscription | Publish notifications without retain |
| Broadcast notifications | `<topic_prefix>/all/notify` | 1 subscription | Publish notifications without retain |
| Per-entity discovery | `homeassistant/<component>/<node_id>/<unique_id>/config` | 1 | Yes |

Discovery topic node/unique-ID segments are sanitized for Home Assistant.
Discovery is sent when the agent connects. Disabled/unsupported discovery items
are cleared with an empty retained payload. State is published on significant
changes or a heartbeat (default 60 seconds), checked on each collection tick
(default 5 seconds). Availability payloads are `online` and `offline`.

Commands accept plain text (`lock`) or JSON (`{"action":"lock"}` or
`{"command":"lock"}`). Canonical actions and their required settings:

| Actions | Setting in `expose` |
| --- | --- |
| `lock` | `remote_lock` |
| `display_sleep` | `display_state` |
| `media_play_pause`, `media_play`, `media_pause`, `media_next`, `media_prev`, `media_stop` | `media_control` |
| `volume_mute`, `volume_up`, `volume_down` | `media_control` |

Notifications require `expose.notifications`. Their payload is:

```json
{
  "title": "Laundry",
  "message": "The washing machine has finished.",
  "url": "https://example.com/dashboard",
  "silent": true
}
```

`message` must be nonempty. `title`, `url`, and `silent` are optional. Omitting the
title uses the hostname (or node ID if the hostname is unavailable).

The runtime routing and discovery definitions are in
[`client.go`](../windows/internal/mqtt/client.go) and
[`discovery.go`](../windows/internal/mqtt/discovery.go).

## Android

`<base>` is `satellite/<device_id>`, where the app persists its generated device
ID. The prefix is fixed in the current Android implementation.

| Purpose | Topic | QoS | Retained / expiry |
| --- | --- | --- | --- |
| Availability / Last Will | `<base>/availability` | 1 | Retained; no message expiry |
| Scalar entity state | `<base>/state/<key>` | 1 | Retained; 1 hour |
| Device commands | `<base>/cmd/<action>` | 1 subscription | Publish commands without retain |
| Device discovery | `homeassistant/device/satellite_<device_id>/config` | 1 | Retained; 7 days |
| Home Assistant birth | `homeassistant/status` | 0 subscription | `online` triggers discovery and state refresh |

Sensor states are strings such as `87`, `ON`, or `wifi`. Read-only entities are
included by the catalog; controllable entities are added only after opt-in.

| Command suffix | Payload | Required enabled entity |
| --- | --- | --- |
| `volume` | Integer from `0` to `100` | `volume_media` |
| `mute` | `ON` or `OFF` (case-insensitive) | `mic_muted` |
| `refresh` | `PRESS` (case-insensitive) | `refresh_discovery` |

After accepting a command, the app republishes observed state. Acceptance does
not guarantee Android applied the change; OEM restrictions can refuse it.

**Refresh discovery.** Enabling `refresh_discovery` on the Entities screen adds
a Home Assistant button (`button` platform, `config` category, command topic
only) and a diagnostic `refresh_result` sensor (`ok`, `partial`, `failed`).
Both are removed again when the control is turned off, and presses are ignored
while it is off. A press republishes the discovery payload and every active
state once, over the existing subscriptions, then publishes the result:
`ok` when the broker accepted everything, `partial` when only discovery or
only the states were accepted, `failed` when neither was. Presses that arrive
while a refresh is running, or within two seconds of it finishing, are dropped
rather than queued. While the broker is unreachable no press can arrive, so a
`failed` result only appears for broker rejections.

The authoritative definitions are
[`Topics.kt`](../android/core/discovery/src/main/kotlin/uk/co/aaronburt/satellite/discovery/Topics.kt),
[`EntityCatalog.kt`](../android/core/discovery/src/main/kotlin/uk/co/aaronburt/satellite/discovery/EntityCatalog.kt),
and [`SatelliteCommand.kt`](../android/core/reporter/src/main/kotlin/uk/co/aaronburt/satellite/reporter/SatelliteCommand.kt).
Retained `online` is republished on every connect, including automatic
reconnects (fixed after v0.26.0; see [troubleshooting](troubleshooting.md)).

### Identity, expiry, and cleanup (Android)

**Stable identity.** The device ID is generated once and persisted; Home
Assistant keys the device on `satellite_android_<device_id>`. Renaming the
device only changes `device.name` in the next discovery payload, so the device
and its entity IDs survive renames. Clearing app data or reinstalling creates a
new ID, and therefore a new Home Assistant device.

**Broker expiry vs Home Assistant registry.** Retained state expires on the
broker after 1 hour and discovery after 7 days (MQTT 5 message expiry, verified
against HiveMQ in `HiveMqttClientTest`). Expiry only stops the broker from
replaying the message to new subscribers; Home Assistant keeps the device in
its registry. Only an empty retained message on the discovery topic removes the
device from Home Assistant. Brokers without MQTT 5 ignore expiry and keep
messages until they are deleted.

| Scenario | What the app does |
| --- | --- |
| Control disabled | Clears the control's retained state and republishes discovery without it. If offline, the state is cleared on the next connect. |
| Device renamed | Republishes discovery under the same ID; nothing is left behind. |
| Device removed on purpose | `SatelliteReporter.removeFromHomeAssistant()` publishes empty retained messages to the discovery, every state, and the availability topic, touching only this device's topics. |
| App reinstalled / data cleared | The old device stays in Home Assistant until its retained topics expire. Remove it in Home Assistant (Settings → Devices → device → Delete) or clear its topics manually, below. |

To clear a replaced installation by hand, using the old device ID:

```sh
mosquitto_pub -h <broker> -r -n -t homeassistant/device/satellite_<old_id>/config
mosquitto_pub -h <broker> -r -n -t satellite/<old_id>/availability
# and, for each state key, satellite/<old_id>/state/<key>
```

## Telemetry webhooks

Both apps send JSON with `Content-Type: application/json` and optionally
`Authorization: Bearer <token>`. HTTP 2xx responses count as success. A webhook
delivers telemetry only; MQTT discovery and remote commands need MQTT.

Desktop can send MQTT and a webhook simultaneously. Android chooses MQTT or
webhook in Settings.

| Field | Desktop | Android |
| --- | --- | --- |
| `device_id` | Node ID | Persisted device ID |
| `device_name` | Hostname | Configured display name |
| `app_version` | Agent version | APK version |
| `timestamp` | Unix milliseconds | Unix milliseconds |
| `platform` | `windows` or `linux` | Omitted |
| `manufacturer`, `model`, `android_version` | Omitted | Device metadata |
| `state` | Snapshot with JSON numbers, booleans, strings, and nested values | Map of entity keys to strings |

For example, a desktop body can include:

```json
{
  "device_id": "office-pc",
  "device_name": "OFFICE-PC",
  "platform": "windows",
  "app_version": "0.26.1",
  "timestamp": 1792800000000,
  "state": { "cpu_percent": 12.5, "memory_percent": 48.1, "timestamp": 1792800000 }
}
```

Desktop's inner `state.timestamp` is in seconds. Android's state values remain
strings, including values that look numeric. A receiver must handle these schema
differences rather than assuming every body is identical.

### Delivery recovery

Both apps use the same bounded, in-memory policy: **only the newest snapshot
is kept**, and a failed POST is retried with the original body, including its
original `timestamp`. Nothing is written to disk, and nothing is replayed after
a restart.

| | Value |
| --- | --- |
| Retried | no response (offline, DNS, connection reset, timeout), 408, 425, 429, 5xx |
| Not retried | any other non-2xx (400, 401, 403, 404, 422, ...) and invalid URLs |
| Attempts | 5 per snapshot, including the first |
| Backoff | 2 s, 4 s, 8 s, ... capped at 60 s, with jitter (each wait is 50 to 100% of that) |
| Retry-After | used when longer than the backoff, capped at 5 minutes. Desktop accepts seconds or an HTTP date; Android accepts seconds |
| Expiry | a snapshot captured more than 5 minutes ago is dropped instead of retried |

Retries stop when the webhook is disabled or cleared, its URL changes, or the
app/reporter shuts down; a changed bearer token is used on the next retry.

Platform differences:

- **Desktop** runs one request at a time off the collection loop, so webhook
  work never delays MQTT. Snapshots that become due while a request or retry
  is pending replace one queued slot (newest wins) and are sent next, still
  after the current backoff wait.
- **Android** cancels a pending retry as soon as a newer publish happens
  (interval tick, event, settings change or manual refresh), then sends the new
  snapshot straight away.

Because a request can time out after the receiver has processed it, receivers
may occasionally see the **same snapshot twice**. Treat `device_id` +
`timestamp` as an idempotency key if duplicates matter.
