# Satellite

Lightweight Windows & Linux telemetry agent and remote controller for Home Assistant over MQTT.

[Documentation index](../docs/README.md) | [Architecture](../docs/architecture.md) |
[HTTP API](../docs/http-api.md) | [Protocol reference](../docs/mqtt-and-webhooks.md) |
[Troubleshooting](../docs/troubleshooting.md) | [Privacy](../PRIVACY.md)

---

## Local web UI

The local web UI works on desktop and phone-sized windows. Its five tabs are:

| Tab | Purpose |
| --- | --- |
| Sensors | Cards for every enabled, supported sensor, grouped by System, Network, Session, Media & devices and Screen. Processor, memory, network and each GPU include recent-history graphs. Enabled thresholds highlight warning and critical readings; optional screen previews appear here. |
| Privacy | Saved permissions, temporary privacy-pause restrictions, allowed media commands, reporting destinations and an expandable current JSON snapshot. |
| Diagnostics | MQTT and telemetry webhook health, connection tests, delivery counters and recent failures. |
| Logs | Live log events with search and severity filtering. |
| Settings | Configure reporting, sensors, thresholds, remote controls, local access and maintenance. |

Performance graphs are included in Sensors. See [Web UI monitoring and privacy](#web-ui-monitoring-and-privacy) for graph history, snapshot details and threshold behavior.

## Features

- **Telemetry:** CPU, GPU (Core Load, VRAM, Temperature, Power), RAM, Disk, Network, Battery, Uptime, Local IP, Wi-Fi SSID & Signal Strength.
- **Sensors:** Workstation Lock, User Presence/Idle, Microphone Active, Webcam Active, Display Power State, Audio Output Device, Fullscreen/Gaming, System Dark/Light Theme, Pending Reboot.
- **Screen Thumbnail (Windows, opt-in):** a tiny picture of the main screen (200px by default, adjustable from 64 to 400px), taken with each telemetry update (every 5 seconds by default), shown in Home Assistant as an image entity. Off by default; see [Screen thumbnail](#screen-thumbnail).
- **Media Tracking:** Now Playing track title, artist, and app via Windows Media Transport Controls or Linux MPRIS D-Bus.
- **iCUE RGB (Windows x64, opt-in):** read device LED colours through Corsair's SDK and publish them in MQTT telemetry, without changing lighting. See [iCUE RGB](#icue-rgb).
- **Remote Controls:** Workstation Lock, Sleep Displays, Play/Pause, Next/Prev Track, Mute, Volume Up/Down.
- **Desktop Notifications:** Native Windows 10/11 toast alerts and Linux FreeDesktop D-Bus notifications with titles, messages, clickable URLs, and silent chime option.
- **Home Assistant:** Instant MQTT Auto-Discovery for sensors, media player, and buttons. Only sensors supported by the host OS/hardware are discovered.
- **Generic Webhook:** Optionally POST telemetry snapshots as JSON on changes or heartbeats, alongside (or instead of) MQTT.
- **Privacy overview:** Review saved permissions, privacy-pause restrictions, reporting destinations and the current telemetry dataset without enabling `/json`.
- **Sensor thresholds:** Opt-in warning and critical limits for CPU, memory, GPU usage, disk space used and battery. Highlight cards and the sensor summary without notifications or changes to published telemetry.
- **Connection Diagnostics:** The web UI's Diagnostics tab shows MQTT and telemetry webhook health, attempts, reconnects, successful telemetry writes, recent failures with suggested fixes, and scheduled retries. Test saved connections or copy a report without server addresses, credentials or sensor readings.
- **Tray, Web UI & Headless:** Windows system tray, local Web UI for setup, and headless operation (`--headless`) including Linux systemd user services. Linux has no tray implementation.
- **Quick Privacy Pause (Windows tray):** pause sensitive sensors for 5, 15, 30, or 60 minutes, with a live countdown, early resume, and automatic resume.

---

## Quick Start

### 1. Download or build

Download `satellite.exe` or the Windows installer, or `satellite-linux-amd64`, from
[GitHub Releases](https://github.com/aaronburt/satellite/releases).
For the downloaded Linux binary, make it executable and rename it before using
the commands below:

```sh
chmod +x satellite-linux-amd64
mv satellite-linux-amd64 satellite
```

To build from source, install the Go version declared in [`go.mod`](go.mod) and run
these commands from `windows/`:

- **Windows Binary:**
  ```powershell
  go build -ldflags="-H windowsgui -s -w" -o satellite.exe .
  ```
- **Linux Binary:**
  ```bash
  go build -ldflags="-s -w" -o satellite .
  ```
- **Windows Installer (Inno Setup):**
  ```powershell
  & "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe" installer.iss
  ```

### 2. Run

- **Tray Mode (Windows):** Run `satellite.exe`, then choose **Open Satellite** from its tray menu. Use **Open in Browser** for browser access.
- **Headless Mode (Linux Server / Background Service):** Run `./satellite --headless`.
- **Setup on Linux:** Run `./satellite --debug` and open the URL printed in the terminal. Stop it after saving, then launch normally or as a service.
- **CLI Mode:** Run `./satellite --cli` for a local terminal dashboard. CLI mode does not start MQTT or webhook reporting.
- **Systemd User Service (Linux):**
  ```bash
  install -Dm755 satellite ~/.local/bin/satellite
  mkdir -p ~/.config/systemd/user
  cp satellite.service ~/.config/systemd/user/
  systemctl --user daemon-reload
  systemctl --user enable --now satellite
  ```

### 3. Configure

Select **Open Satellite** from the Windows tray menu to open the dedicated configuration window. It enables the local web UI automatically and loads the existing interface using Microsoft Edge WebView2. Closing the window leaves the agent running; selecting **Open Satellite** again focuses or reopens it. **Open in Browser** opens the same interface in your default browser. On Linux, launch `--debug` and open the printed URL to connect your MQTT broker, toggle exposed sensors, configure the `/json` snapshot endpoint, set up the telemetry webhook, or enable webhook log pushing.

The Windows window requires the Microsoft Edge WebView2 Runtime. If it is unavailable, use **Open in Browser** or install the runtime from [Microsoft](https://developer.microsoft.com/en-us/microsoft-edge/webview2/). Satellite does not download or install it automatically. The client runs as a child of the same executable, uses a separate profile under the user's local cache directory, and exits when the agent exits or the web UI is disabled. External web links open in your default browser. The management HTTP server and token authentication remain in use.

Settings has jump-to sections for **Reporting**, **Sensors**, **Remote controls**, **Local access**, and **Maintenance**. MQTT and both webhooks are grouped under Reporting. Connection tests use the values entered in the form without saving them, and keep their results beside the test button.

Use **Apply changes** to save all sections, or **Discard changes** to restore the last saved values. **Enable hardware sensors** only selects supported hardware sensors; sensitive sensors and remote controls require individual choices. **Disable all sensors** leaves remote-control permissions and the device summary unchanged.

The WebUI automatically probes your operating system and hardware on boot and only displays sensor toggles that are actually supported on your current machine.

Config is saved at:

- **Windows:** `%APPDATA%\satellite\config.json`
- **Linux:** `~/.config/satellite/config.json`

---

## Configuration reference

The default listener is `127.0.0.1` with a dynamically selected port. Use the URL
from the tray or debug output. The WebUI uses an ephemeral token; the optional
`/json` endpoint instead uses `X-API-Key` or the `api_key` query parameter.
See the [HTTP API reference](../docs/http-api.md) for every endpoint, request
formats, authentication, and PowerShell/Go examples.

| Setting | Default | Meaning |
| --- | --- | --- |
| `mqtt.broker` | Empty | Broker URL; leave empty for webhook-only operation |
| `mqtt.topic_prefix` | `satellite` | Prefix for state, command, and notification topics |
| `interval_sec` | `5` | Collection interval; changes apply without a restart |
| `heartbeat_sec` | `60` | Publish even without a significant change, checked on collection ticks |
| `bind_address` | `127.0.0.1` | HTTP listener address |
| `port` / `webui_port` | `0` | HTTP port; zero selects an available port |
| `json_enabled` | `false` | Enable the last-snapshot JSON endpoint |
| `check_updates` | `false` | Enable background GitHub release checks |
| `telemetry_webhook.enabled` | `false` | Send state snapshots to an HTTP endpoint |
| `webhook.enabled` | `false` | Send application log events to an HTTP endpoint |

CPU, memory, and network telemetry are enabled by default. Other sensors,
notifications, and remote controls are opt-in and depend on platform support.
The full configuration model is in [`config.go`](internal/config/config.go).

Configuration can also be exported and imported. With the agent stopped, run:

```powershell
.\satellite.exe --export satellite-config.json
.\satellite.exe --import satellite-config.json
```

Exports may contain credentials. Supply `--password <passphrase>` on both commands
for an encrypted export/import; the passphrase is supplied as a command-line
argument. Import adapts node/client identity to the current hostname by default;
use `--reset-node-id=false` to retain the imported identity. Restart the agent
after importing. Linux uses `./satellite` with the same flags.

## Device summary entity

Enable **Device summary entity (Home Assistant)** in Settings to discover one optional sensor named after the Satellite device. It uses the existing `<topic_prefix>/<node_id>/state` topic and exposes that snapshot as attributes; individual sensors and controls remain available. It does not enable any additional sensors. The setting is `expose.device_summary` and defaults to `false`.

The state is **Locked** when the enabled session-lock sensor reports locked, otherwise **Active** or **Idle** when presence reporting is enabled, and **Online** when neither reading is available. MQTT availability makes the summary unavailable when the device disconnects.

When screen thumbnails are enabled, the summary resolves an image entity belonging to the same device in Home Assistant and uses its current picture. With multiple images, the first image entity in alphabetical entity-ID order is used. The picture is cleared on the next telemetry update when thumbnails are disabled, privacy pause is active, or the workstation is locked. Renaming the summary or image entity is supported. The summary does not upload additional images.

Add a tile using the actual summary entity ID from Home Assistant:

```yaml
type: tile
entity: sensor.your_satellite_device
show_entity_picture: true
```

Attributes include only the readings present in the published snapshot, plus thumbnail availability metadata. Home Assistant may record changing attributes in its database; exclude the summary from Recorder if you do not need its history. A standard tile shows the summary state and picture; individual controls still use their own entities.

## Media command permissions

Media details controls reporting of track, artist, and playback state. Allow media commands is independent, so controls can be enabled without sharing media details. In Settings, choose separate allowances for play/pause, stop, next track, previous track, mute/unmute, volume up, and volume down. Disabled commands are rejected over MQTT and the local action API, and their Home Assistant buttons are removed.

The command switch is stored in `expose.media_commands`; when omitted, it follows the legacy `media_control` setting. Permissions are stored in `expose.media_permissions` using `play_pause`, `stop`, `next`, `previous`, `mute`, `volume_up`, and `volume_down`. Existing configurations without this object retain all media allowances when media commands are enabled. In an explicit permissions object, omitted allowances are disabled.

## Lock cooldown

In Settings → Remote controls, **Lock cooldown (seconds)** sets the minimum
time between lock attempts. The default is **60 seconds**, including for existing
configurations. Set it to **0** to disable the cooldown. Changes apply immediately
after saving and are preserved by configuration export/import.

MQTT and HTTP lock commands share the cooldown, including command aliases.
Extra requests are discarded without extending the cooldown or being queued.
Failed lock attempts also start the cooldown. HTTP returns status 429 when a
lock is blocked. The timer resets when Satellite restarts; it does not affect
locking directly through the operating system.

## Web UI monitoring and privacy

The **Sensors** tab includes recent-history graphs for processor, memory, network traffic and each GPU alongside the other sensor cards. The former Performance tab is combined into this view. Graph history is kept in the open page and resets on reload.

The **Privacy** tab lists saved permissions, temporary privacy-pause restrictions, allowed media commands and reporting destinations. Expand **Inspect current JSON** to see the current dataset used by `/json`, even when the JSON endpoint is disabled. This sample is not a delivery log: MQTT may adjust RGB and summary-thumbnail fields, telemetry webhooks add device metadata, and screen images and log events travel separately. Opening Privacy does not request a screen capture. Destination hostnames are shown without credentials, paths or query strings.

Under **Settings → Sensors → Sensor thresholds**, enable warning and critical percentages for processor usage, memory usage, GPU usage, disk space used or battery level. Usage and disk warnings trigger at or above a limit; battery warnings trigger at or below it. GPU and disk limits apply to each device individually. Warning and critical messages appear on cards and in the sensor summary, and clear on the next reading below the applicable limit (above it for battery). Missing readings do not trigger thresholds.

Thresholds are disabled by default, are saved with the device configuration and survive export/import. They only affect web UI highlights: they do not enable sensors, send notifications or alter MQTT, webhook or `/json` telemetry.

## Quick privacy pause

From the Windows tray, open **Quick Privacy Pause** and choose **5, 15, 30, or
60 minutes**. The tray menu and hover tooltip show the remaining time, updated
every second. Choosing another duration starts a new countdown from that moment.
Choose **Resume sensitive sensors now** to end it early.

The pause covers the **High** and **Critical Privacy Risk** groups in Settings:
user presence/idle time, session lock, media details, iCUE RGB, active process, window
titles, microphone/webcam usage, and thumbnails from every selected display.
These readings are omitted from MQTT, telemetry webhooks (including queued
retries and connection tests), `/json`, and the Sensors view. Thumbnail capture
stops and cached previews are cleared. Old snapshots and frames cannot reappear
after resume; fresh readings arrive on the next normal collection tick.

Hardware telemetry, network/peripheral context, notifications, and enabled
remote commands keep working. Saved sensor choices are preserved, including
changes made during the pause. Only sensors currently enabled resume when the
deadline passes. Time spent asleep counts toward the duration.

The pause lasts for the current Satellite session; exiting or restarting the app
ends it. It cannot retract requests already sent or erase readings/images already
stored by Home Assistant, brokers, webhook receivers, or an open browser.

## iCUE RGB

Satellite can read RGB colours from devices exposed by Corsair iCUE. This is
off by default and read only: Satellite never sets colours, requests exclusive
control, or switches iCUE profiles. It requires Windows x64, iCUE 4.31 or newer
running locally, and permission for Satellite in iCUE's SDK integration settings.
Run Satellite and iCUE at the same privilege level. Not every device visible in
iCUE is necessarily exposed by the SDK.

On Windows x64, enable **iCUE RGB Colours (Read Only)** under
**User Presence & Application Activity** in Settings. Satellite immediately checks
whether the SDK DLL is present and can be loaded. If it is missing or unusable,
a setup prompt links to [Corsair's official SDK downloads](https://github.com/CorsairOfficial/cue-sdk/releases)
and [licence terms](https://corsairofficial.github.io/cue-sdk/#end-user-license-agreement).
Satellite and its installer do not bundle, download, or install the SDK.

Download the Windows SDK archive yourself and extract
`iCUESDK/redist/x64/iCUESDK.x64_2019.dll`. Create
`%APPDATA%\satellite\sdk` if needed and copy the DLL there, or place it beside
`satellite.exe`. Satellite checks beside its executable first, then the current
user's SDK folder. The Microsoft Visual C++ 2015–2022 x64 runtime may be needed
for the SDK DLL to load.

While RGB is checked and Settings is visible, Satellite rechecks every 3 seconds
until it can load the SDK. You can close the prompt while installing it. Checks
stop when RGB is unchecked, you leave Settings, or the browser tab is hidden;
they resume when you return. A successful check closes the prompt and tells you
to apply your settings. If RGB was already saved as enabled, readings resume on
the next collection tick without restarting Satellite. The check itself neither
enables reporting nor connects to iCUE. The Sensors tab shows connection status
and counts of readable devices and LEDs. Bulk enable does not enable RGB.

Readings appear under `rgb` in the existing `<topic_prefix>/<node_id>/state`
MQTT JSON message. Each device contains a stable hashed identifier, model, SDK
device type, `leds` with numeric IDs and RGBA values from 0–255, and
`average_rgb`: the rounded arithmetic mean of all returned LED RGB channels,
including black LEDs, without alpha weighting. Raw serial numbers are omitted.
Home Assistant discovers an **iCUE RGB Devices** sensor; its attributes contain
the RGB status and full device list for compatibility. It also discovers each
readable physical RGB device as a separate Corsair device, linked to the
Satellite computer. Each has an **Average colour** sensor whose state is a hex
colour such as `#FF4000`. Its attributes contain that device's `average_rgb`,
`led_count`, and individual `leds` with their IDs and RGBA values, plus its model
and SDK device type. There is one sensor per device, rather than one per LED.
No light controls are created.

Subdevices are discovered when their first readable sample arrives. Their
identities stay stable across reconnects and changes to colour or model name;
matching models remain separate devices. Discovery is retained for Home
Assistant restarts. An unplugged or unreadable device keeps its entity but
becomes unavailable on the next state update, with its colour attributes
cleared. Privacy pause also makes RGB subdevices unavailable. Disabling RGB
clears their retained discovery entries, including after restarting Satellite.

Colours are sampled at the normal collection interval (5 seconds by default).
Changes trigger normal telemetry publishing; this is not an animation stream.
Disconnected or unreadable devices are removed from the next reading rather
than retaining stale colours. A partial read is labelled `partial`. RGB also
appears in enabled telemetry webhooks and local snapshots. Disabling the sensor
ends the SDK connection on the next collection tick and omits RGB from subsequent
snapshots. Privacy pause stops RGB reads and filters cached RGB readings because
screen-reactive lighting can reveal screen or application activity.

## Remote Commands

Publish plain text or JSON `{"action": "<command>"}` to `satellite/<node_id>/command`:

| Command | Action |
| :--- | :--- |
| `LOCK` | Lock Windows session |
| `display_sleep` | Put displays to sleep (turn off monitors) |
| `media_play_pause` | Toggle media play/pause |
| `media_play` | Play media |
| `media_pause` | Pause media |
| `media_next` | Skip to next track |
| `media_prev` | Skip to previous track |
| `media_stop` | Stop media playback |
| `volume_mute` | Toggle master mute |
| `volume_up` | Volume up (+2%) |
| `volume_down` | Volume down (-2%) |
 
---
 
## Toast Notifications
 
Send native Windows toast alerts from Home Assistant by publishing JSON to:
- **Targeted Machine**: `satellite/<node_id>/notify` (e.g. `satellite/my-pc/notify`)
- **All Machines (Broadcast)**: `satellite/all/notify` (delivered to all connected Satellite agents simultaneously)
 
```json
{
  "title": "Front Door",
  "message": "Motion detected on the porch",
  "url": "https://homeassistant.local:8123/dashboard-cameras",
  "silent": false
}
```
 
- `title` *(optional)*: Notification title. Defaults to the PC hostname if omitted.
- `message` *(required)*: Body text of the notification.
- `url` *(optional)*: Clicking the notification launches this URL in your default browser.
- `silent` *(optional)*: Set to `true` to suppress audio chime.
 
### Home Assistant Automation Examples

**Broadcast to All PCs:**
```yaml
action: mqtt.publish
data:
  topic: satellite/all/notify
  payload: >
    {
      "title": "Doorbell",
      "message": "Someone is at the front door.",
      "url": "https://homeassistant.local:8123/dashboard-cameras"
    }
```

**Targeted to a Specific PC:**
```yaml
action: mqtt.publish
data:
  topic: satellite/my-pc/notify
  payload: >
    {
      "title": "Laundry Done",
      "message": "The washing machine cycle has finished.",
      "url": "https://homeassistant.local:8123/lovelace/appliances"
    }
```


---

## Home Assistant

When connected to your MQTT broker, Home Assistant automatically discovers:
- Telemetry sensors and binary sensors (including Webcam, Display Power, Audio Output, and Wi-Fi).
- `media_player.<node_id>_media_player` entity.
- Quick action buttons (Lock PC, Sleep Displays, Play/Pause, Next, Previous, Mute, Volume).
- `image.<node_id>_screen_thumbnail`, if you turn on the screen thumbnail (Windows only).

### Screen thumbnail

This is a privacy-sensitive feature, so it is **off by default** and is never
switched on by the web UI's "enable all" option. Turn it on under
**Sensitive Content & Surveillance → Screen Thumbnails** in the web UI, or set
`expose.screen_thumbnail` to `true` in the config file.

By default, Satellite takes a picture of the main (primary) monitor each time
it collects telemetry (every `interval_sec` seconds, 5 by default), scaled so its
longest side is 200 pixels by default with the screen's shape kept (a
1920×1080 screen gives 200×113). The size can be changed in the web UI or with
`screen_thumbnail_size` in the config file. It is limited to 64 to 400 pixels;
400 is a hard privacy cap. Changing the update interval in settings changes how often the
picture is taken. It is published as a JPEG to
`<topic_prefix>/<node_id>/screen_thumbnail` and appears in Home Assistant as an
image entity.

To capture other monitors, set **Displays to capture → Choose displays**, then
select **Display 1**, **Display 2**, **Display 3**, and so on. **Select all connected**
selects every currently connected display; newly connected displays need to be
selected explicitly. The list shows each display's resolution and which one is
primary. Use **Refresh displays** after connecting or disconnecting a monitor.
Selections are saved as `screen_thumbnail_displays`, for example `[1, 2, 3]`.
Numbers identify Windows display devices and can change when Windows reassigns
them after hardware or driver changes. Review the selection after such changes.

Each selected display gets a separate Sensors preview, MQTT topic
`<topic_prefix>/<node_id>/screen_thumbnail_display_<number>`, and Home Assistant
image named **Screen Thumbnail Display 1**, **Screen Thumbnail Display 2**, etc.
Display numbers from 1 to 64 are supported. An empty or omitted selection keeps
the existing primary-display topic and entity. Switching to numbered displays
removes the legacy image from discovery; switching back restores it. Deselecting
a display removes its discovery entry. A disconnected selected display shows
as unavailable in the web UI, keeps its selection, and resumes after reconnecting.
Moving or rotating a display recreates its capture resources automatically.

- Windows scales the screen down while copying it, so Satellite never holds a
  full-size screenshot in memory. Only the 200px image is made.
- Nothing is saved to disk. The MQTT message is not retained, so the broker does
  not keep a copy. Home Assistant shows the latest image; it can keep its own
  copy like any other entity state.
- Nothing is captured while the PC is locked. It is also not captured while MQTT is disconnected, unless you have the web UI's **Sensors** tab open (see below).
- If **NVIDIA NVML** under the GPU sensors is also on, the screen is shrunk on
  the graphics card instead (Windows Desktop Duplication, which works with any
  GPU that drives each selected monitor). The full frame stays in video memory
  only while it is being shrunk and is never copied to normal memory; only the
  200px result is read back. If the GPU path is unavailable or fails for any
  reason (remote desktop, rotated screen, driver errors) Satellite uses the
  standard method instead and retries the GPU later (after 30 seconds, backing
  off to 10 minutes). Opening the GPU path loads graphics driver libraries, so
  expect Satellite's memory use to rise by tens of MB while it is on. Turn off
  NVML to release it.
- The thumbnail goes to MQTT, and to the web UI's **Sensors** tab as a live
  preview. It is not added to the telemetry webhook or the `/json` endpoint.
- For the preview, Satellite keeps only the latest small frame per selected display in memory, drops
  it as soon as the sensor is turned off, and serves it at
  `/api/screen-thumbnail`, which needs the web UI token. It captures for the
  preview only while a browser has the Sensors tab open (and for a few seconds
  after), so a closed or backgrounded tab costs nothing. This works even when
  MQTT is not connected.

**Hiding programs.** The program filter is prefilled with common browsers,
messaging apps (including Discord, WhatsApp, Signal and Telegram), and email
clients. Browser variants that share an executable name are covered by that
entry; renamed executables and other programs need to be added manually.
Existing custom lists are preserved. You can edit or clear the defaults; an
explicitly cleared list stays empty after saving, restarting, or exporting and
importing. Up to 128 program names are supported. Thumbnails remain off until
you enable them.

Under the thumbnail size you can list programs (for
example `chrome.exe`, one per line) whose windows are painted black in the
picture, in the web UI or as `screen_thumbnail_hidden_apps` in the config file.
Satellite finds the visible windows of those programs on each selected screen each
time and blacks out their whole rectangle, even where another window overlaps
it. If the window lookup fails, no picture is sent for that update. Matching is
by program file name, so apps that run inside a shared host process (some Store
apps) cannot be told apart. This is separate from DRM: protected video is often
blank in screen captures anyway, but do not rely on that.

Masking also covers windows spanning multiple monitors, including displays
above or to the left of the primary screen. Thumbnail size, hidden programs,
and the update interval apply to all selected displays. Capture work and GPU
memory use increase with the number of selected displays.

At the default 200px ordinary text is unreadable (larger sizes show more, and
small text can become readable above about 200px), but large headings and titles may be
guessable, and the picture shows what kind of thing is on screen (a video, a
game, a document, a dark or light page). Only turn it on
if you are happy for everyone who can read your MQTT topics or your Home
Assistant to see that.

---

## Telemetry Webhook

Satellite can also send its data to anything that accepts an HTTP POST, such as Node-RED, n8n, a Home Assistant webhook trigger, or your own server. It runs alongside MQTT, so Home Assistant keeps working as before. Leave the MQTT broker blank if you only want the webhook.

Enable it in the Web UI under **Telemetry Webhook**, or in `config.json`:

```json
"telemetry_webhook": {
  "enabled": true,
  "url": "https://example.com/satellite",
  "bearer_token": "optional-secret",
  "min_interval_sec": 0
}
```

- A snapshot is sent when a value changes noticeably, and at least once per heartbeat (`heartbeat_sec`, default 60s).
- `min_interval_sec` sets a minimum gap between posts (0 means post on every change).
- If `bearer_token` is set, it is sent as `Authorization: Bearer <token>`.
- Any 2xx response counts as success. Failures are logged and retried on the next change or heartbeat.

The body shares common envelope fields with Android, but metadata and state value types differ. See the [schema comparison](../docs/mqtt-and-webhooks.md#telemetry-webhooks) before handling both in one receiver:

```json
{
  "device_id": "my-pc",
  "device_name": "MY-PC",
  "platform": "windows",
  "app_version": "0.26.1",
  "timestamp": 1761259200000,
  "state": { "cpu_percent": 12.5, "memory_percent": 48.1, "timestamp": 1761259200 }
}
```

`state` holds the same fields as the MQTT state topic and the `/json` endpoint. Only sensors you have enabled are included. The outer `timestamp` is in milliseconds, and `state.timestamp` is in seconds.

The webhook only sends data out. Remote commands such as lock and media control still need MQTT.

---

## Disclaimer

This project was built with AI assistance and reviewed by a human.
