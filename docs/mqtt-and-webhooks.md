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

After accepting a command, the app republishes observed state. Acceptance does
not guarantee Android applied the change; OEM restrictions can refuse it.

The authoritative definitions are
[`Topics.kt`](../android/core/discovery/src/main/kotlin/uk/co/aaronburt/satellite/discovery/Topics.kt),
[`EntityCatalog.kt`](../android/core/discovery/src/main/kotlin/uk/co/aaronburt/satellite/discovery/EntityCatalog.kt),
and [`SatelliteCommand.kt`](../android/core/reporter/src/main/kotlin/uk/co/aaronburt/satellite/reporter/SatelliteCommand.kt).
Retained `online` is republished on every connect, including automatic
reconnects (fixed after v0.26.0; see [troubleshooting](troubleshooting.md)).

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
  "app_version": "0.26.0",
  "timestamp": 1792800000000,
  "state": { "cpu_percent": 12.5, "memory_percent": 48.1, "timestamp": 1792800000 }
}
```

Desktop's inner `state.timestamp` is in seconds. Android's state values remain
strings, including values that look numeric. A receiver must handle these schema
differences rather than assuming every body is identical.

Desktop skips new webhook snapshots while a request is running, and retries
failed delivery on a subsequent eligible collection tick. Android attempts
delivery on its configured reporting events/interval. Neither persists failed
snapshots for later replay.
